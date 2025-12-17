package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/thereceipt/receipt-engine/internal/printer"
)

// JobsView shows detailed information about print jobs
type JobsView struct {
	app     *tview.Application
	queue   *printer.PrintQueue
	table   *tview.Table
	details *tview.TextView
	layout  *tview.Flex
}

// NewJobsView creates a new jobs view screen
func NewJobsView(app *tview.Application, queue *printer.PrintQueue) *JobsView {
	j := &JobsView{
		app:   app,
		queue: queue,
	}

	j.setupUI()
	return j
}

func (j *JobsView) setupUI() {
	// Jobs table
	j.table = tview.NewTable()
	j.table.SetBorder(true)
	j.table.SetTitle("Print Jobs")
	j.table.SetSelectable(true, false)
	j.table.SetSelectedFunc(func(row, column int) {
		j.selectJob(row)
	})

	// Details view
	j.details = tview.NewTextView()
	j.details.SetBorder(true)
	j.details.SetTitle("Job Details")
	j.details.SetDynamicColors(true)

	// Layout: Table | Details
	j.layout = tview.NewFlex().
		AddItem(j.table, 0, 2, true).
		AddItem(j.details, 0, 1, false)

	// Key bindings
	j.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			return event // Let parent handle
		case tcell.KeyRune:
			switch event.Rune() {
			case 'r':
				j.refresh()
				return nil
			case 'c':
				j.clearCompleted()
				return nil
			}
		}
		return event
	})

	j.refresh()
}

func (j *JobsView) refresh() {
	j.table.Clear()

	// Headers
	j.table.SetCell(0, 0, tview.NewTableCell("ID").SetAlign(tview.AlignCenter).SetSelectable(false))
	j.table.SetCell(0, 1, tview.NewTableCell("Printer").SetAlign(tview.AlignCenter).SetSelectable(false))
	j.table.SetCell(0, 2, tview.NewTableCell("Status").SetAlign(tview.AlignCenter).SetSelectable(false))
	j.table.SetCell(0, 3, tview.NewTableCell("Retries").SetAlign(tview.AlignCenter).SetSelectable(false))
	j.table.SetCell(0, 4, tview.NewTableCell("Time").SetAlign(tview.AlignCenter).SetSelectable(false))

	jobs := j.queue.GetAllJobs()

	for i, job := range jobs {
		row := i + 1
		statusIcon := getStatusIcon(job.Status)

		j.table.SetCell(row, 0, tview.NewTableCell(job.ID))
		j.table.SetCell(row, 1, tview.NewTableCell(job.PrinterID))
		j.table.SetCell(row, 2, tview.NewTableCell(statusIcon+" "+job.Status))
		j.table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%d", job.Retries)))

		timeStr := time.Since(job.CreatedAt).Truncate(time.Second).String()
		j.table.SetCell(row, 4, tview.NewTableCell(timeStr))
	}

	if len(jobs) == 0 {
		j.details.SetText("[yellow]No jobs in queue[white]")
	}
}

func (j *JobsView) selectJob(row int) {
	if row == 0 {
		return // Header row
	}

	jobs := j.queue.GetAllJobs()
	if row-1 >= len(jobs) {
		return
	}

	job := jobs[row-1]

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Job ID:[white] %s\n", job.ID))
	details.WriteString(fmt.Sprintf("[yellow]Printer ID:[white] %s\n", job.PrinterID))
	details.WriteString(fmt.Sprintf("[yellow]Status:[white] %s %s\n", getStatusIcon(job.Status), job.Status))
	details.WriteString(fmt.Sprintf("[yellow]Retries:[white] %d\n", job.Retries))
	details.WriteString(fmt.Sprintf("[yellow]Created:[white] %s\n", job.CreatedAt.Format("2006-01-02 15:04:05")))

	if job.Error != nil {
		details.WriteString(fmt.Sprintf("\n[red]Error:[white] %v\n", job.Error))
	}

	details.WriteString("\n[yellow]Press 'r' to refresh, 'c' to clear completed[white]")

	j.details.SetText(details.String())
}

func (j *JobsView) clearCompleted() {
	// Note: This would require adding a ClearCompleted method to PrintQueue
	j.refresh()
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

// GetRoot returns the root primitive for this screen
func (j *JobsView) GetRoot() tview.Primitive {
	return j.layout
}
