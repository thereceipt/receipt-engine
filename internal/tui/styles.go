package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors - A clean, modern color palette
var (
	// Primary colors
	Primary   = lipgloss.Color("#7C3AED") // Purple
	Secondary = lipgloss.Color("#06B6D4") // Cyan
	Success   = lipgloss.Color("#10B981") // Green
	Warning   = lipgloss.Color("#F59E0B") // Amber
	Error     = lipgloss.Color("#EF4444") // Red
	Muted     = lipgloss.Color("#6B7280") // Gray

	// Background colors
	BgDark    = lipgloss.Color("#0F172A") // Slate 900
	BgCard    = lipgloss.Color("#1E293B") // Slate 800
	BgHover   = lipgloss.Color("#334155") // Slate 700
	BgSidebar = lipgloss.Color("#18181B") // Zinc 900
	BgConsole = lipgloss.Color("#09090B") // Zinc 950

	// Text colors (as lipgloss.Color for use in styles)
	colorTextBright = lipgloss.Color("#F8FAFC") // Slate 50
	colorTextNormal = lipgloss.Color("#CBD5E1") // Slate 300
	colorTextMuted  = lipgloss.Color("#64748B") // Slate 500
)

// Text styles (can call .Render())
var (
	TextBright = lipgloss.NewStyle().Foreground(colorTextBright)
	TextNormal = lipgloss.NewStyle().Foreground(colorTextNormal)
	TextMuted  = lipgloss.NewStyle().Foreground(colorTextMuted)
)

// Styles
var (
	// Sidebar - solid background that fills the area
	SidebarStyle = lipgloss.NewStyle().
			Background(BgSidebar).
			Foreground(colorTextNormal).
			Padding(1, 0).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(BgHover)

	SidebarItemStyle = lipgloss.NewStyle().
				Foreground(colorTextMuted)

	SidebarActiveStyle = lipgloss.NewStyle().
				Foreground(colorTextBright).
				Background(Primary).
				Bold(true)

	// Content area
	ContentStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// Console
	ConsoleStyle = lipgloss.NewStyle().
			Background(BgConsole).
			Foreground(colorTextNormal).
			Padding(0, 1).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(BgHover)

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTextBright).
			Background(Primary).
			Padding(0, 2).
			MarginBottom(1)

	LogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTextBright)

	// Cards/Panels
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Muted).
			Padding(1, 2)

	CardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Secondary).
			MarginBottom(1)

	// List items
	ListItemStyle = lipgloss.NewStyle().
			Foreground(colorTextNormal).
			PaddingLeft(2)

	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(colorTextBright).
				Background(BgHover).
				Bold(true).
				PaddingLeft(2)

	// Status indicators
	StatusOnline = lipgloss.NewStyle().
			Foreground(Success).
			SetString("●")

	StatusOffline = lipgloss.NewStyle().
			Foreground(Error).
			SetString("●")

	StatusPending = lipgloss.NewStyle().
			Foreground(Warning).
			SetString("●")

	// Input
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Muted).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Primary).
				Padding(0, 1)

	InputLabelStyle = lipgloss.NewStyle().
			Foreground(colorTextMuted).
			MarginBottom(0)

	InputLabelFocusedStyle = lipgloss.NewStyle().
				Foreground(Secondary).
				Bold(true).
				MarginBottom(0)

	// Buttons
	ButtonStyle = lipgloss.NewStyle().
			Foreground(colorTextBright).
			Background(Primary).
			Padding(0, 3).
			MarginTop(1)

	ButtonInactiveStyle = lipgloss.NewStyle().
				Foreground(colorTextMuted).
				Background(BgCard).
				Padding(0, 3).
				MarginTop(1)

	// Help bar
	HelpStyle = lipgloss.NewStyle().
			Foreground(colorTextMuted)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Secondary).
			Bold(true)

	HelpBarStyle = lipgloss.NewStyle().
			Foreground(colorTextMuted).
			Background(BgCard).
			Padding(0, 2)

	// Messages
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	InfoStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	// Table
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Secondary).
				BorderBottom(true).
				BorderForeground(Muted)

	TableCellStyle = lipgloss.NewStyle().
			Foreground(colorTextNormal).
			Padding(0, 1)

	// Spinner
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(Primary)

	// Section headers in content
	SectionHeaderStyle = lipgloss.NewStyle().
				Foreground(colorTextMuted).
				Bold(true).
				MarginBottom(1)
)

// Helper functions
func RenderKey(key string) string {
	return HelpKeyStyle.Render(key)
}

func RenderHelp(key, desc string) string {
	return RenderKey(key) + HelpStyle.Render(" "+desc)
}

func StatusIcon(status string) string {
	switch status {
	case "online", "connected", "completed":
		return StatusOnline.String()
	case "offline", "disconnected", "failed":
		return StatusOffline.String()
	case "pending", "queued", "printing":
		return StatusPending.String()
	default:
		return StatusPending.String()
	}
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
