package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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

	command := strings.Join(flag.Args(), " ")
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
