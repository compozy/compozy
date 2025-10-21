package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
)

// RenderASCIIHeader renders the Compozy logo as colored ASCII art with styling
func RenderASCIIHeader(width int) (string, error) {
	logo := figure.NewFigure("COMPOZY", "standard", true)
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")). // Compozy green
		Bold(true).
		Align(lipgloss.Left).
		Width(width)
	return headerStyle.Render(logo.String()), nil
}
