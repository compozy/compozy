package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/charmtheme"
)

const (
	progressGradientStart = charmtheme.ProgressGradientStart
	progressGradientEnd   = charmtheme.ProgressGradientEnd
)

// Semantic color palette — all UI colors defined here, nowhere else.
var (
	colorBgBase    = charmtheme.ColorBgBase
	colorBgSurface = charmtheme.ColorBgSurface

	colorBrand      = charmtheme.ColorBrand
	colorAccent     = charmtheme.ColorAccent
	colorAccentAlt  = charmtheme.ColorAccentAlt
	colorAccentDeep = charmtheme.ColorAccentDeep

	colorSuccess = charmtheme.ColorSuccess
	colorError   = charmtheme.ColorError
	colorWarning = charmtheme.ColorWarning
	colorInfo    = charmtheme.ColorInfo

	colorFgBright = charmtheme.ColorFgBright
	colorMuted    = charmtheme.ColorMuted
	colorDim      = charmtheme.ColorDim

	colorBorder      = charmtheme.ColorBorder
	colorBorderFocus = charmtheme.ColorBorderFocus
)

var techBorder = charmtheme.TechBorder

// Pre-built styles reused across the UI.
var (
	styleSeparator     = lipgloss.NewStyle().Foreground(colorBorder)
	styleTitle         = lipgloss.NewStyle().Bold(true).Foreground(colorBrand)
	styleTitleMeta     = lipgloss.NewStyle().Foreground(colorMuted)
	styleBodyText      = lipgloss.NewStyle().Foreground(colorFgBright)
	styleMutedText     = lipgloss.NewStyle().Foreground(colorMuted)
	styleDimText       = lipgloss.NewStyle().Foreground(colorDim)
	stylePanelLabel    = lipgloss.NewStyle().Bold(true).Foreground(colorAccentDeep)
	styleTimelineTitle = lipgloss.NewStyle().Bold(true).Foreground(colorAccentDeep)
	styleTimelineBadge = lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
	styleKeycap        = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
)

func rootScreenStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(max(width, 1)).
		Height(max(height, 1)).
		Background(colorBgBase).
		Foreground(colorFgBright)
}

func techPanelStyle(renderWidth int, borderColor color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(renderWidth).
		BorderStyle(techBorder).
		BorderForeground(borderColor).
		BorderBackground(colorBgSurface).
		Background(colorBgSurface).
		Foreground(colorFgBright).
		Padding(0, 1)
}

func techSidebarStyle(width int, borderColor color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		BorderStyle(techBorder).
		BorderForeground(borderColor).
		BorderBackground(colorBgSurface).
		Background(colorBgSurface).
		Foreground(colorFgBright).
		Padding(0, 1)
}

func selectedSidebarRowStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().Width(max(width, 1))
}

func panelContentWidth(width int) int {
	return max(width-techPanelStyle(width, colorBorder).GetHorizontalFrameSize(), 1)
}

func sidebarContentWidth(width int) int {
	return max(width-techSidebarStyle(width, colorBorder).GetHorizontalFrameSize(), 1)
}

func sidebarContentHeight(height int) int {
	return max(height-techSidebarStyle(1, colorBorder).GetVerticalFrameSize(), 1)
}

func renderStyledOnBackground(style lipgloss.Style, bg color.Color, text string) string {
	return style.Background(bg).Render(text)
}

func renderGap(bg color.Color, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Background(bg).
		Render(strings.Repeat(" ", width))
}

func renderOwnedLine(width int, bg color.Color, content string) string {
	content = reapplyOwnedBackground(content, bg)
	return lipgloss.NewStyle().
		Width(max(width, 1)).
		Foreground(colorFgBright).
		Background(bg).
		Render(content)
}

func renderStyledOwnedLine(width int, style lipgloss.Style, bg color.Color, text string) string {
	return renderOwnedLine(width, bg, renderStyledOnBackground(style, bg, text))
}

func renderOwnedBlock(width int, bg color.Color, content string) string {
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = renderOwnedLine(width, bg, lines[i])
	}
	return strings.Join(lines, "\n")
}

func renderTechLabel(text string, bg color.Color) string {
	return renderStyledOnBackground(stylePanelLabel, bg, strings.ToUpper(text))
}

func renderKeycap(key string, bg color.Color) string {
	return renderStyledOnBackground(styleMutedText, bg, "[") +
		renderStyledOnBackground(styleKeycap, bg, strings.ToUpper(key)) +
		renderStyledOnBackground(styleMutedText, bg, "]")
}

func reapplyOwnedBackground(content string, bg color.Color) string {
	if content == "" || !strings.Contains(content, "\x1b[") {
		return content
	}

	bgSeq := ansiBackgroundSequence(bg)
	var builder strings.Builder
	builder.Grow(len(content) + len(bgSeq)*4)

	for idx := 0; idx < len(content); idx++ {
		if content[idx] != '\x1b' || idx+1 >= len(content) || content[idx+1] != '[' {
			builder.WriteByte(content[idx])
			continue
		}

		end := idx + 2
		for end < len(content) && content[end] != 'm' {
			end++
		}
		if end >= len(content) || content[end] != 'm' {
			builder.WriteByte(content[idx])
			continue
		}

		params := content[idx+2 : end]
		builder.WriteString(content[idx : end+1])
		if sgrClearsBackground(params) {
			builder.WriteString(bgSeq)
		}
		idx = end
	}

	return builder.String()
}

func ansiBackgroundSequence(bg color.Color) string {
	r, g, b, _ := bg.RGBA()
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>8, g>>8, b>>8)
}

func sgrClearsBackground(params string) bool {
	if params == "" {
		return true
	}
	for _, part := range strings.Split(params, ";") {
		if part == "" || part == "0" || part == "49" {
			return true
		}
	}
	return false
}
