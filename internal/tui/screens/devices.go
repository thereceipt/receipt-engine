package screens

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/thereceipt/receipt-engine/internal/printer"
)

// DevicesView shows detailed information about connected devices
type DevicesView struct {
	app     *tview.Application
	manager *printer.Manager
	list    *tview.List
	details *tview.TextView
	layout  *tview.Flex
}

// NewDevicesView creates a new devices view screen
func NewDevicesView(app *tview.Application, manager *printer.Manager) *DevicesView {
	d := &DevicesView{
		app:     app,
		manager: manager,
	}

	d.setupUI()
	return d
}

func (d *DevicesView) setupUI() {
	// Device list
	d.list = tview.NewList()
	d.list.SetBorder(true)
	d.list.SetTitle("Connected Devices")
	d.list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		d.selectDevice(mainText)
	})

	// Details view
	d.details = tview.NewTextView()
	d.details.SetBorder(true)
	d.details.SetTitle("Device Details")
	d.details.SetDynamicColors(true)

	// Layout: List | Details
	d.layout = tview.NewFlex().
		AddItem(d.list, 0, 1, true).
		AddItem(d.details, 0, 2, false)

	// Key bindings
	d.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			return event // Let parent handle
		case tcell.KeyRune:
			switch event.Rune() {
			case 'r':
				d.refresh()
				return nil
			}
		}
		return event
	})

	d.refresh()
}

func (d *DevicesView) refresh() {
	d.list.Clear()

	printers, err := d.manager.DetectPrinters()
	if err != nil {
		d.list.AddItem("Error loading devices", err.Error(), 0, nil)
		return
	}

	if len(printers) == 0 {
		d.list.AddItem("No devices detected", "", 0, nil)
		d.details.SetText("[yellow]No devices connected[white]")
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
		d.list.AddItem(fmt.Sprintf("%s %s", status, name), details, 0, nil)
	}

	// Select first item
	if d.list.GetItemCount() > 0 {
		d.list.SetCurrentItem(0)
		mainText, _ := d.list.GetItemText(0)
		d.selectDevice(mainText)
	}
}

func (d *DevicesView) selectDevice(displayText string) {
	parts := strings.Fields(displayText)
	if len(parts) < 2 {
		return
	}

	printerName := strings.Join(parts[1:], " ")
	printers := d.manager.GetAllPrinters()

	var selectedPrinter *printer.Printer
	for _, p := range printers {
		name := p.Name
		if name == "" {
			name = p.Description
		}
		if name == "" {
			name = p.ID
		}
		if name == printerName {
			selectedPrinter = p
			break
		}
	}

	if selectedPrinter == nil {
		return
	}

	// Build detailed info
	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]ID:[white] %s\n", selectedPrinter.ID))
	details.WriteString(fmt.Sprintf("[yellow]Type:[white] %s\n", strings.ToUpper(selectedPrinter.Type)))
	details.WriteString(fmt.Sprintf("[yellow]Description:[white] %s\n", selectedPrinter.Description))

	if selectedPrinter.Device != "" {
		details.WriteString(fmt.Sprintf("[yellow]Device:[white] %s\n", selectedPrinter.Device))
	}
	if selectedPrinter.Host != "" {
		details.WriteString(fmt.Sprintf("[yellow]Host:[white] %s\n", selectedPrinter.Host))
	}
	if selectedPrinter.Port > 0 {
		details.WriteString(fmt.Sprintf("[yellow]Port:[white] %d\n", selectedPrinter.Port))
	}
	if selectedPrinter.VID > 0 {
		details.WriteString(fmt.Sprintf("[yellow]VID:[white] 0x%04X\n", selectedPrinter.VID))
	}
	if selectedPrinter.PID > 0 {
		details.WriteString(fmt.Sprintf("[yellow]PID:[white] 0x%04X\n", selectedPrinter.PID))
	}

	details.WriteString(fmt.Sprintf("[yellow]Name:[white] %s\n", selectedPrinter.Name))
	details.WriteString("\n[yellow]Press 'r' to refresh[white]")

	d.details.SetText(details.String())
}

// GetRoot returns the root primitive for this screen
func (d *DevicesView) GetRoot() tview.Primitive {
	return d.layout
}
