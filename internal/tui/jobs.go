package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thereceipt/receipt-engine/internal/printer"
)

// JobsModel handles the jobs tab
type JobsModel struct {
	queue        *printer.PrintQueue
	jobs         []*printer.PrintJob
	cursor       int
	scrollOffset int // Track scroll position
	width        int
	height       int
	message      string
	msgType      string
}

// NewJobsModel creates a new jobs model
func NewJobsModel(queue *printer.PrintQueue) JobsModel {
	return JobsModel{
		queue: queue,
		jobs:  make([]*printer.PrintJob, 0),
	}
}

// SetSize sets the component size
func (m *JobsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.adjustScroll() // Adjust scroll when size changes
}

// Refresh refreshes the job list
func (m *JobsModel) Refresh() {
	m.jobs = m.queue.GetAllJobs()
	if m.cursor >= len(m.jobs) && len(m.jobs) > 0 {
		m.cursor = len(m.jobs) - 1
	}
	m.adjustScroll()
}

// Update handles messages
func (m JobsModel) Update(msg tea.Msg) (JobsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Calculate which job was clicked
			// Account for title and header spacing (approximately 3 lines)
			clickY := msg.Y - 3
			if clickY >= 0 {
				// Adjust for scroll offset
				actualIdx := m.scrollOffset + clickY
				if actualIdx >= 0 && actualIdx < len(m.jobs) {
					m.cursor = actualIdx
					m.adjustScroll()
				}
			}
		} else if msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
			// Handle mouse wheel scrolling
			if msg.Button == tea.MouseButtonWheelUp && m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			} else if msg.Button == tea.MouseButtonWheelDown && m.cursor < len(m.jobs)-1 {
				m.cursor++
				m.adjustScroll()
			}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
		case "down", "j":
			if m.cursor < len(m.jobs)-1 {
				m.cursor++
				m.adjustScroll()
			}
		case "r":
			m.Refresh()
			m.message = "Refreshed"
			m.msgType = "success"
		case "c":
			m.queue.ClearCompleted()
			m.Refresh()
			m.message = "Cleared completed"
			m.msgType = "success"
		}
	}

	return m, nil
}

// View renders the jobs tab
func (m JobsModel) View() string {
	var b strings.Builder

	// Calculate available lines (account for title, spacing, message)
	availableLines := m.height - 4 // Reserve space for title, spacing, and message
	if m.message != "" {
		availableLines -= 2 // Reserve space for message
	}

	// Title
	title := CardTitleStyle.Render("Print Queue")
	b.WriteString(title)
	b.WriteString("\n\n")

	if len(m.jobs) == 0 {
		b.WriteString(TextMuted.Render("No jobs in queue.\n"))
		b.WriteString(TextMuted.Render("Go to Print tab to send a job.\n"))
	} else {
		// Stats bar
		queued, printing, completed, failed := 0, 0, 0, 0
		for _, j := range m.jobs {
			switch j.Status {
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

		statsLine := ""
		if queued > 0 {
			statsLine += WarningStyle.Render(fmt.Sprintf("%d queued", queued)) + "  "
		}
		if printing > 0 {
			statsLine += InfoStyle.Render(fmt.Sprintf("%d printing", printing)) + "  "
		}
		if completed > 0 {
			statsLine += SuccessStyle.Render(fmt.Sprintf("%d completed", completed)) + "  "
		}
		if failed > 0 {
			statsLine += ErrorStyle.Render(fmt.Sprintf("%d failed", failed))
		}
		b.WriteString(statsLine)
		b.WriteString("\n\n")

		// Jobs list - limit to available lines
		maxJobs := availableLines - 8 // Reserve space for stats, details section
		if maxJobs < 0 {
			maxJobs = 0
		}

		// Use scroll offset for proper scrolling
		startIdx := m.scrollOffset
		endIdx := startIdx + maxJobs
		if endIdx > len(m.jobs) {
			endIdx = len(m.jobs)
		}

		for i := startIdx; i < endIdx; i++ {
			job := m.jobs[i]
			cursor := "  "
			style := ListItemStyle
			if i == m.cursor {
				cursor = "▸ "
				style = SelectedItemStyle
			}

			// Status color
			var statusStyle lipgloss.Style
			switch job.Status {
			case "queued":
				statusStyle = lipgloss.NewStyle().Foreground(Warning)
			case "printing":
				statusStyle = lipgloss.NewStyle().Foreground(Secondary)
			case "completed":
				statusStyle = lipgloss.NewStyle().Foreground(Success)
			case "failed":
				statusStyle = lipgloss.NewStyle().Foreground(Error)
			default:
				statusStyle = TextMuted.Copy()
			}

			// Format job ID
			jobID := job.ID
			if len(jobID) > 16 {
				jobID = jobID[:16]
			}

			// Age
			age := time.Since(job.CreatedAt).Truncate(time.Second).String()

			status := statusStyle.Render(job.Status)
			line := fmt.Sprintf("%s%s  %s  %s", cursor, Truncate(jobID, 18), status, TextMuted.Render(age))

			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}

		// Show scroll indicator if needed
		if len(m.jobs) > maxJobs {
			if m.scrollOffset > 0 {
				b.WriteString(TextMuted.Render("  ... (↑ to scroll) ...\n"))
			}
			if endIdx < len(m.jobs) {
				b.WriteString(TextMuted.Render("  ... (↓ to scroll) ...\n"))
			}
		}

		// Selected job details
		if m.cursor < len(m.jobs) {
			job := m.jobs[m.cursor]
			b.WriteString("\n")
			b.WriteString(SectionHeaderStyle.Render("DETAILS"))
			b.WriteString("\n")

			b.WriteString(TextMuted.Render("ID: ") + TextNormal.Render(job.ID))
			b.WriteString("\n")
			b.WriteString(TextMuted.Render("Printer: ") + TextNormal.Render(job.PrinterID))
			b.WriteString("\n")
			b.WriteString(TextMuted.Render("Created: ") + TextNormal.Render(job.CreatedAt.Format("15:04:05")))

			if job.Retries > 0 {
				b.WriteString("\n")
				b.WriteString(TextMuted.Render("Retries: ") + WarningStyle.Render(fmt.Sprintf("%d", job.Retries)))
			}

			if job.Error != nil {
				b.WriteString("\n")
				b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", job.Error)))
			}
		}
	}

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

// Help returns help text for this tab
func (m JobsModel) Help() string {
	return RenderHelp("↑/↓", "select") + "  " +
		RenderHelp("click", "select") + "  " +
		RenderHelp("c", "clear done") + "  " +
		RenderHelp("r", "refresh")
}

// adjustScroll adjusts the scroll offset to keep cursor visible
func (m *JobsModel) adjustScroll() {
	if len(m.jobs) == 0 {
		m.scrollOffset = 0
		return
	}

	maxVisible := m.height - 12 // Account for title, stats, details section, message
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
	maxOffset := len(m.jobs) - maxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}
