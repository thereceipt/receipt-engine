package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	defaultServerURL = "http://localhost:12212"
)

func main() {
	var serverURL string
	flag.StringVar(&serverURL, "server", defaultServerURL, "Server URL")
	flag.StringVar(&serverURL, "s", defaultServerURL, "Server URL (short)")
	flag.Parse()

	if flag.NArg() == 0 {
		printUsage()
		os.Exit(1)
	}

	args := flag.Args()

	// Check if this is a print command with --compose flag
	var command string

	if len(args) >= 2 && args[0] == "print" {
		// Look for --compose flag
		composeIndex := -1
		for i, arg := range args {
			if arg == "--compose" {
				composeIndex = i
				break
			}
		}

		if composeIndex >= 0 {
			// Parse compose arguments and create receipt JSON
			rest := args[composeIndex+1:]
			if len(rest) == 0 {
				fmt.Fprintf(os.Stderr, "Error: --compose requires at least one command argument\n")
				os.Exit(1)
			}

			// Stop compose parsing when flags start (e.g. --repeat, --var, --var-array)
			composeEnd := 0
			for composeEnd < len(rest) && !strings.HasPrefix(rest[composeEnd], "--") {
				composeEnd++
			}
			composeArgs := rest[:composeEnd]
			remainderArgs := rest[composeEnd:]
			if len(composeArgs) == 0 {
				fmt.Fprintf(os.Stderr, "Error: --compose requires at least one command argument\n")
				os.Exit(1)
			}

			// Create receipt JSON from compose arguments
			receiptJSON, err := createComposedReceiptJSON(composeArgs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating composed receipt: %v\n", err)
				os.Exit(1)
			}

			// Base64 encode the JSON to safely pass it in the command string
			encodedJSON := base64.StdEncoding.EncodeToString(receiptJSON)
			inlineReceipt := fmt.Sprintf("compose://%s", encodedJSON)

			// Reconstruct command with inline receipt instead of --compose args
			newArgs := append(args[:composeIndex], inlineReceipt)
			newArgs = append(newArgs, remainderArgs...)
			command = strings.Join(newArgs, " ")
		} else {
			command = strings.Join(args, " ")
		}
	} else {
		command = strings.Join(args, " ")
	}

	result := executeCommand(serverURL, command)

	if result.Success {
		printSuccess(result)
		os.Exit(0)
	} else {
		printError(result)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Receipt Engine CLI

Usage:
  receipt-cli [flags] <command>

Flags:
  -s, -server <url>    Server URL (default: %s)

Commands:
  print <printer-id> <receipt-path> [--repeat N] [--var key=value]
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
      qrcode:"data" error_correction:L|M|Q|H - QR code with error correction
      
    Note: Use align as a property of text commands, not as a standalone command
    Note: Add --repeat N after the receipt / compose to print multiple times
      
  server status|stop|restart
    Server lifecycle commands (stop/restart affect the running server process)
    
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
    Show help message

Examples:
  receipt-cli print printer-123 ./receipt.receipt
  receipt-cli print printer-123 ./receipt.receipt --var customer="John Doe"
  receipt-cli print printer-123 --compose text:"Hello" feed:2 cut
  receipt-cli print printer-123 --compose text:"Title" size:32 align:center feed:1 cut
  receipt-cli printer add-network 192.168.1.100 9100
  receipt-cli printer rename printer-123 "Kitchen Printer"
  receipt-cli job status job-456
  receipt-cli -s http://localhost:8080 printer list

`, defaultServerURL)
}

type CommandResult struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

func executeCommand(serverURL, command string) *CommandResult {
	url := strings.TrimSuffix(serverURL, "/") + "/command"

	reqBody := map[string]string{
		"command": command,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return &CommandResult{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal request: %v", err),
		}
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return &CommandResult{
			Success: false,
			Error:   fmt.Sprintf("failed to connect to server: %v", err),
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &CommandResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read response: %v", err),
		}
	}

	var result CommandResult
	if err := json.Unmarshal(body, &result); err != nil {
		return &CommandResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse response: %v", err),
		}
	}

	return &result
}

func printSuccess(result *CommandResult) {
	if result.Message != "" {
		fmt.Println(result.Message)
	}

	if result.Data != nil {
		// Pretty print data
		if printers, ok := result.Data["printers"].([]interface{}); ok {
			for _, p := range printers {
				if printer, ok := p.(map[string]interface{}); ok {
					displayName := getString(printer, "name")
					if displayName == "" {
						displayName = getString(printer, "description")
					}
					if displayName == "" {
						displayName = "Unnamed"
					}

					printerID := getString(printer, "id")
					printerType := getString(printer, "type")

					// Build description line
					desc := fmt.Sprintf("  %s: %s (%s", printerID, displayName, printerType)

					if printerType == "network" {
						host := getString(printer, "host")
						port := getInt(printer, "port")
						if host != "" && port > 0 {
							desc += fmt.Sprintf(", %s:%d", host, port)
						}
					} else if printerType == "serial" {
						device := getString(printer, "device")
						if device != "" {
							desc += fmt.Sprintf(", %s", device)
						}
					} else if printerType == "usb" {
						vid := getInt(printer, "vid")
						pid := getInt(printer, "pid")
						if vid > 0 && pid > 0 {
							desc += fmt.Sprintf(", VID:0x%04X PID:0x%04X", vid, pid)
						}
						device := getString(printer, "device")
						if device != "" {
							desc += fmt.Sprintf(", %s", device)
						}
					}

					desc += ")"
					fmt.Println(desc)
				}
			}
		}

		if jobs, ok := result.Data["jobs"].([]interface{}); ok {
			for _, j := range jobs {
				if job, ok := j.(map[string]interface{}); ok {
					jobID := getString(job, "id")
					status := getString(job, "status")
					printerID := getString(job, "printer_id")
					retries := getInt(job, "retries")
					createdAt := getString(job, "created_at")
					errorMsg := getString(job, "error")

					line := fmt.Sprintf("  %s: %s (printer: %s, retries: %d", jobID, status, printerID, retries)
					if createdAt != "" {
						line += fmt.Sprintf(", created: %s", createdAt)
					}
					line += ")"
					if errorMsg != "" {
						line += fmt.Sprintf(" - Error: %s", errorMsg)
					}
					fmt.Println(line)
				}
			}
		}

		if jobData, ok := result.Data["job"].(map[string]interface{}); ok {
			// Single job details
			printJobDetails(jobData)
		}

		if jobID, ok := result.Data["job_id"].(string); ok {
			fmt.Printf("Job ID: %s\n", jobID)
		}

		if printerID, ok := result.Data["printer_id"].(string); ok && result.Data["printer"] == nil {
			// Only print if we don't have full printer data
			fmt.Printf("Printer ID: %s\n", printerID)
		}

		if printer, ok := result.Data["printer"].(map[string]interface{}); ok {
			displayName := getString(printer, "name")
			if displayName == "" {
				displayName = getString(printer, "description")
			}
			printerID := getString(printer, "id")
			printerType := getString(printer, "type")
			fmt.Printf("  ID: %s, Name: %s, Type: %s", printerID, displayName, printerType)
			if printerType == "network" {
				host := getString(printer, "host")
				port := getInt(printer, "port")
				if host != "" && port > 0 {
					fmt.Printf(", %s:%d", host, port)
				}
			}
			fmt.Println()
		}

		if oldName, ok := result.Data["old_name"].(string); ok {
			if newName, ok2 := result.Data["new_name"].(string); ok2 && newName != "" {
				fmt.Printf("  Renamed from '%s' to '%s'\n", oldName, newName)
			}
		}
	}
}

// Helper functions for safe map access
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

func printJobDetails(job map[string]interface{}) {
	jobID := getString(job, "id")
	status := getString(job, "status")
	printerID := getString(job, "printer_id")
	retries := getInt(job, "retries")
	createdAt := getString(job, "created_at")
	errorMsg := getString(job, "error")

	fmt.Printf("  Job ID: %s\n", jobID)
	fmt.Printf("  Status: %s\n", status)
	fmt.Printf("  Printer ID: %s\n", printerID)
	fmt.Printf("  Retries: %d\n", retries)
	if createdAt != "" {
		fmt.Printf("  Created: %s\n", createdAt)
	}
	if errorMsg != "" {
		fmt.Printf("  Error: %s\n", errorMsg)
	}
}

func printError(result *CommandResult) {
	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error)
	} else if result.Message != "" {
		fmt.Fprintf(os.Stderr, "%s\n", result.Message)
	}
}

// createComposedReceiptJSON parses compose arguments and returns receipt JSON as bytes
// Arguments are parsed into commands. Each command starts with a command type (e.g., "text:", "feed:", "cut")
// and can be followed by properties (e.g., "size:32", "align:center")
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
	// Note: align is not a standalone command; use it as a property (e.g. text:"Hi" align:center)
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
