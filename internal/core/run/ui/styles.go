package ui

import (
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
	// colorBgBase / colorBgSurface are retained ONLY as foreground colors for the
	// few accent *chips* that intentionally invert (text drawn on an accent fill).
	// The cockpit no longer paints a frame or surface background of its own — it is
	// foreground-only so the terminal's native background shows through, matching
	// the wizard.
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

// Pre-built styles reused across the UI. Every style here is foreground/border
// only: no Background()/BorderBackground() calls, so the terminal background is
// preserved end to end.
var (
	styleRootScreenBase = lipgloss.NewStyle().Foreground(colorFgBright)
	styleTechPanelBase  = lipgloss.NewStyle().
				BorderStyle(techBorder).
				Foreground(colorFgBright).
				Padding(0, 1)
	// styleTechSidebarBase frames the JOBS panel with a single column of inner
	// padding; the job cards stack inside it with no margin between them.
	styleTechSidebarBase = lipgloss.NewStyle().
				BorderStyle(techBorder).
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

// styleSidebarCardBase frames one job as a bordered card. The JOBS column has no
// outer panel border — the cards themselves are the structure — so the card is
// foreground/border only (no fill). Width is the full footprint, so the inner
// content area is width - GetHorizontalFrameSize().
var styleSidebarCardBase = lipgloss.NewStyle().
	BorderStyle(techBorder).
	Foreground(colorFgBright).
	Padding(0, 1)

var sidebarCardFrameWidth = styleSidebarCardBase.GetHorizontalFrameSize()

func sidebarCardStyle(width int, borderColor color.Color) lipgloss.Style {
	return styleSidebarCardBase.Width(max(width, 1)).BorderForeground(borderColor)
}

func sidebarCardSeparator(width int, borderColor color.Color) string {
	left := "├"
	right := "┤"
	fill := techBorder.Top
	if fill == "" {
		fill = "─"
	}
	width = max(width, 1)
	if width == 1 {
		return lipgloss.NewStyle().Foreground(borderColor).Render(fill)
	}
	innerWidth := max(width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	line := left + strings.Repeat(fill, innerWidth) + right
	return lipgloss.NewStyle().Foreground(borderColor).Render(line)
}

func sidebarCardContentWidth(width int) int {
	return max(width-sidebarCardFrameWidth, 1)
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

// renderGap returns n blank columns. With no owned background these are plain
// spaces that render with the terminal's background.
func renderGap(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(" ", width)
}

// renderOwnedLineKnownOwned pads content to a full-width line. Callers pre-truncate
// content to width, so this only adds trailing padding (the terminal background).
func renderOwnedLineKnownOwned(width int, content string) string {
	return lipgloss.NewStyle().
		Width(max(width, 1)).
		Foreground(colorFgBright).
		Render(content)
}

// renderOwnedBlock pads every line of a multi-line block to width.
func renderOwnedBlock(width int, content string) string {
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = renderOwnedLineKnownOwned(width, lines[i])
	}
	return strings.Join(lines, "\n")
}

func renderTechLabel(text string) string {
	return stylePanelLabel.Render(strings.ToUpper(text))
}
