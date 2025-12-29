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
		// Handle command area first if visible - it has priority
		if a.command.IsVisible() {
			newCmd, cmd := a.command.Update(msg)
			a.command = newCmd
			// Always return early when command area is visible to prevent other key processing
			return a, cmd
		}

		// Global keys (only processed when command area is not visible)
		switch msg.String() {
		case ":":
			// Show command line (vim-style)
			if !a.printTab.inputFocused {
				a.command.Show()
				a.command.SetSize(maxInt(20, a.width))
				a.command.SetHeight(a.bottomAreaHeight())
			}
		case "ctrl+c", "q":
			if a.command.IsVisible() {
				a.command.Hide()
			} else if a.activeTab != TabPrint || !a.printTab.inputFocused {
				a.quitting = true
				return a, tea.Quit
			}
		case "1":
			if !a.printTab.inputFocused {
				a.activeTab = TabPrinters
			}
		case "2":
			if !a.printTab.inputFocused {
				a.activeTab = TabJobs
			}
		case "3":
			if !a.printTab.inputFocused {
				a.activeTab = TabPrint
			}
		case "tab":
			if !a.printTab.inputFocused {
				a.activeTab = (a.activeTab + 1) % 3
			}
		case "shift+tab":
			if !a.printTab.inputFocused {
				a.activeTab = (a.activeTab + 2) % 3
			}
		}

		// Delegate to active tab
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

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

		sidebarWidth := 24
		contentWidth := a.width - sidebarWidth - 1
		if contentWidth < 20 {
			contentWidth = 20
		}
		contentHeight := a.height - a.bottomAreaHeight()
		if contentHeight < 1 {
			contentHeight = 1
		}

		a.printers.SetSize(contentWidth, contentHeight)
		a.jobs.SetSize(contentWidth, contentHeight)
		a.printTab.SetSize(contentWidth, contentHeight)
		a.command.SetSize(a.width)
		a.command.SetHeight(a.bottomAreaHeight())

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
		// If the command area is visible, it owns the bottom area; otherwise ignore clicks there.
		if msg.Y >= a.height-a.bottomAreaHeight() {
			return a, nil
		}

		sidebarWidth := 24

		// Translate mouse coords into the content pane (account for sidebar + ContentStyle padding).
		translated := msg
		translated.X = msg.X - sidebarWidth - 2 // sidebar + ContentStyle padding left
		translated.Y = msg.Y - 1                // ContentStyle padding top
		if translated.X < 0 {
			translated.X = 0
		}
		if translated.Y < 0 {
			translated.Y = 0
		}

		// Delegate mouse events to active tab
		switch a.activeTab {
		case TabPrinters:
			newPrinters, cmd := a.printers.Update(translated)
			a.printers = newPrinters
			cmds = append(cmds, cmd)
		case TabJobs:
			newJobs, cmd := a.jobs.Update(translated)
			a.jobs = newJobs
			cmds = append(cmds, cmd)
		case TabPrint:
			newPrint, cmd := a.printTab.Update(translated)
			a.printTab = newPrint
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

// View renders the UI
func (a *App) View() string {
	if a.quitting {
		return "\n  Goodbye!\n\n"
	}

	if !a.ready {
		return "\n  Loading...\n"
	}

	sidebarWidth := 24
	contentHeight := a.height - a.bottomAreaHeight()
	if contentHeight < 1 {
		contentHeight = 1
	}
	contentWidth := a.width - sidebarWidth - 1
	if contentWidth < 20 {
		contentWidth = 20
	}
	sidebar := a.renderSidebar(sidebarWidth, contentHeight)
	content := a.renderContent(contentWidth, contentHeight)
	top := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	var bottom string
	if a.command.IsVisible() {
		// Bottom transforms into a terminal-like command area.
		a.command.SetSize(a.width)
		a.command.SetHeight(a.bottomAreaHeight())
		bottom = a.renderCommandArea()
	} else {
		bottom = a.renderStatusBar()
	}

	fullView := lipgloss.JoinVertical(lipgloss.Left, top, bottom)

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

	// Fill remaining space (no stats here anymore; those moved to statusline)
	content := strings.Join(lines, "\n")
	for lipgloss.Height(content) < height-2 {
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

func (a *App) renderStatusBar() string {
	// ASCII-only statusline (no powerline glyphs, no bullets).
	// Design intent: stable, glanceable operational state, not navigation.
	base := lipgloss.NewStyle().Background(BgCard).Foreground(colorTextNormal)

	seg := func(text string, fg, bg lipgloss.Color, bold bool) string {
		s := lipgloss.NewStyle().Foreground(fg).Background(bg).Padding(0, 1)
		if bold {
			s = s.Bold(true)
		}
		return s.Render(text)
	}
	pipe := base.Render(" | ")

	// Mode (only indicates whether ':' command input is active)
	modeText := "NAV"
	modeBg := BgHover
	if a.command.IsVisible() {
		modeText = "CMD"
		modeBg = Warning
	}
	mode := seg(modeText, colorTextBright, modeBg, true)

	// Static app/runtime context (useful when running multiple instances).
	port := seg("port "+a.port, colorTextBright, Primary, false)

	// Printers count (never do slow I/O in View).
	prCount := len(a.printers.printers)
	pr := seg("printers "+itoa(prCount), colorTextBright, Secondary, true)

	// Queue/printing counts (use cached jobs list when available).
	queued, printing := 0, 0
	for _, j := range a.jobs.jobs {
		switch j.Status {
		case "queued":
			queued++
		case "printing":
			printing++
		}
	}
	q := seg("queued "+itoa(queued), colorTextBright, BgHover, false)
	p := seg("printing "+itoa(printing), colorTextBright, BgHover, false)

	// Last message (colored by severity).
	msgText := "ready"
	msgBg := BgCard
	msgFg := colorTextNormal
	if len(a.logs) > 0 {
		last := a.logs[len(a.logs)-1]
		msgText = last.message
		switch last.level {
		case "error":
			msgBg = Error
			msgFg = colorTextBright
		case "warning":
			msgBg = Warning
			msgFg = colorTextBright
		case "success":
			msgBg = Success
			msgFg = colorTextBright
		default:
			msgBg = BgConsole
			msgFg = colorTextBright
		}
	}

	// Uptime lives on the far right, as requested.
	uptime := time.Since(a.startTime)
	h := int(uptime.Hours())
	m := int(uptime.Minutes()) % 60
	up := seg("up "+pad2(h)+":"+pad2(m), colorTextBright, Primary, true)

	// Compose left side first, then allocate space for message.
	leftFixed := mode + pipe + port + pipe + pr + pipe + q + pipe + p + pipe
	// Remaining width for message (keep at least 10 chars).
	remaining := a.width - lipgloss.Width(leftFixed) - lipgloss.Width(pipe) - lipgloss.Width(up)
	if remaining < 10 {
		remaining = 10
	}
	msgText = Truncate(msgText, remaining)
	msg := seg(msgText, msgFg, msgBg, false)

	left := leftFixed + msg
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(pipe) - lipgloss.Width(up)
	if gap < 1 {
		gap = 1
	}

	line := left + strings.Repeat(" ", gap) + pipe + up
	return base.Width(a.width).Render(line)
}

func (a *App) renderCommandArea() string {
	// A small "terminal" zone: results above, input on last line.
	// Background matches the app chrome.
	base := lipgloss.NewStyle().Background(BgCard).Foreground(colorTextNormal)
	view := a.command.View()

	// Ensure fixed height.
	lines := strings.Split(view, "\n")
	h := a.bottomAreaHeight()
	if len(lines) < h {
		for len(lines) < h {
			lines = append(lines, "")
		}
	} else if len(lines) > h {
		lines = lines[len(lines)-h:]
	}
	return base.Width(a.width).Height(h).Render(strings.Join(lines, "\n"))
}

func (a *App) bottomAreaHeight() int {
	if a.command.IsVisible() {
		// Neovim-like commandline: a few lines for output + input.
		h := a.height / 3
		if h < 5 {
			h = 5
		}
		if h > 10 {
			h = 10
		}
		return h
	}
	return 1
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func pad2(v int) string {
	if v < 10 {
		return "0" + itoa(v)
	}
	return itoa(v)
}

func itoa(v int) string {
	// Small, dependency-free int->string for statusline.
	// (Avoid importing strconv just for this file.)
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	var buf [32]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
