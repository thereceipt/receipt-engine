package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	osc52 "github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thereceipt/receipt-engine/internal/command"
)

// CommandModel handles command input
type CommandModel struct {
	executor   *command.Executor
	input      textinput.Model
	visible    bool
	lastResult *command.Result
	width      int
	height     int
	scrollPos  int // For scrolling long results

	// When lastResult contains printers, allow copying IDs
	printerIDs         []string
	selectedPrinterIdx int
}

// NewCommandModel creates a new command model
func NewCommandModel(executor *command.Executor) CommandModel {
	input := textinput.New()
	input.Placeholder = "Enter command (e.g., 'printer list', 'help')"
	input.CharLimit = 200
	input.Prompt = "> "
	input.PromptStyle = lipgloss.NewStyle().Foreground(Secondary)

	return CommandModel{
		executor: executor,
		input:    input,
		visible:  false,
		width:    80,
	}
}

// SetSize sets the component size
func (m *CommandModel) SetSize(width int) {
	if width < 40 {
		width = 40
	}
	m.width = width
	// Input width should account for prompt and padding
	m.input.Width = width - 6
}

// SetHeight sets the maximum height for the command view
func (m *CommandModel) SetHeight(height int) {
	m.height = height
}

// Show shows the command input
func (m *CommandModel) Show() {
	m.visible = true
	m.input.Focus()
	m.lastResult = nil
	m.scrollPos = 0
	m.printerIDs = nil
	m.selectedPrinterIdx = 0
}

// Hide hides the command input
func (m *CommandModel) Hide() {
	m.visible = false
	m.input.Blur()
	m.input.SetValue("")
	m.printerIDs = nil
	m.selectedPrinterIdx = 0
}

// IsVisible returns whether the command input is visible
func (m *CommandModel) IsVisible() bool {
	return m.visible
}

// Update handles messages
func (m CommandModel) Update(msg tea.Msg) (CommandModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Execute command
			cmdStr := strings.TrimSpace(m.input.Value())
			if cmdStr != "" {
				m.lastResult = m.executor.Execute(cmdStr)
				m.input.SetValue("")
				m.scrollPos = 0 // Reset scroll on new command
				// Keep command bar open for quick commands

				// Extract printer IDs (if any) for selection/copy
				m.printerIDs = extractPrinterIDs(m.lastResult)
				m.selectedPrinterIdx = 0
			}
			return m, cmd

		case "esc":
			// Hide command bar
			m.Hide()
			return m, nil

		case "up":
			// Scroll up in results
			if m.scrollPos > 0 {
				m.scrollPos--
			}
			return m, nil

		case "down":
			// Scroll down in results (will be limited by available content)
			m.scrollPos++
			return m, nil

		case "pageup":
			if m.scrollPos > 5 {
				m.scrollPos -= 5
			} else {
				m.scrollPos = 0
			}
			return m, nil

		case "pagedown":
			m.scrollPos += 5
			return m, nil

		case "home":
			m.scrollPos = 0
			return m, nil

		case "end":
			// Set to a large number, will be clamped in View
			m.scrollPos = 9999
			return m, nil

		case "ctrl+j":
			// Select next printer (does not interfere with typing digits/letters)
			if len(m.printerIDs) > 0 && m.selectedPrinterIdx < len(m.printerIDs)-1 {
				m.selectedPrinterIdx++
			}
			return m, nil

		case "ctrl+k":
			// Select previous printer (does not interfere with typing digits/letters)
			if len(m.printerIDs) > 0 && m.selectedPrinterIdx > 0 {
				m.selectedPrinterIdx--
			}
			return m, nil

		case "ctrl+y":
			// Copy selected printer ID (does not interfere with typing digits/letters)
			if len(m.printerIDs) > 0 && m.selectedPrinterIdx >= 0 && m.selectedPrinterIdx < len(m.printerIDs) {
				id := m.printerIDs[m.selectedPrinterIdx]
				if id != "" {
					if err := copyToClipboard(id); err != nil {
						if m.lastResult != nil && m.lastResult.Success {
							m.lastResult.Message = fmt.Sprintf("%s (copy failed: %v)", m.lastResult.Message, err)
						}
					} else {
						if m.lastResult != nil && m.lastResult.Success {
							m.lastResult.Message = fmt.Sprintf("%s (copied printer ID)", m.lastResult.Message)
						}
					}
				}
			}
			return m, nil

		default:
			// Process input normally
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	return m, cmd
}

// View renders the command input
func (m CommandModel) View() string {
	if !m.visible {
		return ""
	}

	// Calculate available height for results
	// Header (1) + blank (1) + input (1) + blank (1) + help (2) = 6 lines minimum
	// Plus some padding
	headerHeight := 3 // Title + blank + input
	footerHeight := 2 // Help text
	availableHeight := m.height - headerHeight - footerHeight
	if m.height == 0 {
		// If height not set, use a reasonable default
		availableHeight = 15
	}
	if availableHeight < 5 {
		availableHeight = 5 // Minimum for results
	}

	var b strings.Builder

	// Title
	b.WriteString(HeaderStyle.Render("Command"))
	b.WriteString("\n\n")

	// Command input box
	inputView := m.input.View()
	boxStyle := InputFocusedStyle.
		Width(m.width-4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Secondary).
		Padding(0, 1)

	b.WriteString(boxStyle.Render(inputView))
	b.WriteString("\n")

	// Build result content first to calculate total lines
	var resultLines []string

	if m.lastResult != nil {
		if m.lastResult.Success {
			if m.lastResult.Message != "" {
				// Format help text specially
				if strings.Contains(m.lastResult.Message, "Available Commands:") {
					// Help text - wrap it properly
					helpLines := strings.Split(m.lastResult.Message, "\n")
					for _, line := range helpLines {
						if len(line) > m.width-4 {
							// Wrap long lines
							wrapped := wrapText(line, m.width-4)
							for _, wline := range wrapped {
								resultLines = append(resultLines, TextMuted.Render(wline))
							}
						} else {
							resultLines = append(resultLines, TextMuted.Render(line))
						}
					}
				} else {
					// Wrap message if needed
					msg := "✓ " + m.lastResult.Message
					if len(msg) > m.width-4 {
						wrapped := wrapText(msg, m.width-4)
						for _, line := range wrapped {
							resultLines = append(resultLines, SuccessStyle.Render(line))
						}
					} else {
						resultLines = append(resultLines, SuccessStyle.Render(msg))
					}
				}
			}
			// Show data if present
			if m.lastResult.Data != nil && len(m.lastResult.Data) > 0 {
				// Try to display printers
				printersVal, hasPrinters := m.lastResult.Data["printers"]
				if hasPrinters {
					resultLines = append(resultLines, "")
					resultLines = append(resultLines, SectionHeaderStyle.Render("Printers:"))

					// Handle []map[string]interface{} (from handlers) or []interface{} (from JSON)
					switch v := printersVal.(type) {
					case []map[string]interface{}:
						// Direct type from executor
						for i, printer := range v {
							if i > 0 {
								resultLines = append(resultLines, "") // Blank line between printers
							}
							lines := formatPrinterLine(printer)
							if len(lines) > 0 && i == m.selectedPrinterIdx {
								// Mark selection without truncating content
								lines[0] = "▶ " + strings.TrimPrefix(lines[0], "  ")
							}
							resultLines = append(resultLines, lines...)
						}
					case []interface{}:
						// Type from JSON unmarshaling
						for i, p := range v {
							if printer, ok := p.(map[string]interface{}); ok {
								if i > 0 {
									resultLines = append(resultLines, "") // Blank line between printers
								}
								lines := formatPrinterLine(printer)
								if len(lines) > 0 && i == m.selectedPrinterIdx {
									lines[0] = "▶ " + strings.TrimPrefix(lines[0], "  ")
								}
								resultLines = append(resultLines, lines...)
							}
						}
					default:
						// Fallback for debugging
						resultLines = append(resultLines, ErrorStyle.Render(fmt.Sprintf("  Unexpected printers type: %T", v)))
					}
				}

				// Try to display jobs
				jobsVal, hasJobs := m.lastResult.Data["jobs"]
				if hasJobs {
					resultLines = append(resultLines, "")
					resultLines = append(resultLines, SectionHeaderStyle.Render("Jobs:"))

					switch v := jobsVal.(type) {
					case []map[string]interface{}:
						for i, job := range v {
							if i > 0 {
								resultLines = append(resultLines, "") // Blank line between jobs
							}
							resultLines = append(resultLines, formatJobLine(job)...)
						}
					case []interface{}:
						for i, j := range v {
							if job, ok := j.(map[string]interface{}); ok {
								if i > 0 {
									resultLines = append(resultLines, "") // Blank line between jobs
								}
								resultLines = append(resultLines, formatJobLine(job)...)
							}
						}
					default:
						resultLines = append(resultLines, ErrorStyle.Render(fmt.Sprintf("  Unexpected jobs type: %T", v)))
					}
				}

				if jobData, ok := m.lastResult.Data["job"].(map[string]interface{}); ok {
					resultLines = append(resultLines, "")
					resultLines = append(resultLines, SectionHeaderStyle.Render("Job Details:"))
					jobID := getStringValue(jobData, "id")
					status := getStringValue(jobData, "status")
					printerID := getStringValue(jobData, "printer_id")
					retries := getIntValue(jobData, "retries")
					createdAt := getStringValue(jobData, "created_at")
					errorMsg := getStringValue(jobData, "error")

					resultLines = append(resultLines, fmt.Sprintf("  Job ID: %s", jobID))
					resultLines = append(resultLines, fmt.Sprintf("  Status: %s", status))
					resultLines = append(resultLines, fmt.Sprintf("  Printer ID: %s", printerID))
					resultLines = append(resultLines, fmt.Sprintf("  Retries: %d", retries))
					if createdAt != "" {
						resultLines = append(resultLines, fmt.Sprintf("  Created: %s", createdAt))
					}
					if errorMsg != "" {
						resultLines = append(resultLines, ErrorStyle.Render(fmt.Sprintf("  Error: %s", errorMsg)))
					}
				}
				if jobID, ok := m.lastResult.Data["job_id"].(string); ok {
					if m.lastResult.Data["job"] == nil {
						resultLines = append(resultLines, "")
						resultLines = append(resultLines, InfoStyle.Render(fmt.Sprintf("Job ID: %s", jobID)))
					}
				}
				if printer, ok := m.lastResult.Data["printer"].(map[string]interface{}); ok {
					resultLines = append(resultLines, "")
					displayName := getStringValue(printer, "name")
					if displayName == "" {
						displayName = getStringValue(printer, "description")
					}
					printerID := getStringValue(printer, "id")
					printerType := getStringValue(printer, "type")
					resultLines = append(resultLines, fmt.Sprintf("  ID: %s, Name: %s, Type: %s", printerID, displayName, printerType))
					if printerType == "network" {
						host := getStringValue(printer, "host")
						port := getIntValue(printer, "port")
						if host != "" && port > 0 {
							resultLines = append(resultLines, fmt.Sprintf("  Host: %s:%d", host, port))
						}
					}
				}
				if oldName, ok := m.lastResult.Data["old_name"].(string); ok {
					if newName, ok2 := m.lastResult.Data["new_name"].(string); ok2 && newName != "" {
						resultLines = append(resultLines, "")
						resultLines = append(resultLines, fmt.Sprintf("  Renamed from '%s' to '%s'", oldName, newName))
					}
				}
			}
		} else {
			// Wrap error message
			errMsg := "✗ " + m.lastResult.Error
			if len(errMsg) > m.width-4 {
				wrapped := wrapText(errMsg, m.width-4)
				for _, line := range wrapped {
					resultLines = append(resultLines, ErrorStyle.Render(line))
				}
			} else {
				resultLines = append(resultLines, ErrorStyle.Render(errMsg))
			}
		}
	}

	// Apply scrolling (calculate limits, but don't modify m.scrollPos in View)
	totalLines := len(resultLines)
	maxScroll := totalLines - availableHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	scrollPos := m.scrollPos
	if scrollPos > maxScroll {
		scrollPos = maxScroll
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	// Render visible lines
	start := scrollPos
	end := start + availableHeight
	if end > totalLines {
		end = totalLines
	}

	if totalLines > 0 {
		b.WriteString("\n")
		for i := start; i < end; i++ {
			b.WriteString(resultLines[i])
			b.WriteString("\n")
		}

		// Show scroll indicator
		if totalLines > availableHeight {
			if scrollPos > 0 && scrollPos < maxScroll {
				b.WriteString(TextMuted.Render(fmt.Sprintf("  ... (↑/↓ to scroll, %d/%d lines) ...", scrollPos+1, totalLines)))
			} else if scrollPos > 0 {
				b.WriteString(TextMuted.Render(fmt.Sprintf("  ... (↑ to scroll, %d/%d lines)", scrollPos+1, totalLines)))
			} else if scrollPos < maxScroll {
				b.WriteString(TextMuted.Render(fmt.Sprintf("  ... (↓ to scroll, %d/%d lines) ...", scrollPos+1, totalLines)))
			}
			b.WriteString("\n")
		}
	}

	// Help text
	b.WriteString("\n")
	helpText := "Press Enter to execute, Esc to close"
	if totalLines > availableHeight {
		helpText += ", ↑/↓ to scroll"
	}
	if len(m.printerIDs) > 0 {
		helpText += ", Ctrl+J/K select printer, Ctrl+Y copy ID"
	}
	b.WriteString(TextMuted.Render(helpText))

	// Return the content - the overlay will handle height constraints
	return b.String()
}

func extractPrinterIDs(res *command.Result) []string {
	if res == nil || !res.Success || res.Data == nil {
		return nil
	}
	val, ok := res.Data["printers"]
	if !ok || val == nil {
		return nil
	}

	ids := []string{}
	switch v := val.(type) {
	case []map[string]interface{}:
		for _, p := range v {
			if id := getStringValue(p, "id"); id != "" {
				ids = append(ids, id)
			}
		}
	case []interface{}:
		for _, p := range v {
			if m, ok := p.(map[string]interface{}); ok {
				if id := getStringValue(m, "id"); id != "" {
					ids = append(ids, id)
				}
			}
		}
	}

	if len(ids) == 0 {
		return nil
	}
	return ids
}

func copyToClipboard(text string) error {
	// Prefer system clipboard (works in most setups including alt-screen).
	if err := clipboard.WriteAll(text); err == nil {
		return nil
	}

	// Fallback to OSC52 for terminals that support it (incl. tmux/screen).
	seq := osc52.New(text).Tmux().Screen()
	_, _ = fmt.Fprint(os.Stderr, seq)
	return fmt.Errorf("system clipboard unavailable; sent OSC52 copy sequence (may not be supported by your terminal)")
}

// wrapText wraps text to fit within a given width
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// Helper functions for safe map access
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntValue(m map[string]interface{}, key string) int {
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

// formatPrinterLine formats a printer map into display lines (one field per line)
func formatPrinterLine(printer map[string]interface{}) []string {
	var lines []string
	printerID := getStringValue(printer, "id")
	printerType := getStringValue(printer, "type")

	lines = append(lines, fmt.Sprintf("  ID: %s", printerID))

	displayName := getStringValue(printer, "name")
	if displayName != "" {
		lines = append(lines, fmt.Sprintf("  Name: %s", displayName))
	}

	description := getStringValue(printer, "description")
	if description != "" && description != displayName {
		lines = append(lines, fmt.Sprintf("  Description: %s", description))
	}

	lines = append(lines, fmt.Sprintf("  Type: %s", printerType))

	if printerType == "network" {
		host := getStringValue(printer, "host")
		port := getIntValue(printer, "port")
		if host != "" && port > 0 {
			lines = append(lines, fmt.Sprintf("  Host: %s:%d", host, port))
		}
	} else if printerType == "serial" {
		device := getStringValue(printer, "device")
		if device != "" {
			lines = append(lines, fmt.Sprintf("  Device: %s", device))
		}
	} else if printerType == "usb" {
		vid := getIntValue(printer, "vid")
		pid := getIntValue(printer, "pid")
		if vid > 0 && pid > 0 {
			lines = append(lines, fmt.Sprintf("  VID: 0x%04X", vid))
			lines = append(lines, fmt.Sprintf("  PID: 0x%04X", pid))
		}
		device := getStringValue(printer, "device")
		if device != "" {
			lines = append(lines, fmt.Sprintf("  Device: %s", device))
		}
	}

	return lines
}

// formatJobLine formats a job map into display lines (one field per line)
func formatJobLine(job map[string]interface{}) []string {
	var lines []string
	jobID := getStringValue(job, "id")
	status := getStringValue(job, "status")
	printerID := getStringValue(job, "printer_id")
	retries := getIntValue(job, "retries")
	createdAt := getStringValue(job, "created_at")
	errorMsg := getStringValue(job, "error")

	lines = append(lines, fmt.Sprintf("  ID: %s", jobID))
	lines = append(lines, fmt.Sprintf("  Status: %s", status))
	lines = append(lines, fmt.Sprintf("  Printer ID: %s", printerID))
	lines = append(lines, fmt.Sprintf("  Retries: %d", retries))
	if createdAt != "" {
		lines = append(lines, fmt.Sprintf("  Created: %s", createdAt))
	}
	if errorMsg != "" {
		lines = append(lines, ErrorStyle.Render(fmt.Sprintf("  Error: %s", errorMsg)))
	}

	return lines
}
