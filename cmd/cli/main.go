package main

import (
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
	var tempFile string
	var err error

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
			// Parse compose arguments and create temporary receipt file
			composeArgs := args[composeIndex+1:]
			tempFile, err = createComposedReceipt(composeArgs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating composed receipt: %v\n", err)
				os.Exit(1)
			}
			defer os.Remove(tempFile) // Clean up temp file

			// Reconstruct command with temp file path instead of --compose args
			newArgs := append(args[:composeIndex], tempFile)
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
  print <printer-id> <receipt-path> [--var key=value]
    Print a receipt to the specified printer
    
  print <printer-id> --compose <commands...>
    Compose and print a receipt from command-line arguments
    Compose commands:
      text:"Hello World"              - Text command
      text:"Title" size:32 align:center - Text with properties
      feed:2                          - Feed lines
      cut                             - Cut paper
      align:center                    - Set alignment
      divider                         - Add divider
      
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
			fmt.Println("\nPrinters:")
			for _, p := range printers {
				if printer, ok := p.(map[string]interface{}); ok {
					name := printer["name"]
					if name == "" {
						name = printer["description"]
					}
					fmt.Printf("  %s: %s (%s)\n", printer["id"], name, printer["type"])
				}
			}
		}

		if jobs, ok := result.Data["jobs"].([]interface{}); ok {
			fmt.Println("\nJobs:")
			for _, j := range jobs {
				if job, ok := j.(map[string]interface{}); ok {
					fmt.Printf("  %s: %s (printer: %s)\n",
						job["id"], job["status"], job["printer_id"])
				}
			}
		}

		if jobID, ok := result.Data["job_id"].(string); ok {
			fmt.Printf("Job ID: %s\n", jobID)
		}

		if printerID, ok := result.Data["printer_id"].(string); ok {
			fmt.Printf("Printer ID: %s\n", printerID)
		}
	}
}

func printError(result *CommandResult) {
	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error)
	} else if result.Message != "" {
		fmt.Fprintf(os.Stderr, "%s\n", result.Message)
	}
}

// createComposedReceipt parses compose arguments and creates a temporary receipt file
// Arguments are parsed into commands. Each command starts with a command type (e.g., "text:", "feed:", "cut")
// and can be followed by properties (e.g., "size:32", "align:center")
func createComposedReceipt(composeArgs []string) (string, error) {
	if len(composeArgs) == 0 {
		return "", fmt.Errorf("no compose arguments provided")
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
				return "", fmt.Errorf("failed to parse command '%s': %v", arg, err)
			}
		} else if currentCmd != nil {
			// This is a property for the current command
			if err := parseCommandProperty(currentCmd, arg); err != nil {
				return "", fmt.Errorf("failed to parse property '%s': %v", arg, err)
			}
		} else {
			return "", fmt.Errorf("unexpected argument '%s' (expected command start)", arg)
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

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "receipt-composed-*.receipt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	// Write JSON to file
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(receipt); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write receipt JSON: %v", err)
	}

	return tmpFile.Name(), nil
}

// isCommandStart checks if an argument starts a new command
func isCommandStart(arg string) bool {
	// Check for command types: text:, feed:, align:, cut, divider, etc.
	knownCommands := []string{"text:", "feed:", "align:", "cut", "divider", "image:", "barcode:", "qrcode:"}
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
	case "align":
		cmd["value"] = strings.Trim(firstValue, `"'`)
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
