package screens

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/thereceipt/receipt-engine/internal/printer"
)

// RegistryEditor is a screen for editing printer registry
type RegistryEditor struct {
	app              *tview.Application
	manager          *printer.Manager
	form             *tview.Form
	list             *tview.List
	details          *tview.TextView
	layout           *tview.Flex
	currentPrinterID string
}

// NewRegistryEditor creates a new registry editor screen
func NewRegistryEditor(app *tview.Application, manager *printer.Manager) *RegistryEditor {
	r := &RegistryEditor{
		app:     app,
		manager: manager,
	}

	r.setupUI()
	return r
}

func (r *RegistryEditor) setupUI() {
	// Printer list
	r.list = tview.NewList()
	r.list.SetBorder(true)
	r.list.SetTitle("Printers")
	r.list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		r.selectPrinter(mainText)
	})

	// Details view
	r.details = tview.NewTextView()
	r.details.SetBorder(true)
	r.details.SetTitle("Printer Details")
	r.details.SetDynamicColors(true)

	// Form for editing
	r.form = tview.NewForm()
	r.form.SetBorder(true)
	r.form.SetTitle("Edit Printer Name")
	r.form.AddInputField("Name", "", 30, nil, nil)
	r.form.AddButton("Save", func() {
		r.savePrinterName()
	})
	r.form.AddButton("Cancel", func() {
		r.app.SetRoot(r.layout, true)
		r.app.SetFocus(r.list)
	})

	// Layout: List | Details + Form
	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(r.details, 0, 1, false).
		AddItem(r.form, 0, 1, true)

	r.layout = tview.NewFlex().
		AddItem(r.list, 0, 1, true).
		AddItem(rightPanel, 0, 2, false)

	// Key bindings
	r.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			return event // Let parent handle
		case tcell.KeyRune:
			switch event.Rune() {
			case 'r':
				r.refresh()
				return nil
			case 'e':
				if r.list.GetItemCount() > 0 {
					mainText, _ := r.list.GetItemText(r.list.GetCurrentItem())
					r.selectPrinter(mainText)
					r.app.SetFocus(r.form)
				}
				return nil
			}
		}
		return event
	})

	r.refresh()
}

func (r *RegistryEditor) refresh() {
	r.list.Clear()

	printers, err := r.manager.DetectPrinters()
	if err != nil {
		r.list.AddItem("Error loading printers", err.Error(), 0, nil)
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
		r.list.AddItem(fmt.Sprintf("%s %s", status, name), details, 0, nil)
	}
}

func (r *RegistryEditor) selectPrinter(displayText string) {
	// Extract printer ID from display text
	parts := strings.Fields(displayText)
	if len(parts) < 2 {
		return
	}

	printerName := strings.Join(parts[1:], " ")
	printers := r.manager.GetAllPrinters()

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

	// Show details
	details := fmt.Sprintf(`[yellow]ID:[white] %s
[yellow]Type:[white] %s
[yellow]Description:[white] %s
[yellow]Device:[white] %s
[yellow]Name:[white] %s

[yellow]Press 'e' to edit name`,
		selectedPrinter.ID,
		strings.ToUpper(selectedPrinter.Type),
		selectedPrinter.Description,
		selectedPrinter.Device,
		selectedPrinter.Name)

	r.details.SetText(details)

	// Update form
	r.form.GetFormItem(0).(*tview.InputField).SetText(selectedPrinter.Name)
	r.currentPrinterID = selectedPrinter.ID
}

func (r *RegistryEditor) savePrinterName() {
	if r.currentPrinterID == "" {
		r.details.SetText("[red]✗ No printer selected[white]")
		return
	}

	nameField := r.form.GetFormItem(0).(*tview.InputField)
	newName := strings.TrimSpace(nameField.GetText())

	// Verify printer exists before trying to rename
	printer := r.manager.GetPrinter(r.currentPrinterID)
	if printer == nil {
		r.details.SetText(fmt.Sprintf("[red]✗ Printer not found: %s[white]\n\n[yellow]Try refreshing the list[white]", r.currentPrinterID))
		return
	}

	success := r.manager.SetPrinterName(r.currentPrinterID, newName)
	if success {
		// Update the in-memory printer immediately
		printer.Name = newName

		// Refresh the UI to show updated name
		r.refresh()

		// Re-select the same printer to show updated details
		// Find the printer by ID in the refreshed list
		for i := 0; i < r.list.GetItemCount(); i++ {
			mainText, _ := r.list.GetItemText(i)
			// The display text format is "🟢 <name>", so check if it contains the new name or ID
			if strings.Contains(mainText, newName) || strings.Contains(mainText, r.currentPrinterID) {
				r.list.SetCurrentItem(i)
				r.selectPrinter(mainText)
				break
			}
		}

		r.app.SetFocus(r.list)
		// Show success message with verification
		updatedPrinter := r.manager.GetPrinter(r.currentPrinterID)
		if updatedPrinter != nil && updatedPrinter.Name == newName {
			r.details.SetText(fmt.Sprintf("[green]✓ Name updated successfully![white]\n\n[yellow]New name:[white] %s\n[yellow]Printer ID:[white] %s\n\n[yellow]Press 'r' to refresh[white]", newName, r.currentPrinterID))
		} else {
			r.details.SetText(fmt.Sprintf("[yellow]⚠ Name saved, but may need refresh[white]\n\n[yellow]New name:[white] %s\n[yellow]Press 'r' to refresh[white]", newName))
		}
	} else {
		r.details.SetText(fmt.Sprintf("[red]✗ Failed to update printer name[white]\n\n[yellow]Printer ID:[white] %s\n[yellow]This may happen if the printer is not in the registry yet.[white]\n[yellow]Try refreshing first.[white]", r.currentPrinterID))
	}
}

// GetRoot returns the root primitive for this screen
func (r *RegistryEditor) GetRoot() tview.Primitive {
	return r.layout
}
