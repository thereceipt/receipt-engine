package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thereceipt/receipt-engine/internal/printer"
)

// PrintersModel handles the printers tab
type PrintersModel struct {
	manager      *printer.Manager
	printers     []*printer.Printer
	cursor       int
	scrollOffset int // Track scroll position
	width        int
	height       int

	// Add network printer mode
	addingPrinter bool
	hostInput     textinput.Model
	portInput     textinput.Model
	inputFocus    int // 0 = host, 1 = port
	message       string
	messageType   string
}

// NewPrintersModel creates a new printers model
func NewPrintersModel(manager *printer.Manager) PrintersModel {
	hostInput := textinput.New()
	hostInput.Placeholder = "192.168.1.100"
	hostInput.CharLimit = 45
	hostInput.Width = 30

	portInput := textinput.New()
	portInput.Placeholder = "9100"
	portInput.CharLimit = 5
	portInput.Width = 10

	return PrintersModel{
		manager:   manager,
		printers:  make([]*printer.Printer, 0),
		hostInput: hostInput,
		portInput: portInput,
	}
}

// SetSize sets the component size
func (m *PrintersModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.adjustScroll() // Adjust scroll when size changes
}

// Refresh refreshes the printer list (synchronous, may block)
func (m *PrintersModel) Refresh() {
	printers, _ := m.manager.DetectPrinters()
	m.SetPrinters(printers)
}

// SetPrinters sets the printer list (thread-safe update)
func (m *PrintersModel) SetPrinters(printers []*printer.Printer) {
	m.printers = printers
	if m.cursor >= len(m.printers) && len(m.printers) > 0 {
		m.cursor = len(m.printers) - 1
	}
	m.adjustScroll()
}

// Update handles messages
func (m PrintersModel) Update(msg tea.Msg) (PrintersModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Calculate which printer was clicked
			// Account for title and header spacing (approximately 3 lines)
			clickY := msg.Y - 3
			if clickY >= 0 {
				// Adjust for scroll offset
				actualIdx := m.scrollOffset + clickY
				if actualIdx >= 0 && actualIdx < len(m.printers) {
					m.cursor = actualIdx
					m.adjustScroll()
				}
			}
		} else if msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
			// Handle mouse wheel scrolling
			if msg.Button == tea.MouseButtonWheelUp && m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			} else if msg.Button == tea.MouseButtonWheelDown && m.cursor < len(m.printers)-1 {
				m.cursor++
				m.adjustScroll()
			}
		}
	case tea.KeyMsg:
		if m.addingPrinter {
			return m.updateAddMode(msg)
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
		case "down", "j":
			if m.cursor < len(m.printers)-1 {
				m.cursor++
				m.adjustScroll()
			}
		case "r":
			m.Refresh()
			m.message = "Refreshed printer list"
			m.messageType = "success"
		case "a":
			m.addingPrinter = true
			m.hostInput.Focus()
			m.inputFocus = 0
			m.message = ""
		case "n":
			// Rename printer
			if len(m.printers) > 0 && m.cursor < len(m.printers) {
				p := m.printers[m.cursor]
				// Toggle between name and ID display
				if p.Name == "" {
					m.manager.SetPrinterName(p.ID, p.Description)
					m.message = "Set printer name"
				} else {
					m.manager.SetPrinterName(p.ID, "")
					m.message = "Cleared printer name"
				}
				m.messageType = "success"
				m.Refresh()
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m PrintersModel) updateAddMode(msg tea.KeyMsg) (PrintersModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.addingPrinter = false
		m.hostInput.Reset()
		m.portInput.Reset()
		m.message = ""
		return m, nil

	case "tab", "down":
		if m.inputFocus == 0 {
			m.inputFocus = 1
			m.hostInput.Blur()
			m.portInput.Focus()
		} else {
			m.inputFocus = 0
			m.portInput.Blur()
			m.hostInput.Focus()
		}
		return m, nil

	case "shift+tab", "up":
		if m.inputFocus == 1 {
			m.inputFocus = 0
			m.portInput.Blur()
			m.hostInput.Focus()
		} else {
			m.inputFocus = 1
			m.hostInput.Blur()
			m.portInput.Focus()
		}
		return m, nil

	case "enter":
		host := strings.TrimSpace(m.hostInput.Value())
		portStr := strings.TrimSpace(m.portInput.Value())

		if host == "" {
			m.message = "Host is required"
			m.messageType = "error"
			return m, nil
		}

		port := 9100
		if portStr != "" {
			fmt.Sscanf(portStr, "%d", &port)
		}

		// Add the printer
		desc := fmt.Sprintf("Network: %s:%d", host, port)
		m.manager.AddNetworkPrinter(host, port, desc)

		m.addingPrinter = false
		m.hostInput.Reset()
		m.portInput.Reset()
		m.message = fmt.Sprintf("Added printer %s:%d", host, port)
		m.messageType = "success"
		m.Refresh()
		return m, nil
	}

	// Update focused input
	if m.inputFocus == 0 {
		m.hostInput, cmd = m.hostInput.Update(msg)
	} else {
		m.portInput, cmd = m.portInput.Update(msg)
	}

	return m, cmd
}

// View renders the printers tab
func (m PrintersModel) View() string {
	var b strings.Builder

	// Calculate available lines
	availableLines := m.height - 4 // Reserve space for title, spacing, message
	if m.message != "" {
		availableLines -= 2
	}

	// Title
	title := CardTitleStyle.Render("Connected Printers")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.addingPrinter {
		return m.viewAddMode(&b)
	}

	if len(m.printers) == 0 {
		b.WriteString(TextMuted.Render("No printers detected.\n"))
		b.WriteString(TextMuted.Render("Press ") + HelpKeyStyle.Render("a") + TextMuted.Render(" to add a network printer\n"))
		b.WriteString(TextMuted.Render("Press ") + HelpKeyStyle.Render("r") + TextMuted.Render(" to refresh\n"))
	} else {
		// Limit printers shown to available lines
		maxPrinters := availableLines
		if maxPrinters < 0 {
			maxPrinters = 0
		}

		// Use scroll offset for proper scrolling
		startIdx := m.scrollOffset
		endIdx := startIdx + maxPrinters
		if endIdx > len(m.printers) {
			endIdx = len(m.printers)
		}

		for i := startIdx; i < endIdx; i++ {
			p := m.printers[i]
			cursor := "  "
			style := ListItemStyle
			if i == m.cursor {
				cursor = "▸ "
				style = SelectedItemStyle
			}

			name := p.Name
			if name == "" {
				name = p.Description
			}
			if name == "" {
				name = p.ID
			}

			// Type badge
			typeBadge := lipgloss.NewStyle().
				Foreground(Secondary).
				Render(fmt.Sprintf("[%s]", strings.ToUpper(p.Type)))

			// Status icon
			status := StatusIcon("online")

			// Connection info
			var connInfo string
			if p.Type == "network" {
				connInfo = fmt.Sprintf("%s:%d", p.Host, p.Port)
			} else if p.Device != "" {
				connInfo = p.Device
			}

			line := fmt.Sprintf("%s%s %s %s", cursor, status, name, typeBadge)
			if connInfo != "" {
				line += TextMuted.Render(fmt.Sprintf(" • %s", connInfo))
			}

			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}

		// Show scroll indicator if needed
		if len(m.printers) > maxPrinters {
			if m.scrollOffset > 0 {
				b.WriteString(TextMuted.Render("  ... (↑ to scroll) ...\n"))
			}
			if endIdx < len(m.printers) {
				b.WriteString(TextMuted.Render("  ... (↓ to scroll) ...\n"))
			}
		}
	}

	// Message
	if m.message != "" {
		b.WriteString("\n")
		switch m.messageType {
		case "success":
			b.WriteString(SuccessStyle.Render("✓ " + m.message))
		case "error":
			b.WriteString(ErrorStyle.Render("✗ " + m.message))
		default:
			b.WriteString(InfoStyle.Render("ℹ " + m.message))
		}
	}

	return b.String()
}

func (m PrintersModel) viewAddMode(b *strings.Builder) string {
	b.WriteString(InfoStyle.Render("Add Network Printer"))
	b.WriteString("\n\n")

	// Host input
	if m.inputFocus == 0 {
		b.WriteString(InputLabelFocusedStyle.Render("Host"))
	} else {
		b.WriteString(InputLabelStyle.Render("Host"))
	}
	b.WriteString("\n")
	if m.inputFocus == 0 {
		b.WriteString(InputFocusedStyle.Render(m.hostInput.View()))
	} else {
		b.WriteString(InputStyle.Render(m.hostInput.View()))
	}
	b.WriteString("\n\n")

	// Port input
	if m.inputFocus == 1 {
		b.WriteString(InputLabelFocusedStyle.Render("Port"))
	} else {
		b.WriteString(InputLabelStyle.Render("Port"))
	}
	b.WriteString("\n")
	if m.inputFocus == 1 {
		b.WriteString(InputFocusedStyle.Render(m.portInput.View()))
	} else {
		b.WriteString(InputStyle.Render(m.portInput.View()))
	}
	b.WriteString("\n\n")

	b.WriteString(TextMuted.Render("Enter to add • Esc to cancel"))

	// Error message
	if m.message != "" && m.messageType == "error" {
		b.WriteString("\n\n")
		b.WriteString(ErrorStyle.Render("✗ " + m.message))
	}

	return b.String()
}

// Help returns help text for this tab
func (m PrintersModel) Help() string {
	if m.addingPrinter {
		return RenderHelp("enter", "add") + "  " +
			RenderHelp("tab", "next") + "  " +
			RenderHelp("esc", "cancel")
	}
	return RenderHelp("↑/↓", "select") + "  " +
		RenderHelp("click", "select") + "  " +
		RenderHelp("a", "add") + "  " +
		RenderHelp("r", "refresh")
}

// adjustScroll adjusts the scroll offset to keep cursor visible
func (m *PrintersModel) adjustScroll() {
	if len(m.printers) == 0 {
		m.scrollOffset = 0
		return
	}

	maxVisible := m.height - 4 // Account for title, spacing, message
	if maxVisible < 1 {
		maxVisible = 1
	}

	// If cursor is above visible area, scroll up
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}

	// If cursor is below visible area, scroll down
	visibleEnd := m.scrollOffset + maxVisible
	if m.cursor >= visibleEnd {
		m.scrollOffset = m.cursor - maxVisible + 1
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
	}

	// Ensure scroll offset doesn't go beyond list
	maxOffset := len(m.printers) - maxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}
