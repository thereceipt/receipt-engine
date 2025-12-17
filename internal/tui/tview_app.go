package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/thereceipt/receipt-engine/internal/printer"
	"github.com/thereceipt/receipt-engine/internal/tui/screens"
)

// TViewApp is the main TUI application using tview
type TViewApp struct {
	App     *tview.Application
	manager *printer.Manager
	pool    *printer.ConnectionPool
	queue   *printer.PrintQueue
	port    string

	// Main layout
	flex *tview.Flex

	// Panels
	printersList *tview.List
	queueTable   *tview.Table
	statusBox    *tview.TextView
	logsArea     *tview.TextView
	commandInput *tview.InputField

	// State
	logs      []string
	maxLogs   int
	startTime time.Time

	// Screens
	currentScreen  string // "main", "registry", "devices", "jobs", "print"
	registryScreen *screens.RegistryEditor
	devicesScreen  *screens.DevicesView
	jobsScreen     *screens.JobsView
	printScreen    *screens.PrintBuilder
}

// NewTViewApp creates a new tview-based TUI
func NewTViewApp(manager *printer.Manager, pool *printer.ConnectionPool, queue *printer.PrintQueue, port string) *TViewApp {
	app := tview.NewApplication()

	t := &TViewApp{
		App:           app,
		manager:       manager,
		pool:          pool,
		queue:         queue,
		port:          port,
		logs:          make([]string, 0),
		maxLogs:       100,
		startTime:     time.Now(),
		currentScreen: "main",
	}

	t.setupUI()
	t.setupScreens()
	return t
}

func (t *TViewApp) setupScreens() {
	t.registryScreen = screens.NewRegistryEditor(t.App, t.manager)
	t.devicesScreen = screens.NewDevicesView(t.App, t.manager)
	t.jobsScreen = screens.NewJobsView(t.App, t.queue)
	t.printScreen = screens.NewPrintBuilder(t.App, t.manager, t.queue)
}

func (t *TViewApp) setupUI() {
	// Create panels
	t.printersList = tview.NewList()
	t.printersList.SetBorder(true)
	t.printersList.SetTitle("Connected Printers")

	t.queueTable = tview.NewTable()
	t.queueTable.SetBorder(true)
	t.queueTable.SetTitle("Print Queue")

	t.statusBox = tview.NewTextView()
	t.statusBox.SetBorder(true)
	t.statusBox.SetTitle("Server Status")
	t.statusBox.SetDynamicColors(true)

	t.logsArea = tview.NewTextView()
	t.logsArea.SetBorder(true)
	t.logsArea.SetTitle("Server Logs")
	t.logsArea.SetDynamicColors(true)
	t.logsArea.SetScrollable(true)
	t.logsArea.SetChangedFunc(func() {
		t.App.Draw()
	})

	t.commandInput = tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0).
		SetPlaceholder("Type a command (e.g., 'help')").
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				t.executeCommand(t.commandInput.GetText())
				t.commandInput.SetText("")
			}
		})

	// Top row: Printers, Queue, Status
	topRow := tview.NewFlex().
		AddItem(t.printersList, 0, 1, false).
		AddItem(t.queueTable, 0, 1, false).
		AddItem(t.statusBox, 0, 1, false)

	// Bottom: Logs and command
	bottom := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(t.logsArea, 0, 3, false).
		AddItem(t.commandInput, 1, 0, true)

	// Main layout
	t.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topRow, 0, 1, false).
		AddItem(bottom, 0, 1, false)

	// Set up key bindings
	t.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle screen navigation
		if t.currentScreen != "main" {
			if event.Key() == tcell.KeyEsc {
				t.showMainScreen()
				return nil
			}
			return event
		}

		// If command input has focus, disable navigation shortcuts
		// but allow other keys to pass through
		if t.commandInput.HasFocus() {
			switch event.Key() {
			case tcell.KeyEsc:
				t.App.SetFocus(t.printersList)
				return nil
			case tcell.KeyRune:
				// Disable navigation shortcuts when typing commands
				switch event.Rune() {
				case 'r', 'd', 'j', 'p':
					// Allow these keys to be typed in command input
					return event
				}
			}
			// Let all other keys pass through to command input
			return event
		}

		// Main screen key bindings (when command input doesn't have focus)
		switch event.Key() {
		case tcell.KeyCtrlC, tcell.KeyEsc:
			t.App.Stop()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case ':':
				t.App.SetFocus(t.commandInput)
				return nil
			case 'q':
				t.App.Stop()
				return nil
			case 'r':
				t.showScreen("registry")
				return nil
			case 'd':
				t.showScreen("devices")
				return nil
			case 'j':
				t.showScreen("jobs")
				return nil
			case 'p':
				t.showScreen("print")
				return nil
			}
		}
		return event
	})

	t.App.SetRoot(t.flex, true)
}

// Run starts the TUI
func (t *TViewApp) Run() error {
	// Initial refresh
	t.refreshAll()

	// Start refresh ticker
	go t.refreshTicker()

	// Initial log
	t.AddLog("🖨️  Receipt Engine starting...", "info")

	return t.App.Run()
}

func (t *TViewApp) refreshTicker() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		t.App.QueueUpdateDraw(func() {
			t.refreshAll()
		})
	}
}

// RefreshPrinters is a public method to refresh the printers panel
func (t *TViewApp) RefreshPrinters() {
	t.refreshPrinters()
}

func (t *TViewApp) refreshAll() {
	t.refreshPrinters()
	t.refreshQueue()
	t.refreshStatus()
}

func (t *TViewApp) refreshPrinters() {
	t.printersList.Clear()

	printers, err := t.manager.DetectPrinters()
	if err != nil {
		t.printersList.AddItem("Error loading printers", err.Error(), 0, nil)
		return
	}

	if len(printers) == 0 {
		t.printersList.AddItem("No printers detected", "", 0, nil)
		return
	}

	for _, p := range printers {
		name := p.Name
		if name == "" {
			name = p.Description
		}
		if name == "" {
			name = p.ID
		}

		status := "🟢"
		details := fmt.Sprintf("%s • %s", strings.ToUpper(p.Type), p.Device)

		displayText := fmt.Sprintf("%s %s", status, name)
		t.printersList.AddItem(displayText, details, 0, nil)
	}
}

func (t *TViewApp) refreshQueue() {
	t.queueTable.Clear()

	// Header
	t.queueTable.SetCell(0, 0, tview.NewTableCell("Status").SetAlign(tview.AlignCenter).SetSelectable(false))
	t.queueTable.SetCell(0, 1, tview.NewTableCell("Printer").SetAlign(tview.AlignCenter).SetSelectable(false))
	t.queueTable.SetCell(0, 2, tview.NewTableCell("Retries").SetAlign(tview.AlignCenter).SetSelectable(false))
	t.queueTable.SetCell(0, 3, tview.NewTableCell("Time").SetAlign(tview.AlignCenter).SetSelectable(false))

	jobs := t.queue.GetAllJobs()

	// Count stats
	queued := 0
	printing := 0
	completed := 0
	failed := 0

	for i, job := range jobs {
		row := i + 1
		statusIcon := getStatusIcon(job.Status)

		t.queueTable.SetCell(row, 0, tview.NewTableCell(statusIcon+" "+job.Status))
		t.queueTable.SetCell(row, 1, tview.NewTableCell(job.PrinterID))
		t.queueTable.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%d", job.Retries)))

		timeStr := time.Since(job.CreatedAt).Truncate(time.Second).String()
		t.queueTable.SetCell(row, 3, tview.NewTableCell(timeStr))

		switch job.Status {
		case "queued":
			queued++
		case "printing":
			printing++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}

	// Add summary row
	if len(jobs) > 0 {
		summaryRow := len(jobs) + 1
		summary := fmt.Sprintf("[%d] Queued [%d] Printing [%d] Completed [%d] Failed",
			queued, printing, completed, failed)
		cell := tview.NewTableCell(summary)
		cell.SetSelectable(false)
		t.queueTable.SetCell(summaryRow, 0, cell)
		t.queueTable.SetCell(summaryRow, 1, tview.NewTableCell(""))
		t.queueTable.SetCell(summaryRow, 2, tview.NewTableCell(""))
		t.queueTable.SetCell(summaryRow, 3, tview.NewTableCell(""))
	}
}

func (t *TViewApp) refreshStatus() {
	uptime := time.Since(t.startTime)
	hours := int(uptime.Hours())
	minutes := int(uptime.Minutes()) % 60

	status := fmt.Sprintf(`[green]🟢 Running[white]

Uptime: %dh %dm
API: :%s
Jobs: %d total`, hours, minutes, t.port, len(t.queue.GetAllJobs()))

	t.statusBox.SetText(status)
}

func (t *TViewApp) executeCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	command := strings.ToLower(parts[0])

	t.AddLog(fmt.Sprintf("> %s", cmd), "command")

	switch command {
	case "printers":
		t.AddLog("Refreshing printers...", "info")
		t.refreshPrinters()

	case "status", "s":
		t.AddLog("Refreshing status...", "info")
		t.refreshStatus()

	case "registry", "r":
		t.showScreen("registry")

	case "devices", "d":
		t.showScreen("devices")

	case "jobs", "j":
		t.showScreen("jobs")

	case "print", "p":
		t.showScreen("print")

	case "help", "h", "?":
		t.showHelp()

	case "clear":
		t.logs = make([]string, 0)
		t.logsArea.Clear()

	case "refresh":
		t.AddLog("Refreshing all panels...", "info")
		t.refreshAll()

	default:
		t.AddLog(fmt.Sprintf("Unknown command: %s. Type 'help' for available commands.", command), "error")
	}
}

func (t *TViewApp) showHelp() {
	help := []string{
		"Available commands:",
		"  printers, p          - List all printers",
		"  jobs, j              - List all print jobs",
		"  status, s            - Show server status",
		"  registry, r          - Open registry editor",
		"  devices, d           - View connected devices",
		"  print, p             - Print receipt builder",
		"  clear                - Clear logs",
		"  refresh              - Refresh all panels",
		"  help, h, ?           - Show this help",
		"  quit, q              - Exit application",
		"",
		"Keyboard shortcuts:",
		"  r - Registry editor",
		"  d - Devices view",
		"  j - Jobs view",
		"  p - Print builder",
		"  Esc - Back to main",
	}
	t.AddLog(strings.Join(help, "\n"), "info")
}

func (t *TViewApp) showScreen(screenName string) {
	t.currentScreen = screenName

	switch screenName {
	case "registry":
		t.App.SetRoot(t.registryScreen.GetRoot(), true)
		t.App.SetFocus(t.registryScreen.GetRoot())
	case "devices":
		t.App.SetRoot(t.devicesScreen.GetRoot(), true)
		t.App.SetFocus(t.devicesScreen.GetRoot())
	case "jobs":
		t.App.SetRoot(t.jobsScreen.GetRoot(), true)
		t.App.SetFocus(t.jobsScreen.GetRoot())
	case "print":
		t.App.SetRoot(t.printScreen.GetRoot(), true)
		t.App.SetFocus(t.printScreen.GetRoot())
	case "main":
		t.showMainScreen()
	}
}

func (t *TViewApp) showMainScreen() {
	t.currentScreen = "main"
	t.App.SetRoot(t.flex, true)
	t.App.SetFocus(t.printersList)
}

// AddLog adds a log entry
func (t *TViewApp) AddLog(message string, level string) {
	var color string
	var icon string

	switch level {
	case "error":
		color = "[red]"
		icon = "❌"
	case "warning":
		color = "[yellow]"
		icon = "⚠️"
	case "command":
		color = "[cyan]"
		icon = ">"
	default:
		color = "[white]"
		icon = "ℹ️"
	}

	timeStr := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("%s[%s] %s %s[white]\n", color, timeStr, icon, message)

	t.logs = append(t.logs, logEntry)
	if len(t.logs) > t.maxLogs {
		t.logs = t.logs[len(t.logs)-t.maxLogs:]
	}

	// Update logs area
	t.logsArea.Clear()
	for _, log := range t.logs {
		fmt.Fprint(t.logsArea, log)
	}

	// Auto-scroll to bottom
	t.logsArea.ScrollToEnd()
}

func getStatusIcon(status string) string {
	switch status {
	case "queued":
		return "⏳"
	case "printing":
		return "🟡"
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	default:
		return "⚪"
	}
}

// LogWriter creates an io.Writer that writes to the logs panel
func (t *TViewApp) LogWriter() io.Writer {
	return &tviewLogWriter{app: t}
}

type tviewLogWriter struct {
	app *TViewApp
}

func (w *tviewLogWriter) Write(p []byte) (n int, err error) {
	message := strings.TrimSpace(string(p))
	if message != "" {
		w.app.AddLog(message, "info")
	}
	return len(p), nil
}
