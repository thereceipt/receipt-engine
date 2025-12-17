package screens

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/thereceipt/receipt-engine/internal/parser"
	"github.com/thereceipt/receipt-engine/internal/printer"
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

// PrintBuilder is a screen for building and sending print jobs
type PrintBuilder struct {
	app            *tview.Application
	manager        *printer.Manager
	queue          *printer.PrintQueue
	form           *tview.Form
	printerList    *tview.DropDown
	fileInput      *tview.InputField
	preview        *tview.TextView
	layout         *tview.Flex
	receipt        *receiptformat.Receipt
	variableInputs map[string]*tview.InputField
}

// NewPrintBuilder creates a new print builder screen
func NewPrintBuilder(app *tview.Application, manager *printer.Manager, queue *printer.PrintQueue) *PrintBuilder {
	p := &PrintBuilder{
		app:            app,
		manager:        manager,
		queue:          queue,
		variableInputs: make(map[string]*tview.InputField),
	}

	p.setupUI()
	return p
}

func (p *PrintBuilder) setupUI() {
	// Printer selection
	printers, _ := p.manager.DetectPrinters()
	printerOptions := make([]string, len(printers))
	for i, pr := range printers {
		name := pr.Name
		if name == "" {
			name = pr.Description
		}
		if name == "" {
			name = pr.ID
		}
		printerOptions[i] = name
	}

	p.printerList = tview.NewDropDown()
	p.printerList.SetLabel("Printer: ")
	p.printerList.SetOptions(printerOptions, nil)
	if len(printerOptions) > 0 {
		p.printerList.SetCurrentOption(0)
	}

	// File input
	p.fileInput = tview.NewInputField()
	p.fileInput.SetLabel("Receipt File: ")
	p.fileInput.SetPlaceholder("/path/to/receipt.receipt")

	// Preview area
	p.preview = tview.NewTextView()
	p.preview.SetBorder(true)
	p.preview.SetTitle("Preview")
	p.preview.SetDynamicColors(true)

	// Form
	p.form = tview.NewForm()
	p.form.SetBorder(true)
	p.form.SetTitle("Print Receipt")
	p.form.AddFormItem(p.printerList)
	p.form.AddFormItem(p.fileInput)
	p.form.AddButton("Load Receipt", func() {
		p.loadReceipt()
	})
	p.form.AddButton("Print", func() {
		p.printReceipt()
	})
	p.form.AddButton("Cancel", func() {
		// Return to main view - handled by parent
	})

	// Layout: Form | Preview
	p.layout = tview.NewFlex().
		AddItem(p.form, 0, 1, true).
		AddItem(p.preview, 0, 1, false)

	// Key bindings
	p.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			return event // Let parent handle
		}
		return event
	})
}

func (p *PrintBuilder) loadReceipt() {
	filePath := strings.TrimSpace(p.fileInput.GetText())
	if filePath == "" {
		p.preview.SetText("[red]Please enter a receipt file path[white]")
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		p.preview.SetText(fmt.Sprintf("[red]Error reading file: %v[white]", err))
		return
	}

	receipt, err := receiptformat.Parse(data)
	if err != nil {
		p.preview.SetText(fmt.Sprintf("[red]Error parsing receipt: %v[white]", err))
		return
	}

	p.receipt = receipt

	// Build preview
	var preview strings.Builder
	preview.WriteString(fmt.Sprintf("[green]✓ Receipt loaded[white]\n\n"))
	preview.WriteString(fmt.Sprintf("[yellow]Version:[white] %s\n", receipt.Version))
	if receipt.Name != "" {
		preview.WriteString(fmt.Sprintf("[yellow]Name:[white] %s\n", receipt.Name))
	}
	preview.WriteString(fmt.Sprintf("[yellow]Commands:[white] %d\n", len(receipt.Commands)))

	if len(receipt.Variables) > 0 {
		preview.WriteString(fmt.Sprintf("\n[yellow]Variables:[white]\n"))
		for _, v := range receipt.Variables {
			preview.WriteString(fmt.Sprintf("  - %s (%s)\n", v.Let, v.ValueType))
		}
	}

	// Add variable inputs to form
	p.addVariableInputs(receipt)

	p.preview.SetText(preview.String())
}

func (p *PrintBuilder) addVariableInputs(receipt *receiptformat.Receipt) {
	// Remove old variable inputs
	for i := p.form.GetFormItemCount() - 1; i >= 0; i-- {
		item := p.form.GetFormItem(i)
		if _, ok := item.(*tview.InputField); ok {
			label := item.GetLabel()
			if strings.HasPrefix(label, "Variable: ") {
				p.form.RemoveFormItem(i)
			}
		}
	}

	// Clear variable inputs map
	p.variableInputs = make(map[string]*tview.InputField)

	// Add new variable inputs
	for _, v := range receipt.Variables {
		input := tview.NewInputField()
		label := fmt.Sprintf("Variable: %s", v.Let)
		input.SetLabel(label)
		if v.DefaultValue != nil {
			input.SetText(fmt.Sprintf("%v", v.DefaultValue))
		}
		p.variableInputs[v.Let] = input
		p.form.AddFormItem(input)
	}
}

func (p *PrintBuilder) printReceipt() {
	if p.receipt == nil {
		p.preview.SetText("[red]Please load a receipt first[white]")
		return
	}

	// Get selected printer
	printers, _ := p.manager.DetectPrinters()
	printerIndex, _ := p.printerList.GetCurrentOption()
	if printerIndex < 0 {
		p.preview.SetText("[red]Please select a printer[white]")
		return
	}
	if printerIndex >= len(printers) {
		p.preview.SetText("[red]Invalid printer selection[white]")
		return
	}

	selectedPrinter := printers[printerIndex]

	// Collect variable data
	variableData := make(map[string]interface{})
	for name, input := range p.variableInputs {
		value := strings.TrimSpace(input.GetText())
		if value != "" {
			variableData[name] = value
		}
	}

	// Create parser
	paperWidth := p.receipt.PaperWidth
	if paperWidth == "" {
		paperWidth = "80mm"
	}

	pars, err := parser.New(p.receipt, paperWidth)
	if err != nil {
		p.preview.SetText(fmt.Sprintf("[red]Error creating parser: %v[white]", err))
		return
	}

	// Set variable data
	if len(variableData) > 0 {
		pars.SetVariableData(variableData)
	}

	// Execute and get image
	img, err := pars.Execute()
	if err != nil {
		p.preview.SetText(fmt.Sprintf("[red]Error rendering receipt: %v[white]", err))
		return
	}

	// Enqueue print job
	jobID := p.queue.Enqueue(selectedPrinter.ID, img)
	p.preview.SetText(fmt.Sprintf("[green]✓ Print job enqueued[white]\n\n[yellow]Job ID:[white] %s\n[yellow]Printer:[white] %s", jobID, selectedPrinter.ID))
}

// GetRoot returns the root primitive for this screen
func (p *PrintBuilder) GetRoot() tview.Primitive {
	return p.layout
}
