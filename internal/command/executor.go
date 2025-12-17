// Package command provides a command system for the receipt engine
package command

import (
	"fmt"
	"strings"

	"github.com/thereceipt/receipt-engine/internal/printer"
)

// Executor executes commands
type Executor struct {
	manager *printer.Manager
	pool    *printer.ConnectionPool
	queue   *printer.PrintQueue
}

// NewExecutor creates a new command executor
func NewExecutor(manager *printer.Manager, pool *printer.ConnectionPool, queue *printer.PrintQueue) *Executor {
	return &Executor{
		manager: manager,
		pool:    pool,
		queue:   queue,
	}
}

// Result represents the result of executing a command
type Result struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// Execute executes a command string and returns a result
func (e *Executor) Execute(cmdStr string) *Result {
	// Parse command
	parts := parseCommand(cmdStr)
	if len(parts) == 0 {
		return &Result{
			Success: false,
			Error:   "empty command",
		}
	}

	command := parts[0]
	args := parts[1:]

	// Route to appropriate handler
	switch command {
	case "print":
		return e.handlePrint(args)
	case "printer":
		return e.handlePrinter(args)
	case "job":
		return e.handleJob(args)
	case "detect":
		return e.handleDetect(args)
	case "help":
		return e.handleHelp(args)
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown command: %s. Type 'help' for available commands", command),
		}
	}
}

// parseCommand parses a command string into parts, handling quoted strings
func parseCommand(cmdStr string) []string {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return []string{}
	}

	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(cmdStr); i++ {
		char := cmdStr[i]

		if char == '"' || char == '\'' {
			if !inQuotes {
				inQuotes = true
				quoteChar = char
			} else if char == quoteChar {
				inQuotes = false
				quoteChar = 0
			} else {
				current.WriteByte(char)
			}
		} else if char == ' ' && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
