package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderBrandTabsRow composes the single top row shared by the standalone and
// tabbed cockpits: the COMPOZY brand on the left, the workflow tabs/chips next to
// it, and an optional right-aligned hint. Callers pre-render the chips (already
// styled and truncated) and pre-size to width.
func renderBrandTabsRow(width int, chips []string, rightHint string) string {
	left := renderGap(1) + styleTitle.Render("COMPOZY")
	if len(chips) > 0 {
		left += renderGap(2) + strings.Join(chips, renderGap(1))
	}
	if strings.TrimSpace(rightHint) == "" {
		return renderOwnedLineKnownOwned(width, left)
	}
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(rightHint)-1, 1)
	return renderOwnedLineKnownOwned(width, left+renderGap(gap)+rightHint)
}

// tabChipStyle styles one workflow tab/chip. The active chip inverts onto the
// accent fill so the focused workflow reads at a glance; inactive chips stay
// foreground-only so the terminal background shows through.
func tabChipStyle(statusColor color.Color, active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Padding(0, 1).Foreground(statusColor)
	if active {
		style = style.Bold(true).Background(colorAccent).Foreground(colorBgBase)
	}
	return style
}
