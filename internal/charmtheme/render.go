package charmtheme

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	keycapBracketStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	keycapKeyStyle     = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
)

// Keycap renders a key hint such as "[ENTER]" with a muted bracket and an
// accented, upper-cased key. It is foreground-only so it composes cleanly with
// inline (non alt-screen) surfaces that do not own a background.
func Keycap(key string) string {
	return keycapBracketStyle.Render("[") +
		keycapKeyStyle.Render(strings.ToUpper(key)) +
		keycapBracketStyle.Render("]")
}

// TechPanelStyle returns a bordered panel style using the shared TechBorder.
// totalWidth is the full rendered footprint: in this lipgloss build Width() is
// the total output width (border + padding inclusive), so the inner content
// area is totalWidth - GetHorizontalFrameSize(). Focused panels use the accent
// focus border; unfocused panels use the muted border so the active pane reads
// at a glance.
func TechPanelStyle(totalWidth int, focused bool) lipgloss.Style {
	border := ColorBorder
	if focused {
		border = ColorBorderFocus
	}
	return lipgloss.NewStyle().
		BorderStyle(TechBorder).
		BorderForeground(border).
		Foreground(ColorFgBright).
		Padding(0, 1).
		Width(max(totalWidth, 1))
}
