package tui

import (
	"fmt"
	"strings"

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
}

// Hide hides the command input
func (m *CommandModel) Hide() {
	m.visible = false
	m.input.Blur()
	m.input.SetValue("")
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
			}
			return m, cmd

		case "esc":
			// Hide command bar
			m.Hide()
			return m, nil

		case "up", "k":
			// Scroll up in results
			if m.scrollPos > 0 {
				m.scrollPos--
			}
			return m, nil

		case "down", "j":
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
			if m.lastResult.Data != nil {
				if printers, ok := m.lastResult.Data["printers"].([]interface{}); ok {
					resultLines = append(resultLines, "")
					resultLines = append(resultLines, SectionHeaderStyle.Render("Printers:"))
					for _, p := range printers {
						if printer, ok := p.(map[string]interface{}); ok {
							name := printer["name"]
							if name == "" || name == nil {
								name = printer["description"]
							}
							nameStr := fmt.Sprintf("%v", name)
							idStr := fmt.Sprintf("%v", printer["id"])
							typeStr := fmt.Sprintf("%v", printer["type"])
							line := fmt.Sprintf("  %s: %s (%s)",
								Truncate(idStr, 20), Truncate(nameStr, 30), typeStr)
							resultLines = append(resultLines, line)
						}
					}
				}
				if jobs, ok := m.lastResult.Data["jobs"].([]interface{}); ok {
					resultLines = append(resultLines, "")
					resultLines = append(resultLines, SectionHeaderStyle.Render("Jobs:"))
					for _, j := range jobs {
						if job, ok := j.(map[string]interface{}); ok {
							idStr := fmt.Sprintf("%v", job["id"])
							statusStr := fmt.Sprintf("%v", job["status"])
							printerIDStr := fmt.Sprintf("%v", job["printer_id"])
							line := fmt.Sprintf("  %s: %s (printer: %s)",
								Truncate(idStr, 20), statusStr, Truncate(printerIDStr, 20))
							resultLines = append(resultLines, line)
						}
					}
				}
				if jobID, ok := m.lastResult.Data["job_id"].(string); ok {
					resultLines = append(resultLines, "")
					resultLines = append(resultLines, InfoStyle.Render(fmt.Sprintf("Job ID: %s", jobID)))
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
	b.WriteString(TextMuted.Render(helpText))

	// Return the content - the overlay will handle height constraints
	return b.String()
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
