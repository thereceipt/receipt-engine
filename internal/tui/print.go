package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thereceipt/receipt-engine/internal/parser"
	"github.com/thereceipt/receipt-engine/internal/printer"
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

// PrintModel handles the print tab
type PrintModel struct {
	manager *printer.Manager
	queue   *printer.PrintQueue
	width   int
	height  int

	// File browser
	currentDir string
	files      []fileEntry
	fileCursor int

	// Printer selection
	printers      []*printer.Printer
	printerCursor int

	// Loaded receipt
	receipt     *receiptformat.Receipt
	receiptPath string

	// Variable inputs
	variables    []variableInput
	varCursor    int
	inputFocused bool

	// Focus state: 0=files, 1=printers, 2=variables, 3=print button
	focus int

	// Messages
	message string
	msgType string
}

type fileEntry struct {
	name  string
	isDir bool
}

type variableInput struct {
	name       string
	valueType  string
	input      textinput.Model
	defaultVal string
}

// NewPrintModel creates a new print model
func NewPrintModel(manager *printer.Manager, queue *printer.PrintQueue) PrintModel {
	cwd, _ := os.Getwd()

	m := PrintModel{
		manager:    manager,
		queue:      queue,
		currentDir: cwd,
		focus:      0,
	}

	m.refreshFiles()
	m.refreshPrinters()

	return m
}

// SetSize sets the component size
func (m *PrintModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *PrintModel) refreshFiles() {
	m.files = []fileEntry{}

	// Add parent directory
	if m.currentDir != "/" {
		m.files = append(m.files, fileEntry{name: "..", isDir: true})
	}

	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		return
	}

	// Directories first
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			m.files = append(m.files, fileEntry{name: e.Name(), isDir: true})
		}
	}

	// Then .receipt files
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".receipt") {
			m.files = append(m.files, fileEntry{name: e.Name(), isDir: false})
		}
	}

	if m.fileCursor >= len(m.files) {
		m.fileCursor = 0
	}
}

func (m *PrintModel) refreshPrinters() {
	m.printers, _ = m.manager.DetectPrinters()
	if m.printerCursor >= len(m.printers) && len(m.printers) > 0 {
		m.printerCursor = len(m.printers) - 1
	}
}

func (m *PrintModel) loadReceipt(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		m.message = fmt.Sprintf("Error reading file: %v", err)
		m.msgType = "error"
		return
	}

	receipt, err := receiptformat.Parse(data)
	if err != nil {
		m.message = fmt.Sprintf("Error parsing receipt: %v", err)
		m.msgType = "error"
		return
	}

	m.receipt = receipt
	m.receiptPath = path

	// Setup variable inputs
	m.variables = []variableInput{}
	for _, v := range receipt.Variables {
		input := textinput.New()
		input.Placeholder = v.ValueType
		input.CharLimit = 100
		input.Width = 25

		defaultVal := ""
		if v.DefaultValue != nil {
			defaultVal = fmt.Sprintf("%v", v.DefaultValue)
			input.SetValue(defaultVal)
		}

		m.variables = append(m.variables, variableInput{
			name:       v.Let,
			valueType:  v.ValueType,
			input:      input,
			defaultVal: defaultVal,
		})
	}

	m.message = fmt.Sprintf("Loaded: %s", filepath.Base(path))
	m.msgType = "success"
}

func (m *PrintModel) print() {
	if m.receipt == nil {
		m.message = "No receipt loaded"
		m.msgType = "error"
		return
	}

	if len(m.printers) == 0 {
		m.message = "No printers available"
		m.msgType = "error"
		return
	}

	selectedPrinter := m.printers[m.printerCursor]

	// Collect variables
	variableData := make(map[string]interface{})
	for _, v := range m.variables {
		value := strings.TrimSpace(v.input.Value())
		if value != "" {
			var convertedValue interface{} = value
			if v.valueType == "number" || v.valueType == "double" {
				var num float64
				if _, err := fmt.Sscanf(value, "%f", &num); err == nil {
					convertedValue = num
				}
			} else if v.valueType == "boolean" {
				convertedValue = strings.ToLower(value) == "true" || value == "1"
			}
			variableData[v.name] = convertedValue
		}
	}

	// Create parser
	paperWidth := m.receipt.PaperWidth
	if paperWidth == "" {
		paperWidth = "80mm"
	}

	pars, err := parser.New(m.receipt, paperWidth)
	if err != nil {
		m.message = fmt.Sprintf("Parser error: %v", err)
		m.msgType = "error"
		return
	}

	// Set variables
	if len(variableData) > 0 {
		pars.SetVariableData(variableData)
	}

	// Execute
	img, err := pars.Execute()
	if err != nil {
		m.message = fmt.Sprintf("Render error: %v", err)
		m.msgType = "error"
		return
	}

	// Queue print job
	jobID := m.queue.Enqueue(selectedPrinter.ID, img)

	printerName := selectedPrinter.Name
	if printerName == "" {
		printerName = selectedPrinter.Description
	}

	m.message = fmt.Sprintf("Queued: %s → %s", Truncate(jobID, 12), printerName)
	m.msgType = "success"
}

// Update handles messages
func (m PrintModel) Update(msg tea.Msg) (PrintModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Handle clicks in different sections
			// Files section is typically on the left, printers in middle, variables on right
			// This is approximate - you may need to adjust based on actual layout
			sectionWidth := m.width / 3

			if msg.X < sectionWidth {
				// Clicked in files section
				clickY := msg.Y - 3 // Account for title
				if clickY >= 0 && clickY < len(m.files) {
					m.fileCursor = clickY
					m.focus = 0
				}
			} else if msg.X < sectionWidth*2 {
				// Clicked in printers section
				clickY := msg.Y - 3 // Account for title
				if clickY >= 0 && clickY < len(m.printers) {
					m.printerCursor = clickY
					m.focus = 1
				}
			} else {
				// Clicked in variables section
				clickY := msg.Y - 3 // Account for title
				if clickY >= 0 && clickY < len(m.variables) {
					m.varCursor = clickY
					m.focus = 2
					if !m.inputFocused {
						m.inputFocused = true
						m.variables[m.varCursor].input.Focus()
					}
				}
			}
		} else if msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
			// Handle mouse wheel scrolling based on current focus
			if msg.Button == tea.MouseButtonWheelUp {
				m.navigateUp()
			} else {
				m.navigateDown()
			}
		}
	case tea.KeyMsg:
		// Handle input mode
		if m.inputFocused && m.focus == 2 && len(m.variables) > 0 {
			switch msg.String() {
			case "esc":
				m.inputFocused = false
				m.variables[m.varCursor].input.Blur()
				return m, nil
			case "enter":
				m.inputFocused = false
				m.variables[m.varCursor].input.Blur()
				return m, nil
			case "tab":
				m.variables[m.varCursor].input.Blur()
				m.varCursor = (m.varCursor + 1) % len(m.variables)
				m.variables[m.varCursor].input.Focus()
				return m, nil
			default:
				m.variables[m.varCursor].input, cmd = m.variables[m.varCursor].input.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "up", "k":
			m.navigateUp()
		case "down", "j":
			m.navigateDown()
		case "left", "h":
			if m.focus > 0 {
				m.focus--
			}
		case "right", "l":
			if m.focus < 3 {
				m.focus++
			}
		case "enter":
			m.handleEnter()
		case "r":
			m.refreshFiles()
			m.refreshPrinters()
			m.message = "Refreshed"
			m.msgType = "success"
		case "p":
			m.print()
		}
	}

	return m, cmd
}

func (m *PrintModel) navigateUp() {
	switch m.focus {
	case 0: // Files
		if m.fileCursor > 0 {
			m.fileCursor--
		}
	case 1: // Printers
		if m.printerCursor > 0 {
			m.printerCursor--
		}
	case 2: // Variables
		if m.varCursor > 0 {
			m.varCursor--
		}
	}
}

func (m *PrintModel) navigateDown() {
	switch m.focus {
	case 0: // Files
		if m.fileCursor < len(m.files)-1 {
			m.fileCursor++
		}
	case 1: // Printers
		if m.printerCursor < len(m.printers)-1 {
			m.printerCursor++
		}
	case 2: // Variables
		if m.varCursor < len(m.variables)-1 {
			m.varCursor++
		}
	}
}

func (m *PrintModel) handleEnter() {
	switch m.focus {
	case 0: // Files
		if len(m.files) > 0 && m.fileCursor < len(m.files) {
			f := m.files[m.fileCursor]
			if f.isDir {
				if f.name == ".." {
					m.currentDir = filepath.Dir(m.currentDir)
				} else {
					m.currentDir = filepath.Join(m.currentDir, f.name)
				}
				m.fileCursor = 0
				m.refreshFiles()
			} else {
				m.loadReceipt(filepath.Join(m.currentDir, f.name))
			}
		}
	case 2: // Variables
		if len(m.variables) > 0 {
			m.inputFocused = true
			m.variables[m.varCursor].input.Focus()
		}
	case 3: // Print button
		m.print()
	}
}

// View renders the print tab
func (m PrintModel) View() string {
	var b strings.Builder

	// Title
	title := CardTitleStyle.Render("Print Receipt")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Create two columns
	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth - 6

	// Left column: File browser
	filesView := m.renderFiles(leftWidth)

	// Right column: Receipt info, printer, variables
	rightView := m.renderRight(rightWidth)

	// Join columns
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		filesView,
		"  ",
		rightView,
	)
	b.WriteString(content)

	// Message
	if m.message != "" {
		b.WriteString("\n\n")
		switch m.msgType {
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

func (m PrintModel) renderFiles(width int) string {
	var b strings.Builder

	// Header
	focused := m.focus == 0
	if focused {
		b.WriteString(SectionHeaderStyle.Copy().Foreground(Secondary).Render("FILES"))
	} else {
		b.WriteString(SectionHeaderStyle.Render("FILES"))
	}
	b.WriteString("\n")
	b.WriteString(TextMuted.Render(Truncate(m.currentDir, width-2)))
	b.WriteString("\n\n")

	// File list
	maxFiles := 15
	start := 0
	if m.fileCursor >= maxFiles {
		start = m.fileCursor - maxFiles + 1
	}

	for i := start; i < len(m.files) && i < start+maxFiles; i++ {
		f := m.files[i]
		cursor := "  "
		style := ListItemStyle
		if i == m.fileCursor && focused {
			cursor = "▸ "
			style = SelectedItemStyle
		}

		icon := "📄"
		if f.isDir {
			icon = "📁"
		}

		name := Truncate(f.name, width-8)
		b.WriteString(style.Render(fmt.Sprintf("%s%s %s", cursor, icon, name)))
		b.WriteString("\n")
	}

	if len(m.files) == 0 {
		b.WriteString(TextMuted.Render("  No .receipt files"))
	}

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m PrintModel) renderRight(width int) string {
	var b strings.Builder

	// Receipt info
	b.WriteString(m.renderReceiptInfo())
	b.WriteString("\n\n")

	// Printer selection
	b.WriteString(m.renderPrinterSelect())

	// Variables (if any)
	if len(m.variables) > 0 {
		b.WriteString("\n\n")
		b.WriteString(m.renderVariables())
	}

	// Print button
	b.WriteString("\n\n")
	b.WriteString(m.renderPrintButton())

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m PrintModel) renderReceiptInfo() string {
	var b strings.Builder

	b.WriteString(SectionHeaderStyle.Render("RECEIPT"))
	b.WriteString("\n")

	if m.receipt == nil {
		b.WriteString(TextMuted.Render("No receipt loaded"))
		b.WriteString("\n")
		b.WriteString(TextMuted.Render("Select a .receipt file"))
	} else {
		name := m.receipt.Name
		if name == "" {
			name = filepath.Base(m.receiptPath)
		}
		b.WriteString(TextBright.Render(name))
		b.WriteString("\n")

		if m.receipt.Description != "" {
			b.WriteString(TextMuted.Render(Truncate(m.receipt.Description, 40)))
			b.WriteString("\n")
		}

		paperWidth := m.receipt.PaperWidth
		if paperWidth == "" {
			paperWidth = "80mm"
		}
		b.WriteString(TextMuted.Render(fmt.Sprintf("%d commands • %s", len(m.receipt.Commands), paperWidth)))
	}

	return b.String()
}

func (m PrintModel) renderPrinterSelect() string {
	var b strings.Builder

	focused := m.focus == 1
	if focused {
		b.WriteString(SectionHeaderStyle.Copy().Foreground(Secondary).Render("PRINTER"))
	} else {
		b.WriteString(SectionHeaderStyle.Render("PRINTER"))
	}
	b.WriteString("\n")

	if len(m.printers) == 0 {
		b.WriteString(TextMuted.Render("No printers available"))
	} else {
		for i, p := range m.printers {
			cursor := "  "
			style := ListItemStyle
			if i == m.printerCursor {
				cursor = "▸ "
				if focused {
					style = SelectedItemStyle
				}
			}

			name := p.Name
			if name == "" {
				name = p.Description
			}
			if name == "" {
				name = p.ID
			}

			b.WriteString(style.Render(cursor + StatusIcon("online") + " " + Truncate(name, 30)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m PrintModel) renderVariables() string {
	var b strings.Builder

	focused := m.focus == 2
	if focused {
		b.WriteString(SectionHeaderStyle.Copy().Foreground(Secondary).Render("VARIABLES"))
	} else {
		b.WriteString(SectionHeaderStyle.Render("VARIABLES"))
	}
	b.WriteString("\n")

	for i, v := range m.variables {
		isSelected := i == m.varCursor && focused

		// Label
		label := fmt.Sprintf("%s (%s)", v.name, v.valueType)
		if isSelected {
			b.WriteString(InputLabelFocusedStyle.Render(label))
		} else {
			b.WriteString(InputLabelStyle.Render(label))
		}
		b.WriteString("\n")

		// Input
		inputView := v.input.View()
		if m.inputFocused && i == m.varCursor {
			b.WriteString(InputFocusedStyle.Render(inputView))
		} else if isSelected {
			b.WriteString(InputFocusedStyle.Render(inputView))
		} else {
			b.WriteString(InputStyle.Render(inputView))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m PrintModel) renderPrintButton() string {
	focused := m.focus == 3

	if focused {
		return ButtonStyle.Render("▶ Print Receipt")
	}
	return ButtonInactiveStyle.Render("▶ Print Receipt")
}

// Help returns help text for this tab
func (m PrintModel) Help() string {
	if m.inputFocused {
		return RenderHelp("enter", "confirm") + "  " +
			RenderHelp("tab", "next") + "  " +
			RenderHelp("esc", "done")
	}
	return RenderHelp("↑/↓", "select") + "  " +
		RenderHelp("click", "select") + "  " +
		RenderHelp("←/→", "section") + "  " +
		RenderHelp("enter", "action") + "  " +
		RenderHelp("p", "print")
}
