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
	entries    []fileEntry // unfiltered entries for currentDir
	files      []fileEntry // filtered view (what we render/navigate)
	fileCursor int
	fileScroll int

	// File browser UX
	showHidden   bool
	showAllFiles bool // if false, show dirs + .receipt only
	filtering    bool
	filterInput  textinput.Model
	toolbarFocus bool // when true, keyboard focus is in the file toolbar
	toolbarIndex int  // 0=mode, 1=hidden, 2=filter

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

	filter := textinput.New()
	filter.Prompt = "/"
	filter.Placeholder = "filter…"
	filter.CharLimit = 80
	filter.Width = 30
	filter.PromptStyle = lipgloss.NewStyle().Foreground(Secondary).Bold(true)

	m := PrintModel{
		manager:      manager,
		queue:        queue,
		currentDir:   cwd,
		focus:        0,
		showAllFiles: false,
		showHidden:   false,
		filterInput:  filter,
		toolbarFocus: false,
		toolbarIndex: 0,
	}

	m.rebuildEntries()
	m.refreshPrinters()

	return m
}

// SetSize sets the component size
func (m *PrintModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *PrintModel) refreshFiles() {
	// Back-compat shim: keep old call sites working
	m.rebuildEntries()
}

func (m *PrintModel) rebuildEntries() {
	m.entries = []fileEntry{}

	// Parent directory
	if m.currentDir != "/" {
		m.entries = append(m.entries, fileEntry{name: "..", isDir: true})
	}

	dirEntries, err := os.ReadDir(m.currentDir)
	if err != nil {
		return
	}

	// Directories first
	for _, e := range dirEntries {
		if !e.IsDir() {
			continue
		}
		if !m.showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		m.entries = append(m.entries, fileEntry{name: e.Name(), isDir: true})
	}

	// Then files
	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !m.showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		if !m.showAllFiles {
			// Default mode: show only .receipt files (plus dirs).
			if !strings.HasSuffix(strings.ToLower(name), ".receipt") {
				continue
			}
		}

		m.entries = append(m.entries, fileEntry{name: name, isDir: false})
	}

	m.applyFileFilter()
}

func (m *PrintModel) applyFileFilter() {
	filter := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if filter == "" {
		m.files = append([]fileEntry(nil), m.entries...)
	} else {
		out := make([]fileEntry, 0, len(m.entries))
		for _, e := range m.entries {
			if e.name == ".." {
				out = append(out, e)
				continue
			}
			if strings.Contains(strings.ToLower(e.name), filter) {
				out = append(out, e)
			}
		}
		m.files = out
	}

	if m.fileCursor >= len(m.files) {
		m.fileCursor = 0
	}
	if m.fileCursor < 0 {
		m.fileCursor = 0
	}
	m.adjustFileScroll()
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
	case tea.KeyMsg:
		// File toolbar focus (vim-tree-ish)
		if m.focus == 0 && m.toolbarFocus && !m.filtering {
			switch msg.String() {
			case "esc":
				m.toolbarFocus = false
				return m, nil
			case "left", "h":
				if m.toolbarIndex > 0 {
					m.toolbarIndex--
				} else {
					m.toolbarIndex = 2
				}
				return m, nil
			case "right", "l":
				m.toolbarIndex = (m.toolbarIndex + 1) % 3
				return m, nil
			case "down", "j":
				m.toolbarFocus = false
				return m, nil
			case "enter":
				switch m.toolbarIndex {
				case 0: // mode
					m.showAllFiles = !m.showAllFiles
					m.rebuildEntries()
					if m.showAllFiles {
						m.message = "Showing all files"
					} else {
						m.message = "Showing .receipt only"
					}
					m.msgType = "success"
				case 1: // hidden
					m.showHidden = !m.showHidden
					m.rebuildEntries()
					if m.showHidden {
						m.message = "Showing hidden"
					} else {
						m.message = "Hiding hidden"
					}
					m.msgType = "success"
				case 2: // filter
					m.filtering = true
					m.filterInput.Focus()
				}
				return m, nil
			}
		}

		// File filter mode (vim-like)
		if m.filtering && m.focus == 0 {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterInput.Blur()
				return m, nil
			case "enter":
				m.filtering = false
				m.filterInput.Blur()
				return m, nil
			default:
				before := m.filterInput.Value()
				m.filterInput, cmd = m.filterInput.Update(msg)
				if m.filterInput.Value() != before {
					m.applyFileFilter()
				}
				return m, cmd
			}
		}

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
		case "/":
			// Start file filter
			if m.focus == 0 {
				m.toolbarFocus = false
				m.filtering = true
				m.filterInput.Focus()
			}
		case "esc":
			// Clear filter (if any) without entering filter mode
			if m.focus == 0 && !m.filtering && m.filterInput.Value() != "" {
				m.filterInput.SetValue("")
				m.applyFileFilter()
			}
		case ".":
			// Toggle hidden files/dirs
			if m.focus == 0 {
				m.toolbarFocus = false
				m.showHidden = !m.showHidden
				m.rebuildEntries()
				if m.showHidden {
					m.message = "Showing hidden"
				} else {
					m.message = "Hiding hidden"
				}
				m.msgType = "success"
			}
		case "f":
			// Toggle showing all files vs .receipt only
			if m.focus == 0 {
				m.toolbarFocus = false
				m.showAllFiles = !m.showAllFiles
				m.rebuildEntries()
				if m.showAllFiles {
					m.message = "Showing all files"
				} else {
					m.message = "Showing .receipt only"
				}
				m.msgType = "success"
			}
		case "~":
			// Jump to home directory
			if m.focus == 0 {
				m.toolbarFocus = false
				if home, err := os.UserHomeDir(); err == nil && home != "" {
					m.currentDir = home
					m.fileCursor = 0
					m.fileScroll = 0
					m.rebuildEntries()
					m.message = "Home"
					m.msgType = "success"
				}
			}
		case "r":
			m.rebuildEntries()
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
		if m.filtering {
			return
		}
		if m.toolbarFocus {
			// already at toolbar; do nothing
			return
		}
		if m.fileCursor > 0 {
			m.fileCursor--
			m.adjustFileScroll()
		} else {
			// move focus into toolbar (neovim tree vibe)
			m.toolbarFocus = true
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
		if m.filtering {
			return
		}
		if m.toolbarFocus {
			m.toolbarFocus = false
			return
		}
		if m.fileCursor < len(m.files)-1 {
			m.fileCursor++
			m.adjustFileScroll()
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
				m.fileScroll = 0
				m.rebuildEntries()
			} else {
				// Only load .receipt files; other files can be shown for navigation when showAllFiles is enabled.
				if strings.HasSuffix(strings.ToLower(f.name), ".receipt") {
					m.loadReceipt(filepath.Join(m.currentDir, f.name))
				} else {
					m.message = "Not a .receipt file"
					m.msgType = "error"
				}
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
	// IMPORTANT: the app wraps tab content in ContentStyle which has Padding(1, 2).
	// So the true inner viewport is smaller than m.width/m.height.
	effW := m.width - 4  // left+right padding (2 each)
	effH := m.height - 2 // top+bottom padding (1 each)
	if effW < 20 {
		effW = 20
	}
	if effH < 8 {
		effH = 8
	}

	// Header: CardTitleStyle has MarginBottom(1) which creates too much vertical gap here.
	// Use a no-margin variant so the page is vertically tight.
	header := CardTitleStyle.Copy().MarginBottom(0).Render("Print Receipt")

	// Message (if any) — rendered as its own block so we can size the panels precisely.
	messageView := ""
	if m.message != "" {
		switch m.msgType {
		case "success":
			messageView = SuccessStyle.Render("✓ " + m.message)
		case "error":
			messageView = ErrorStyle.Render("✗ " + m.message)
		default:
			messageView = InfoStyle.Render("ℹ " + m.message)
		}
	}

	headerHeight := lipgloss.Height(header)
	messageHeight := lipgloss.Height(messageView)

	// Compute remaining height using real rendered heights.
	// If we show a message, reserve 1 blank line for separation.
	reserved := headerHeight
	if messageView != "" {
		reserved += 1 + messageHeight
	}
	columnsHeight := effH - reserved
	if columnsHeight < 8 {
		columnsHeight = 8
	}

	// Create two columns (wider, Neovim-tree-like file panel)
	leftWidth := int(float64(effW) * 0.42)
	if leftWidth < 34 {
		leftWidth = 34
	}
	if leftWidth > 60 {
		leftWidth = 60
	}
	// Ensure right side doesn't collapse
	if effW-leftWidth-6 < 40 {
		leftWidth = maxInt(34, effW-6-40)
	}
	rightWidth := effW - leftWidth - 6

	// Left column: File browser
	filesView := m.renderFiles(leftWidth, columnsHeight)

	// Right column: Receipt info, printer, variables
	rightView := m.renderRight(rightWidth, columnsHeight)

	// Join columns
	columns := lipgloss.JoinHorizontal(
		lipgloss.Top,
		filesView,
		"  ",
		rightView,
	)

	var out string
	if messageView != "" {
		out = lipgloss.JoinVertical(lipgloss.Left, header, columns, "", messageView)
	} else {
		out = lipgloss.JoinVertical(lipgloss.Left, header, columns)
	}

	// Hard clamp to exactly the inner viewport height; outer padding is handled by ContentStyle.
	lines := strings.Split(out, "\n")
	if len(lines) > effH {
		lines = lines[:effH]
	} else {
		for len(lines) < effH {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (m PrintModel) renderFiles(width, height int) string {
	focused := m.focus == 0

	// --- Toolbar header (modern, chip-like, tasteful) ---
	chip := func(text string, fg, bg lipgloss.Color, bold bool, selected bool) string {
		// Keep chips tight; separators provide spacing.
		s := lipgloss.NewStyle().Foreground(fg).Background(bg).Padding(0, 0)
		if bold {
			s = s.Bold(true)
		}
		if selected {
			// Focus ring feel without overdoing it.
			s = s.Background(Secondary).Foreground(colorTextBright).Bold(true)
		}
		return s.Render(text)
	}

	titleChip := chip("FILES", colorTextBright, Secondary, true, false)

	modeChip := "RCPT"
	if m.showAllFiles {
		modeChip = "ALL"
	}
	mode := chip(modeChip, colorTextBright, BgHover, true, focused && m.toolbarFocus && m.toolbarIndex == 0)

	hiddenChip := "HID:OFF"
	hiddenBg := BgHover
	if m.showHidden {
		hiddenChip = "HID:ON"
		hiddenBg = Primary
	}
	hidden := chip(hiddenChip, colorTextBright, hiddenBg, true, focused && m.toolbarFocus && m.toolbarIndex == 1)

	filterChip := "FILTER:/"
	if strings.TrimSpace(m.filterInput.Value()) != "" {
		filterChip = "FILTER:/" + Truncate(m.filterInput.Value(), 18)
	}
	filterBg := BgHover
	if m.filtering && focused {
		filterBg = Warning
	}
	filter := chip(filterChip, colorTextBright, filterBg, false, focused && m.toolbarFocus && m.toolbarIndex == 2)

	toolbar := lipgloss.JoinHorizontal(lipgloss.Top, titleChip, "  ", mode, "  ", hidden, "  ", filter)

	pathLine := TextMuted.Render(Truncate(m.currentDir, width-2))

	// Optional interactive filter prompt line
	filterLine := ""
	if m.filtering && focused {
		filterLine = InputFocusedStyle.Copy().
			Border(lipgloss.NormalBorder()).
			BorderForeground(Secondary).
			Padding(0, 1).
			Width(width - 4).
			Render(m.filterInput.View())
	}

	// Calculate list viewport height.
	headerLines := 2 // toolbar + path
	if filterLine != "" {
		headerLines += 2 // filter input box line(s)
	}
	// Panel uses border + padding; keep this simple and conservative.
	available := height - headerLines - 3
	if available < 5 {
		available = 5
	}

	// Visible window for scrolling
	start := m.fileScroll
	end := start + available
	if end > len(m.files) {
		end = len(m.files)
	}

	var list strings.Builder
	for i := start; i < end; i++ {
		f := m.files[i]

		// Tree-like prefix (ASCII-only)
		isLast := i == end-1
		branch := "|-- "
		if isLast {
			branch = "`-- "
		}

		cursor := "  "
		style := ListItemStyle
		if i == m.fileCursor {
			if focused {
				// Tastier highlight than generic hover.
				style = lipgloss.NewStyle().
					Foreground(colorTextBright).
					Background(Primary).
					Bold(true).
					PaddingLeft(1).
					PaddingRight(1)
				cursor = "> "
			} else {
				// Unfocused selection stays subtle.
				style = lipgloss.NewStyle().
					Foreground(colorTextBright).
					Background(BgHover).
					PaddingLeft(1).
					PaddingRight(1)
				cursor = "  "
			}
		}

		kind := "[F]"
		if f.isDir {
			kind = "[D]"
		} else if strings.HasSuffix(strings.ToLower(f.name), ".receipt") {
			kind = "[R]"
		}

		name := f.name
		// Subtle semantic coloring (no background spam).
		nameStyle := TextNormal
		if f.isDir {
			nameStyle = lipgloss.NewStyle().Foreground(Secondary).Bold(true)
		} else if strings.HasSuffix(strings.ToLower(f.name), ".receipt") {
			nameStyle = lipgloss.NewStyle().Foreground(Primary).Bold(true)
		}
		name = nameStyle.Render(Truncate(name, width-12))

		line := fmt.Sprintf("%s%s%s %s", cursor, branch, kind, name)
		list.WriteString(style.Render(line))
		list.WriteString("\n")
	}

	if len(m.files) == 0 {
		empty := "No files"
		if !m.showAllFiles {
			empty = "No .receipt files"
		}
		list.WriteString(TextMuted.Render(empty))
		list.WriteString("\n")
	}

	body := toolbar + "\n" + pathLine
	if filterLine != "" {
		body += "\n" + filterLine
	}
	body += "\n" + list.String()

	panelStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BgHover).
		// Use the standard content background (no explicit background here).
		Padding(0, 1)

	panel := panelStyle.Render(body)

	return panel
}

func (m PrintModel) renderRight(width, height int) string {
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

	// Make right side match left side height for a clean split layout.
	return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
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

func (m *PrintModel) adjustFileScroll() {
	// Calculate a reasonable viewport size for the file list based on current height.
	// The exact number isn't critical; it just keeps the cursor visible.
	viewport := m.height - 8
	if viewport < 5 {
		viewport = 5
	}
	if m.fileCursor < m.fileScroll {
		m.fileScroll = m.fileCursor
	}
	if m.fileCursor >= m.fileScroll+viewport {
		m.fileScroll = m.fileCursor - viewport + 1
	}
	if m.fileScroll < 0 {
		m.fileScroll = 0
	}
	maxScroll := len(m.files) - viewport
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.fileScroll > maxScroll {
		m.fileScroll = maxScroll
	}
}

// Help returns help text for this tab
func (m PrintModel) Help() string {
	if m.inputFocused {
		return RenderHelp("enter", "confirm") + "  " +
			RenderHelp("tab", "next") + "  " +
			RenderHelp("esc", "done")
	}
	return RenderHelp("↑/↓", "select") + "  " +
		RenderHelp("←/→", "section") + "  " +
		RenderHelp("enter", "action") + "  " +
		RenderHelp("↑@top", "toolbar") + "  " +
		RenderHelp("/", "filter") + "  " +
		RenderHelp(".", "hidden") + "  " +
		RenderHelp("f", "all files") + "  " +
		RenderHelp("~", "home") + "  " +
		RenderHelp("p", "print")
}
