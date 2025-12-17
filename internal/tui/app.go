package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thereceipt/receipt-engine/internal/command"
	"github.com/thereceipt/receipt-engine/internal/printer"
)

// Tab represents a navigation tab
type Tab int

const (
	TabPrinters Tab = iota
	TabJobs
	TabPrint
)

func (t Tab) String() string {
	return []string{"Printers", "Jobs", "Print"}[t]
}

func (t Tab) Icon() string {
	return "" // No icons
}

// Messages
type tickMsg time.Time
type printerUpdateMsg struct {
	printers []*printer.Printer
}
type logMsg struct {
	message string
	level   string
}

// App is the main Bubble Tea model
type App struct {
	// Dependencies
	manager *printer.Manager
	pool    *printer.ConnectionPool
	queue   *printer.PrintQueue
	port    string

	// UI State
	activeTab Tab
	width     int
	height    int
	ready     bool
	quitting  bool

	// Logs
	logs    []logEntry
	maxLogs int

	// Components
	spinner  spinner.Model
	printers PrintersModel
	jobs     JobsModel
	printTab PrintModel
	command  CommandModel

	// Timing
	startTime time.Time
}

type logEntry struct {
	time    time.Time
	message string
	level   string
}

// NewApp creates a new Bubble Tea TUI application
func NewApp(manager *printer.Manager, pool *printer.ConnectionPool, queue *printer.PrintQueue, port string) *App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	executor := command.NewExecutor(manager, pool, queue)

	app := &App{
		manager:   manager,
		pool:      pool,
		queue:     queue,
		port:      port,
		activeTab: TabPrinters,
		logs:      make([]logEntry, 0),
		maxLogs:   100,
		spinner:   s,
		startTime: time.Now(),
	}

	// Initialize components
	app.printers = NewPrintersModel(manager)
	app.jobs = NewJobsModel(queue)
	app.printTab = NewPrintModel(manager, queue)
	app.command = NewCommandModel(executor)

	return app
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		a.tickCmd(),
		tea.EnterAltScreen,
	)
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// refreshPrintersCmd refreshes printers asynchronously
func (a *App) refreshPrintersCmd() tea.Cmd {
	return func() tea.Msg {
		printers, _ := a.manager.DetectPrinters()
		return printerUpdateMsg{printers: printers}
	}
}

// Update handles messages
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle command bar first if visible - it has priority
		if a.command.IsVisible() {
			newCmd, cmd := a.command.Update(msg)
			a.command = newCmd
			// Always return early when command bar is visible to prevent other key processing
			return a, cmd
		}

		// Global keys (only processed when command bar is not visible)
		switch msg.String() {
		case ":":
			// Show command bar (vim-style)
			if !a.printTab.inputFocused {
				a.command.Show()
				// Set size based on available width (account for padding)
				contentWidth := a.width - 24 - 3 // sidebar + padding
				a.command.SetSize(contentWidth)
			}
		case "ctrl+c", "q":
			if a.command.IsVisible() {
				a.command.Hide()
			} else if a.activeTab != TabPrint || !a.printTab.inputFocused {
				a.quitting = true
				return a, tea.Quit
			}
		case "1":
			if !a.printTab.inputFocused && !a.command.IsVisible() {
				a.activeTab = TabPrinters
			}
		case "2":
			if !a.printTab.inputFocused && !a.command.IsVisible() {
				a.activeTab = TabJobs
			}
		case "3":
			if !a.printTab.inputFocused && !a.command.IsVisible() {
				a.activeTab = TabPrint
			}
		case "tab":
			if !a.printTab.inputFocused && !a.command.IsVisible() {
				a.activeTab = (a.activeTab + 1) % 3
			}
		case "shift+tab":
			if !a.printTab.inputFocused && !a.command.IsVisible() {
				a.activeTab = (a.activeTab + 2) % 3
			}
		}

		// Delegate to active tab (only if command bar is not visible)
		if !a.command.IsVisible() {
			switch a.activeTab {
			case TabPrinters:
				newPrinters, cmd := a.printers.Update(msg)
				a.printers = newPrinters
				cmds = append(cmds, cmd)
			case TabJobs:
				newJobs, cmd := a.jobs.Update(msg)
				a.jobs = newJobs
				cmds = append(cmds, cmd)
			case TabPrint:
				newPrint, cmd := a.printTab.Update(msg)
				a.printTab = newPrint
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

		// Update component sizes (account for sidebar and console)
		sidebarWidth := 24
		consoleHeight := 8
		contentWidth := a.width - sidebarWidth - 3
		contentHeight := a.height - consoleHeight - 4

		a.printers.SetSize(contentWidth, contentHeight)
		a.jobs.SetSize(contentWidth, contentHeight)
		a.printTab.SetSize(contentWidth, contentHeight)
		a.command.SetSize(contentWidth)

	case tickMsg:
		// Refresh data asynchronously to avoid blocking UI
		a.jobs.Refresh() // Jobs refresh is fast, do it synchronously
		cmds = append(cmds, a.refreshPrintersCmd(), a.tickCmd())

	case printerUpdateMsg:
		if msg.printers != nil {
			a.printers.SetPrinters(msg.printers)
		} else {
			a.printers.Refresh()
		}

	case logMsg:
		a.addLog(msg.message, msg.level)

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tea.MouseMsg:
		// Handle mouse events
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Check if clicking on sidebar navigation
			sidebarWidth := 24
			if msg.X < sidebarWidth {
				// Clicked in sidebar - check which tab
				// Sidebar starts at line 0, tabs are at lines 5-7 (after logo and header)
				clickY := msg.Y
				if clickY >= 5 && clickY <= 7 {
					tabIndex := clickY - 5
					if tabIndex >= 0 && tabIndex < 3 {
						a.activeTab = Tab(tabIndex)
					}
				}
			}
		}

		// Delegate mouse events to active tab
		switch a.activeTab {
		case TabPrinters:
			newPrinters, cmd := a.printers.Update(msg)
			a.printers = newPrinters
			cmds = append(cmds, cmd)
		case TabJobs:
			newJobs, cmd := a.jobs.Update(msg)
			a.jobs = newJobs
			cmds = append(cmds, cmd)
		case TabPrint:
			newPrint, cmd := a.printTab.Update(msg)
			a.printTab = newPrint
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

// View renders the UI
func (a *App) View() string {
	if a.quitting {
		return "\n  👋 Goodbye!\n\n"
	}

	if !a.ready {
		return "\n  Loading...\n"
	}

	// If command bar is visible, show it as an overlay
	if a.command.IsVisible() {
		return a.renderCommandOverlay()
	}

	sidebarWidth := 24
	consoleHeight := 8
	helpHeight := 1

	// Calculate available height for main content area
	availableHeight := a.height - consoleHeight - helpHeight

	// Sidebar
	sidebar := a.renderSidebar(sidebarWidth, availableHeight)

	// Content
	contentWidth := a.width - sidebarWidth - 1
	contentHeight := availableHeight
	content := a.renderContent(contentWidth, contentHeight)

	// Top section: sidebar + content
	top := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Console at bottom
	console := a.renderConsole(a.width, consoleHeight)

	// Help bar
	help := a.renderHelp()

	// Join all sections
	fullView := lipgloss.JoinVertical(lipgloss.Left, top, console, help)

	// Ensure the view exactly fills the screen height to clear any leftover content
	// Pad with spaces if needed to fill exactly a.height lines
	lines := strings.Split(fullView, "\n")
	if len(lines) < a.height {
		// Pad with empty lines to fill screen
		for len(lines) < a.height {
			lines = append(lines, strings.Repeat(" ", a.width))
		}
	} else if len(lines) > a.height {
		// Truncate if somehow too tall
		lines = lines[:a.height]
	}

	return strings.Join(lines, "\n")
}

func (a *App) renderCommandOverlay() string {
	// Create a centered overlay for the command bar
	// Use most of the screen but leave some margin
	overlayWidth := a.width - 8
	if overlayWidth < 60 {
		overlayWidth = 60
	}
	overlayHeight := a.height - 6
	if overlayHeight < 20 {
		overlayHeight = 20
	}
	if overlayHeight > a.height-4 {
		overlayHeight = a.height - 4
	}

	// Update command size to match overlay
	a.command.SetSize(overlayWidth - 4)
	a.command.SetHeight(overlayHeight - 2) // Account for border padding

	commandView := a.command.View()

	// Wrap the command view in a bordered box with fixed height
	overlay := lipgloss.NewStyle().
		Width(overlayWidth).
		Height(overlayHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Secondary).
		Background(BgCard).
		Padding(1, 2).
		Render(commandView)

	// Center on screen
	centered := lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		overlay,
	)

	return centered
}

func (a *App) renderSidebar(width, height int) string {
	var lines []string

	// Logo
	lines = append(lines, LogoStyle.Render("Receipt Engine"))
	lines = append(lines, TextMuted.Render(fmt.Sprintf("Port %s", a.port)))
	lines = append(lines, "")

	// Navigation header
	lines = append(lines, TextMuted.Render(" NAVIGATION"))
	lines = append(lines, "")

	// Nav items
	for i, t := range []Tab{TabPrinters, TabJobs, TabPrint} {
		isActive := t == a.activeTab
		key := fmt.Sprintf("%d", i+1)
		label := t.String()

		itemText := fmt.Sprintf(" %s %s", key, label)
		// Pad to fill width - use same logic as status lines
		displayWidth := lipgloss.Width(itemText)
		padding := width - displayWidth - 2 // -2 for border padding
		if padding > 0 {
			itemText += strings.Repeat(" ", padding)
		}

		if isActive {
			lines = append(lines, SidebarActiveStyle.Render(itemText))
		} else {
			lines = append(lines, SidebarItemStyle.Render(itemText))
		}
	}

	lines = append(lines, "")

	// Stats section
	lines = append(lines, TextMuted.Render(" STATUS"))
	lines = append(lines, "")

	printers, _ := a.manager.DetectPrinters()
	jobs := a.queue.GetAllJobs()

	// Count job statuses
	queued, printing := 0, 0
	for _, j := range jobs {
		switch j.Status {
		case "queued":
			queued++
		case "printing":
			printing++
		}
	}

	// Helper function to pad any line to fill sidebar width
	// All lines should have the same format: " [text]" with padding to fill width
	padSidebarLine := func(text string) string {
		// Format with leading space to match nav items
		formatted := " " + text
		// Calculate padding - account for border (2 chars on right side)
		// lipgloss.Width() strips ANSI codes, so this works for both styled and unstyled text
		displayWidth := lipgloss.Width(formatted)
		padding := width - displayWidth - 2 // -2 for border padding
		if padding > 0 {
			formatted += strings.Repeat(" ", padding)
		}
		return formatted
	}

	printerText := fmt.Sprintf("%d printers", len(printers))
	lines = append(lines, padSidebarLine(printerText))
	if queued > 0 {
		queuedText := fmt.Sprintf("%d queued", queued)
		// Pad the plain text first, then apply style
		paddedQueued := padSidebarLine(queuedText)
		lines = append(lines, WarningStyle.Render(paddedQueued))
	}
	if printing > 0 {
		printingText := fmt.Sprintf("%d printing", printing)
		paddedPrinting := padSidebarLine(printingText)
		lines = append(lines, InfoStyle.Render(paddedPrinting))
	}

	// Uptime
	uptime := time.Since(a.startTime)
	hours := int(uptime.Hours())
	minutes := int(uptime.Minutes()) % 60
	uptimeText := fmt.Sprintf("%dh %dm", hours, minutes)
	lines = append(lines, padSidebarLine(uptimeText))

	// Join all lines
	content := strings.Join(lines, "\n")

	// Pad content to fill height
	lineCount := len(lines)
	for i := lineCount; i < height-2; i++ {
		content += "\n"
	}

	return SidebarStyle.
		Width(width).
		Height(height).
		Render(content)
}

func (a *App) renderContent(width, height int) string {
	// Update component sizes to respect height constraints
	a.printers.SetSize(width, height)
	a.jobs.SetSize(width, height)
	a.printTab.SetSize(width, height)

	var content string
	switch a.activeTab {
	case TabPrinters:
		content = a.printers.View()
	case TabJobs:
		content = a.jobs.View()
	case TabPrint:
		content = a.printTab.View()
	}

	// Truncate content to fit height if needed
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
		content = strings.Join(lines, "\n")
	}

	return ContentStyle.
		Width(width).
		Height(height).
		Render(content)
}

func (a *App) renderConsole(width, height int) string {
	var lines []string

	// Show last N logs - respect height exactly
	maxLines := height - 2 // Account for border
	if maxLines < 1 {
		maxLines = 1
	}

	start := 0
	if len(a.logs) > maxLines {
		start = len(a.logs) - maxLines
	}

	for i := start; i < len(a.logs); i++ {
		log := a.logs[i]
		timeStr := log.time.Format("15:04:05")

		var icon string
		var style lipgloss.Style
		switch log.level {
		case "error":
			icon = "✗"
			style = ErrorStyle
		case "warning":
			icon = "⚠"
			style = WarningStyle
		case "success":
			icon = "✓"
			style = SuccessStyle
		default:
			icon = "›"
			style = TextMuted
		}

		line := fmt.Sprintf(" %s %s %s", TextMuted.Render(timeStr), style.Render(icon), log.message)
		lines = append(lines, Truncate(line, width-4))
	}

	if len(lines) == 0 {
		lines = append(lines, TextMuted.Render(" No logs yet..."))
	}

	// Pad to exactly maxLines to ensure consistent height
	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	return ConsoleStyle.
		Width(width).
		Height(height).
		Render(content)
}

func (a *App) renderHelp() string {
	if a.command.IsVisible() {
		return HelpBarStyle.Width(a.width).Render(
			RenderHelp("enter", "execute") + "  " +
				RenderHelp("esc", "close"),
		)
	}

	var help string

	switch a.activeTab {
	case TabPrinters:
		help = a.printers.Help()
	case TabJobs:
		help = a.jobs.Help()
	case TabPrint:
		help = a.printTab.Help()
	}

	globalHelp := RenderHelp("1/2/3", "nav") + "  " +
		RenderHelp("click", "nav") + "  " +
		RenderHelp("tab", "next") + "  " +
		RenderHelp(":", "command") + "  " +
		RenderHelp("q", "quit")

	fullHelp := help + "  │  " + globalHelp

	return HelpBarStyle.Width(a.width).Render(fullHelp)
}

func (a *App) addLog(message, level string) {
	entry := logEntry{
		time:    time.Now(),
		message: message,
		level:   level,
	}

	a.logs = append(a.logs, entry)
	if len(a.logs) > a.maxLogs {
		a.logs = a.logs[1:]
	}
}

// AddLog adds a log message (thread-safe via program.Send)
func (a *App) AddLog(message, level string) {
	a.addLog(message, level)
}

// RefreshPrinters triggers a printer refresh
func (a *App) RefreshPrinters() {
	a.printers.Refresh()
}

// Run starts the TUI
func (a *App) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// LogWriter returns an io.Writer for logging
func (a *App) LogWriter() io.Writer {
	return &appLogWriter{app: a}
}

type appLogWriter struct {
	app *App
}

func (w *appLogWriter) Write(p []byte) (n int, err error) {
	message := strings.TrimSpace(string(p))
	if message != "" {
		w.app.addLog(message, "info")
	}
	return len(p), nil
}
