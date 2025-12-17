package command

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/thereceipt/receipt-engine/internal/parser"
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

// handlePrint handles print commands
// Usage: print <printer-id> <receipt-path> [--repeat N] [--var key=value] [--var-array key=value1,value2]
//
//	print <printer-id> --compose <commands...> [--repeat N] [--var key=value] [--var-array key=value1,value2]
func (e *Executor) handlePrint(args []string) *Result {
	if len(args) < 2 {
		return &Result{
			Success: false,
			Error:   "usage: print <printer-id> <receipt-path> [--repeat N] [--var key=value] [--var-array key=value1,value2]",
		}
	}

	printerID := args[0]
	receiptArg := args[1]

	// Check if printer exists
	printer := e.manager.GetPrinter(printerID)
	if printer == nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("printer not found: %s", printerID),
		}
	}

	// Load receipt
	var receipt *receiptformat.Receipt
	var err error

	// Determine where options start, and build receipt.
	// After receipt is loaded, we parse options from args[optionsIdx:].
	optionsIdx := 2

	// Case 1: TUI / raw command string: print <printer-id> --compose <commands...> [flags...]
	if receiptArg == "--compose" {
		if len(args) < 3 {
			return &Result{Success: false, Error: "usage: print <printer-id> --compose <commands...> [--repeat N] [--var key=value] [--var-array key=value1,value2]"}
		}

		composeArgs := []string{}
		i := 2
		for ; i < len(args); i++ {
			if strings.HasPrefix(args[i], "--") {
				break
			}
			composeArgs = append(composeArgs, args[i])
		}
		if len(composeArgs) == 0 {
			return &Result{Success: false, Error: "usage: print <printer-id> --compose <commands...> [--repeat N] [--var key=value] [--var-array key=value1,value2]"}
		}

		receiptJSON, composeErr := createComposedReceiptJSON(composeArgs)
		if composeErr != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to create receipt from compose: %v", composeErr)}
		}
		receipt, err = receiptformat.Parse(receiptJSON)
		optionsIdx = i
	} else if strings.HasPrefix(receiptArg, "compose://") {
		// Case 2: CLI pre-processed compose: print <printer-id> compose://<base64> [flags...]
		encodedJSON := strings.TrimPrefix(receiptArg, "compose://")
		jsonData, decodeErr := base64.StdEncoding.DecodeString(encodedJSON)
		if decodeErr != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to decode receipt JSON: %v", decodeErr)}
		}
		receipt, err = receiptformat.Parse(jsonData)
		optionsIdx = 2
	} else if strings.HasPrefix(receiptArg, "http://") || strings.HasPrefix(receiptArg, "https://") {
		// Case 3: URL receipt
		receipt, err = loadReceiptFromURL(receiptArg)
		optionsIdx = 2
	} else {
		// Case 4: file receipt
		data, err2 := os.ReadFile(receiptArg)
		if err2 != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to read receipt file: %v", err2)}
		}
		receipt, err = receiptformat.Parse(data)
		optionsIdx = 2
	}

	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to load receipt: %v", err),
		}
	}

	// Validate receipt
	if err := receiptformat.Validate(receipt); err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("invalid receipt: %v", err),
		}
	}

	// Parse options
	varData := make(map[string]interface{})
	varArrayData := make(map[string][]map[string]interface{})
	repeat := 1

	for i := optionsIdx; i < len(args); i++ {
		arg := args[i]

		// --repeat N / --repeat=N
		if arg == "--repeat" {
			if i+1 >= len(args) {
				return &Result{Success: false, Error: "usage: --repeat <number>"}
			}
			n, convErr := strconv.Atoi(args[i+1])
			if convErr != nil {
				return &Result{Success: false, Error: fmt.Sprintf("invalid --repeat value: %s", args[i+1])}
			}
			repeat = n
			i++
			continue
		}
		if strings.HasPrefix(arg, "--repeat=") {
			nStr := strings.TrimPrefix(arg, "--repeat=")
			n, convErr := strconv.Atoi(nStr)
			if convErr != nil {
				return &Result{Success: false, Error: fmt.Sprintf("invalid --repeat value: %s", nStr)}
			}
			repeat = n
			continue
		}

		// --var key=value / --var=key=value
		if arg == "--var" {
			if i+1 >= len(args) {
				return &Result{Success: false, Error: "usage: --var key=value"}
			}
			kv := args[i+1]
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				varData[parts[0]] = parts[1]
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "--var=") {
			kv := strings.TrimPrefix(arg, "--var=")
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				varData[parts[0]] = parts[1]
			}
			continue
		}

		// --var-array name=v1,v2 / --var-array=name=v1,v2
		if arg == "--var-array" {
			if i+1 >= len(args) {
				return &Result{Success: false, Error: "usage: --var-array name=v1,v2"}
			}
			kv := args[i+1]
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				values := strings.Split(parts[1], ",")
				arrayItems := make([]map[string]interface{}, len(values))
				for j, v := range values {
					arrayItems[j] = map[string]interface{}{"value": strings.TrimSpace(v)}
				}
				varArrayData[parts[0]] = arrayItems
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "--var-array=") {
			kv := strings.TrimPrefix(arg, "--var-array=")
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				values := strings.Split(parts[1], ",")
				arrayItems := make([]map[string]interface{}, len(values))
				for j, v := range values {
					arrayItems[j] = map[string]interface{}{"value": strings.TrimSpace(v)}
				}
				varArrayData[parts[0]] = arrayItems
			}
			continue
		}
	}

	if repeat < 1 {
		return &Result{Success: false, Error: "--repeat must be >= 1"}
	}
	if repeat > 100 {
		return &Result{Success: false, Error: "--repeat too large (max 100)"}
	}

	// Create parser
	paperWidth := receipt.PaperWidth
	if paperWidth == "" {
		paperWidth = "80mm"
	}

	p, err := parser.New(receipt, paperWidth)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to create parser: %v", err),
		}
	}

	// Set variable data
	if len(varData) > 0 {
		p.SetVariableData(varData)
	}
	if len(varArrayData) > 0 {
		p.SetVariableArrayData(varArrayData)
	}

	// Execute
	img, err := p.Execute()
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to render receipt: %v", err),
		}
	}

	// Enqueue print job(s)
	jobIDs := make([]string, 0, repeat)
	for i := 0; i < repeat; i++ {
		jobIDs = append(jobIDs, e.queue.Enqueue(printerID, img))
	}

	// Get printer name for better message
	printerName := printerID
	if p := e.manager.GetPrinter(printerID); p != nil {
		if p.Name != "" {
			printerName = p.Name
		} else if p.Description != "" {
			printerName = p.Description
		}
	}

	data := map[string]interface{}{
		"printer_id": printerID,
		"job_ids":    jobIDs,
		"repeat":     repeat,
	}
	if repeat == 1 {
		data["job_id"] = jobIDs[0]
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Queued %d print job(s) (Printer: %s)", repeat, printerName),
		Data:    data,
	}
}

// handleServer handles server lifecycle/info commands.
// Usage: server status | stop|off|shutdown | restart
func (e *Executor) handleServer(args []string) *Result {
	if len(args) == 0 {
		return &Result{Success: false, Error: "usage: server <status|stop|restart>"}
	}

	switch args[0] {
	case "status":
		return &Result{
			Success: true,
			Message: "Server status",
			Data: map[string]interface{}{
				"pid":         os.Getpid(),
				"printer_cnt": len(e.manager.GetAllPrinters()),
				"job_cnt":     len(e.queue.GetAllJobs()),
			},
		}
	case "stop", "off", "shutdown":
		// Let the response flush before exiting.
		time.AfterFunc(200*time.Millisecond, func() { os.Exit(0) })
		return &Result{Success: true, Message: "Server shutting down..."}
	case "restart":
		// Exit with a non-zero code so a supervisor can restart it.
		time.AfterFunc(200*time.Millisecond, func() { os.Exit(42) })
		return &Result{Success: true, Message: "Server restarting..."}
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown server subcommand: %s (use: status, stop, restart)", args[0])}
	}
}

// handlePrinter handles printer commands
// Usage: printer list | add-network <host> [port] | rename <id> <name>
func (e *Executor) handlePrinter(args []string) *Result {
	if len(args) == 0 {
		return &Result{
			Success: false,
			Error:   "usage: printer <list|add-network|rename>",
		}
	}

	subcommand := args[0]

	switch subcommand {
	case "list":
		printers := e.manager.GetAllPrinters()
		printerList := make([]map[string]interface{}, len(printers))
		for i, p := range printers {
			printerList[i] = map[string]interface{}{
				"id":          p.ID,
				"type":        p.Type,
				"description": p.Description,
				"name":        p.Name,
			}
			if p.Type == "network" {
				printerList[i]["host"] = p.Host
				printerList[i]["port"] = p.Port
			} else if p.Type == "serial" {
				printerList[i]["device"] = p.Device
			} else if p.Type == "usb" {
				printerList[i]["vid"] = p.VID
				printerList[i]["pid"] = p.PID
				if p.Device != "" {
					printerList[i]["device"] = p.Device
				}
			}
		}
		return &Result{
			Success: true,
			Message: fmt.Sprintf("Found %d printer(s):", len(printers)),
			Data: map[string]interface{}{
				"printers": printerList,
			},
		}

	case "add-network":
		if len(args) < 2 {
			return &Result{
				Success: false,
				Error:   "usage: printer add-network <host> [port]",
			}
		}
		host := args[1]
		port := 9100
		if len(args) >= 3 {
			var err error
			port, err = strconv.Atoi(args[2])
			if err != nil {
				return &Result{
					Success: false,
					Error:   fmt.Sprintf("invalid port: %s", args[2]),
				}
			}
		}
		description := fmt.Sprintf("Network: %s:%d", host, port)
		printerID := e.manager.AddNetworkPrinter(host, port, description)
		printer := e.manager.GetPrinter(printerID)
		return &Result{
			Success: true,
			Message: fmt.Sprintf("Added network printer: %s (ID: %s)", description, printerID),
			Data: map[string]interface{}{
				"printer_id": printerID,
				"printer": map[string]interface{}{
					"id":          printer.ID,
					"type":        printer.Type,
					"description": printer.Description,
					"name":        printer.Name,
					"host":        printer.Host,
					"port":        printer.Port,
				},
			},
		}

	case "rename":
		if len(args) < 3 {
			return &Result{
				Success: false,
				Error:   "usage: printer rename <id> <name>",
			}
		}
		printerID := args[1]
		name := args[2]
		printer := e.manager.GetPrinter(printerID)
		if printer == nil {
			return &Result{
				Success: false,
				Error:   fmt.Sprintf("printer not found: %s", printerID),
			}
		}
		oldName := printer.Name
		if oldName == "" {
			oldName = printer.Description
		}
		success := e.manager.SetPrinterName(printerID, name)
		if !success {
			return &Result{
				Success: false,
				Error:   fmt.Sprintf("failed to rename printer: %s", printerID),
			}
		}
		return &Result{
			Success: true,
			Message: fmt.Sprintf("Renamed printer %s from '%s' to '%s'", printerID, oldName, name),
			Data: map[string]interface{}{
				"printer_id": printerID,
				"old_name":   oldName,
				"new_name":   name,
			},
		}

	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown printer subcommand: %s. Use: list, add-network, rename", subcommand),
		}
	}
}

// handleJob handles job commands
// Usage: job list | status <id> | clear
func (e *Executor) handleJob(args []string) *Result {
	if len(args) == 0 {
		return &Result{
			Success: false,
			Error:   "usage: job <list|status|clear>",
		}
	}

	subcommand := args[0]

	switch subcommand {
	case "list":
		jobs := e.queue.GetAllJobs()
		jobList := make([]map[string]interface{}, len(jobs))
		for i, job := range jobs {
			jobList[i] = map[string]interface{}{
				"id":         job.ID,
				"printer_id": job.PrinterID,
				"status":     job.Status,
				"retries":    job.Retries,
				"created_at": job.CreatedAt,
			}
			if job.Error != nil {
				jobList[i]["error"] = job.Error.Error()
			}
		}
		return &Result{
			Success: true,
			Message: fmt.Sprintf("Found %d job(s):", len(jobs)),
			Data: map[string]interface{}{
				"jobs": jobList,
			},
		}

	case "status":
		if len(args) < 2 {
			return &Result{
				Success: false,
				Error:   "usage: job status <id>",
			}
		}
		jobID := args[1]
		job := e.queue.GetJob(jobID)
		if job == nil {
			return &Result{
				Success: false,
				Error:   fmt.Sprintf("job not found: %s", jobID),
			}
		}
		jobData := map[string]interface{}{
			"id":         job.ID,
			"printer_id": job.PrinterID,
			"status":     job.Status,
			"retries":    job.Retries,
			"created_at": job.CreatedAt,
		}
		if job.Error != nil {
			jobData["error"] = job.Error.Error()
		}
		statusMsg := fmt.Sprintf("Job %s: status=%s, printer=%s, retries=%d", job.ID, job.Status, job.PrinterID, job.Retries)
		if job.Error != nil {
			statusMsg += fmt.Sprintf(", error=%s", job.Error.Error())
		}
		return &Result{
			Success: true,
			Message: statusMsg,
			Data:    jobData,
		}

	case "clear":
		allJobs := e.queue.GetAllJobs()
		completedCount := 0
		for _, job := range allJobs {
			if job.Status == "completed" {
				completedCount++
			}
		}
		e.queue.ClearCompleted()
		message := "Cleared completed jobs"
		if completedCount > 0 {
			message = fmt.Sprintf("Cleared %d completed job(s)", completedCount)
		}
		return &Result{
			Success: true,
			Message: message,
		}

	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown job subcommand: %s. Use: list, status, clear", subcommand),
		}
	}
}

// handleDetect handles detect command
// Usage: detect
func (e *Executor) handleDetect(args []string) *Result {
	printers, err := e.manager.DetectPrinters()
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("detection failed: %v", err),
		}
	}
	printerList := make([]map[string]interface{}, len(printers))
	for i, p := range printers {
		printerList[i] = map[string]interface{}{
			"id":          p.ID,
			"type":        p.Type,
			"description": p.Description,
			"name":        p.Name,
		}
		if p.Type == "network" {
			printerList[i]["host"] = p.Host
			printerList[i]["port"] = p.Port
		} else if p.Type == "serial" {
			printerList[i]["device"] = p.Device
		} else if p.Type == "usb" {
			printerList[i]["vid"] = p.VID
			printerList[i]["pid"] = p.PID
			if p.Device != "" {
				printerList[i]["device"] = p.Device
			}
		}
	}
	return &Result{
		Success: true,
		Message: fmt.Sprintf("Detected %d printer(s):", len(printers)),
		Data: map[string]interface{}{
			"printers": printerList,
		},
	}
}

// handleHelp handles help command
func (e *Executor) handleHelp(args []string) *Result {
	helpText := `Available Commands:

  print <printer-id> <receipt-path> [--var key=value]
    Print a receipt to the specified printer
    
  print <printer-id> --compose <commands...>
    Compose and print a receipt from command-line arguments
    
    Available Commands:
      text:"Hello"                    - Text with value
      text:"Title" size:32 align:center weight:bold - Text with properties
      feed:2                          - Feed N lines
      cut                             - Cut paper
      divider                         - Add divider line
      divider style:solid|dashed|dotted|double - Divider with style
      image:"/path/to/image.png"      - Print image from path
      barcode:"123456"                - Print barcode
      barcode:"123456" format:CODE128 height:50 - Barcode with options
      qrcode:"https://example.com"    - Print QR code
    
    Note: Use align as a property of text commands (e.g., text:"Hello" align:center)
    
    Example: print printer-123 --compose text:"Hello" feed:2 cut

  server status
    Show server info (pid, printers, jobs)

  server stop|off|shutdown
    Stop the running server process

  server restart
    Exit with restart code (for supervisors)
    
  printer list
    List all detected printers
    
  printer add-network <host> [port]
    Add a network printer (default port: 9100)
    
  printer rename <id> <name>
    Set a custom name for a printer
    
  job list
    List all print jobs
    
  job status <id>
    Get status of a specific job
    
  job clear
    Clear completed jobs from the queue
    
  detect
    Detect/scan for printers
    
  help
    Show this help message

Examples:
  print printer-123 ./receipt.receipt
  print printer-123 ./receipt.receipt --var customer="John Doe"
  print printer-123 --compose text:"Hello World" feed:2 cut
  print printer-123 --compose text:"Title" size:32 align:center feed:1 cut
  printer add-network 192.168.1.100 9100
  printer rename printer-123 "Kitchen Printer"
  job status job-456
`

	return &Result{
		Success: true,
		Message: helpText,
	}
}

// loadReceiptFromURL loads a receipt from a URL
func loadReceiptFromURL(url string) (*receiptformat.Receipt, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch receipt from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch receipt: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read receipt from URL: %w", err)
	}

	receipt, err := receiptformat.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse receipt: %w", err)
	}

	return receipt, nil
}

// createComposedReceiptJSON parses compose arguments and returns receipt JSON as bytes
func createComposedReceiptJSON(composeArgs []string) ([]byte, error) {
	if len(composeArgs) == 0 {
		return nil, fmt.Errorf("no compose arguments provided")
	}

	commands := []map[string]interface{}{}
	var currentCmd map[string]interface{}

	for _, arg := range composeArgs {
		// Check if this argument starts a new command
		if isCommandStart(arg) {
			// Save previous command if exists
			if currentCmd != nil {
				commands = append(commands, currentCmd)
			}
			// Start new command
			var err error
			currentCmd, err = parseComposeCommandStart(arg)
			if err != nil {
				return nil, fmt.Errorf("failed to parse command '%s': %v", arg, err)
			}
		} else if currentCmd != nil {
			// This is a property for the current command
			if err := parseCommandProperty(currentCmd, arg); err != nil {
				return nil, fmt.Errorf("failed to parse property '%s': %v", arg, err)
			}
		} else {
			return nil, fmt.Errorf("unexpected argument '%s' (expected command start)", arg)
		}
	}

	// Don't forget the last command
	if currentCmd != nil {
		commands = append(commands, currentCmd)
	}

	receipt := map[string]interface{}{
		"version":  "1.0",
		"commands": commands,
	}

	// Marshal to JSON (compact, single line)
	jsonData, err := json.Marshal(receipt)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal receipt JSON: %v", err)
	}

	return jsonData, nil
}

// isCommandStart checks if an argument starts a new command
func isCommandStart(arg string) bool {
	// Check for command types: text:, feed:, cut, divider, etc.
	// Note: align is not a standalone command - use it as a property of text commands
	knownCommands := []string{"text:", "feed:", "cut", "divider", "image:", "barcode:", "qrcode:"}
	for _, cmd := range knownCommands {
		if strings.HasPrefix(arg, cmd) || arg == strings.TrimSuffix(cmd, ":") {
			return true
		}
	}
	return false
}

// parseComposeCommandStart parses the start of a command (type and first value)
func parseComposeCommandStart(arg string) (map[string]interface{}, error) {
	cmd := make(map[string]interface{})
	colonIndex := strings.Index(arg, ":")

	if colonIndex == -1 {
		// No colon - it's a simple command like "cut" or "divider"
		cmd["type"] = arg
		return cmd, nil
	}

	cmdType := arg[:colonIndex]
	firstValue := arg[colonIndex+1:]

	cmd["type"] = cmdType

	// Parse first value based on command type
	switch cmdType {
	case "text":
		// Remove quotes if present
		value := strings.Trim(firstValue, `"'`)
		cmd["value"] = value
	case "feed":
		lines, err := strconv.Atoi(firstValue)
		if err != nil {
			return nil, fmt.Errorf("invalid feed lines value: %s", firstValue)
		}
		cmd["lines"] = lines
	case "image":
		cmd["path"] = strings.Trim(firstValue, `"'`)
	case "barcode", "qrcode":
		cmd["value"] = strings.Trim(firstValue, `"'`)
	default:
		// For unknown command types, treat first value as a generic value
		cmd["value"] = strings.Trim(firstValue, `"'`)
	}

	return cmd, nil
}

// parseCommandProperty parses a property argument and adds it to the command
func parseCommandProperty(cmd map[string]interface{}, arg string) error {
	colonIndex := strings.Index(arg, ":")
	if colonIndex == -1 {
		return fmt.Errorf("property must be in format 'name:value', got: %s", arg)
	}

	propName := arg[:colonIndex]
	propValue := arg[colonIndex+1:]

	// Try to parse as number first
	if intVal, err := strconv.Atoi(propValue); err == nil {
		cmd[propName] = intVal
	} else if boolVal, err := strconv.ParseBool(propValue); err == nil {
		cmd[propName] = boolVal
	} else {
		// String value, remove quotes if present
		cmd[propName] = strings.Trim(propValue, `"'`)
	}

	return nil
}
