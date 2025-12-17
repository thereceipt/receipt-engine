package command

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/thereceipt/receipt-engine/internal/parser"
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

// handlePrint handles print commands
// Usage: print <printer-id> <receipt-path> [--var key=value] [--var-array key=value1,value2]
func (e *Executor) handlePrint(args []string) *Result {
	if len(args) < 2 {
		return &Result{
			Success: false,
			Error:   "usage: print <printer-id> <receipt-path> [--var key=value] [--var-array key=value1,value2]",
		}
	}

	printerID := args[0]
	receiptPath := args[1]

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

	if strings.HasPrefix(receiptPath, "http://") || strings.HasPrefix(receiptPath, "https://") {
		// Load from URL
		receipt, err = loadReceiptFromURL(receiptPath)
	} else {
		// Load from file
		data, err2 := os.ReadFile(receiptPath)
		if err2 != nil {
			return &Result{
				Success: false,
				Error:   fmt.Sprintf("failed to read receipt file: %v", err2),
			}
		}
		receipt, err = receiptformat.Parse(data)
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

	// Parse variable data from args
	varData := make(map[string]interface{})
	varArrayData := make(map[string][]map[string]interface{})

	for i := 2; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--var ") {
			kv := strings.TrimPrefix(arg, "--var ")
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				varData[parts[0]] = parts[1]
			}
		} else if strings.HasPrefix(arg, "--var-array ") {
			kv := strings.TrimPrefix(arg, "--var-array ")
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				// Simple parsing: comma-separated values
				// For more complex arrays, use JSON format
				values := strings.Split(parts[1], ",")
				arrayItems := make([]map[string]interface{}, len(values))
				for j, v := range values {
					arrayItems[j] = map[string]interface{}{"value": strings.TrimSpace(v)}
				}
				varArrayData[parts[0]] = arrayItems
			}
		}
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

	// Enqueue print job
	jobID := e.queue.Enqueue(printerID, img)

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Print job queued: %s", jobID),
		Data: map[string]interface{}{
			"job_id":     jobID,
			"printer_id": printerID,
		},
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
			}
		}
		return &Result{
			Success: true,
			Message: fmt.Sprintf("Found %d printer(s)", len(printers)),
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
			Message: fmt.Sprintf("Added network printer: %s", description),
			Data: map[string]interface{}{
				"printer_id": printerID,
				"printer":    printer,
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
		success := e.manager.SetPrinterName(printerID, name)
		if !success {
			return &Result{
				Success: false,
				Error:   fmt.Sprintf("printer not found: %s", printerID),
			}
		}
		return &Result{
			Success: true,
			Message: fmt.Sprintf("Renamed printer %s to %s", printerID, name),
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
			Message: fmt.Sprintf("Found %d job(s)", len(jobs)),
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
		return &Result{
			Success: true,
			Data:    jobData,
		}

	case "clear":
		e.queue.ClearCompleted()
		return &Result{
			Success: true,
			Message: "Cleared completed jobs",
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
	return &Result{
		Success: true,
		Message: fmt.Sprintf("Detected %d printer(s)", len(printers)),
		Data: map[string]interface{}{
			"count": len(printers),
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
    Example: print printer-123 --compose text:"Hello" feed:2 cut
    
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
