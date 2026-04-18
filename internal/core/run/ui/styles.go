package ui

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

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
	styleRootScreenBase = lipgloss.NewStyle().
				Background(colorBgBase).
				Foreground(colorFgBright)
	styleTechPanelBase = lipgloss.NewStyle().
				BorderStyle(techBorder).
				BorderBackground(colorBgSurface).
				Background(colorBgSurface).
				Foreground(colorFgBright).
				Padding(0, 1)
	styleTechSidebarBase = lipgloss.NewStyle().
				BorderStyle(techBorder).
				BorderBackground(colorBgSurface).
				Background(colorBgSurface).
				Foreground(colorFgBright).
				Padding(0, 1)
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
	panelFrameWidth    = styleTechPanelBase.GetHorizontalFrameSize()
	sidebarFrameWidth  = styleTechSidebarBase.GetHorizontalFrameSize()
	sidebarFrameHeight = styleTechSidebarBase.GetVerticalFrameSize()
)

func rootScreenStyle(width, height int) lipgloss.Style {
	return styleRootScreenBase.
		Width(max(width, 1)).
		Height(max(height, 1))
}

func techPanelStyle(renderWidth int, borderColor color.Color) lipgloss.Style {
	return styleTechPanelBase.Width(renderWidth).BorderForeground(borderColor)
}

func techSidebarStyle(width int, borderColor color.Color) lipgloss.Style {
	return styleTechSidebarBase.Width(width).BorderForeground(borderColor)
}

func selectedSidebarRowStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().Width(max(width, 1))
}

func panelContentWidth(width int) int {
	return max(width-panelFrameWidth, 1)
}

func sidebarContentWidth(width int) int {
	return max(width-sidebarFrameWidth, 1)
}

func sidebarContentHeight(height int) int {
	return max(height-sidebarFrameHeight, 1)
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
	return renderOwnedLineWithOwnership(width, bg, content, false)
}

func renderOwnedLineKnownOwned(width int, bg color.Color, content string) string {
	return renderOwnedLineWithOwnership(width, bg, content, true)
}

func renderOwnedLineWithOwnership(width int, bg color.Color, content string, owned bool) string {
	if !owned {
		content = reapplyOwnedBackground(content, bg)
	}
	return lipgloss.NewStyle().
		Width(max(width, 1)).
		Foreground(colorFgBright).
		Background(bg).
		Render(content)
}

func renderStyledOwnedLine(width int, style lipgloss.Style, bg color.Color, text string) string {
	return renderOwnedLineKnownOwned(width, bg, renderStyledOnBackground(style, bg, text))
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

var ansiBackgroundSequenceCache sync.Map

func ansiBackgroundSequence(bg color.Color) string {
	r, g, b, _ := bg.RGBA()
	key := r>>8<<16 | g>>8<<8 | b>>8
	if cached, ok := ansiBackgroundSequenceCache.Load(key); ok {
		if sequence, ok := cached.(string); ok {
			return sequence
		}
	}
	sequence := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>8, g>>8, b>>8)
	actual, _ := ansiBackgroundSequenceCache.LoadOrStore(key, sequence)
	if cached, ok := actual.(string); ok {
		return cached
	}
	return sequence
}

func sgrClearsBackground(params string) bool {
	if params == "" {
		return true
	}
	start := 0
	for start <= len(params) {
		end := start
		for end < len(params) && params[end] != ';' {
			end++
		}
		part := params[start:end]
		if part == "" || part == "0" || part == "49" {
			return true
		}
		start = end + 1
	}
	return false
}
