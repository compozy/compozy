package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors - Compozy brand colors
	Primary    = lipgloss.Color("#2E86AB") // Blue
	Secondary  = lipgloss.Color("#A23B72") // Purple
	Success    = lipgloss.Color("#46A758") // Green
	Warning    = lipgloss.Color("#F18F01") // Orange
	Error      = lipgloss.Color("#C73E1D") // Red
	Muted      = lipgloss.Color("#666666") // Gray
	Background = lipgloss.Color("#1A1A1A") // Dark background
	Foreground = lipgloss.Color("#FAFAFA") // Light text
	Border     = lipgloss.Color("#3A3A3A") // Border color
	Highlight  = lipgloss.Color("#FFD700") // Gold for highlights
	Surface    = lipgloss.Color("#2A2A2A") // Elevated surface color

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Foreground(Foreground).
			Background(Background)

	// Title styles
	TitleStyle = BaseStyle.
			Bold(true).
			Foreground(Primary).
			MarginBottom(1)

	// Subtitle styles
	SubtitleStyle = BaseStyle.
			Foreground(Secondary).
			Italic(true)

	// Error styles
	ErrorStyle = BaseStyle.
			Foreground(Error).
			Bold(true)

	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Error).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	// Success styles
	SuccessStyle = BaseStyle.
			Foreground(Success).
			Bold(true)

	// Warning styles
	WarningStyle = BaseStyle.
			Foreground(Warning).
			Bold(true)

	// Info styles
	InfoStyle = BaseStyle.
			Foreground(Primary)

	// Help styles
	HelpStyle = BaseStyle.
			Foreground(Muted).
			Italic(true)

	HelpKeyStyle = BaseStyle.
			Foreground(Primary).
			Bold(true)

	HelpDescStyle = BaseStyle.
			Foreground(Muted)

	// Input field styles
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(0, 1)

	FocusedInputStyle = InputStyle.
				BorderForeground(Primary)

	// Button styles
	ButtonStyle = lipgloss.NewStyle().
			Foreground(Foreground).
			Background(Primary).
			Padding(0, 3).
			MarginTop(1)

	ActiveButtonStyle = ButtonStyle.
				Background(Secondary).
				Bold(true)

	// Table styles
	TableHeaderStyle = BaseStyle.
				Bold(true).
				Foreground(Primary).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(Border)

	TableRowStyle = BaseStyle

	SelectedRowStyle = BaseStyle.
				Background(Surface).
				Foreground(Highlight)

	// Status bar styles
	StatusBarStyle = lipgloss.NewStyle().
			Background(Surface).
			Padding(0, 1)

	// Spinner styles
	SpinnerStyle = BaseStyle.
			Foreground(Primary)

	// Dialog styles
	DialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(1, 2).
			MarginTop(2).
			MarginBottom(2)

	// Pagination styles
	PaginationStyle = BaseStyle.
			Foreground(Muted)

	ActivePageStyle = BaseStyle.
			Foreground(Primary).
			Bold(true)

	// Code block styles
	CodeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2A2A2A")).
			Foreground(Success).
			Padding(1)

	// Badge styles
	BadgeStyle = lipgloss.NewStyle().
			Background(Primary).
			Foreground(Foreground).
			Padding(0, 1).
			Bold(true)

	// Breadcrumb styles
	BreadcrumbStyle = BaseStyle.
			Foreground(Muted)

	BreadcrumbActiveStyle = BaseStyle.
				Foreground(Primary).
				Bold(true)
)

// RenderTitle renders a title with consistent styling
func RenderTitle(text string) string {
	return TitleStyle.Render(text)
}

// RenderError renders an error message with consistent styling
func RenderError(text string) string {
	return ErrorStyle.Render("Error: ") + text
}

// RenderSuccess renders a success message with consistent styling
func RenderSuccess(text string) string {
	return SuccessStyle.Render("✓ ") + text
}

// RenderHelp renders help text with key bindings
func RenderHelp(bindings [][2]string) string {
	var help string
	for i, binding := range bindings {
		if i > 0 {
			help += " • "
		}
		help += HelpKeyStyle.Render(binding[0]) + " " + HelpDescStyle.Render(binding[1])
	}
	return help
}

// RenderStatusBar renders a status bar with the given content
func RenderStatusBar(width int, left, center, right string) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	centerWidth := width - leftWidth - rightWidth - 4 // 4 for padding

	if centerWidth < 0 {
		centerWidth = 0
		// When width is too small, prioritize left and right content
		// and skip center content if necessary
		if width < leftWidth+rightWidth {
			return StatusBarStyle.Width(width).Render(left + right)
		}
	}

	status := lipgloss.JoinHorizontal(
		lipgloss.Left,
		left,
		lipgloss.PlaceHorizontal(centerWidth, lipgloss.Center, center),
		right,
	)

	return StatusBarStyle.Width(width).Render(status)
}
