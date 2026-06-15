package cli

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/charmtheme"
)

// Presentation layer for the task-run wizard. The wizard renders inline (no
// alt-screen), so visual identity comes from the shared TechBorder, focus-aware
// panel borders, the brand header, a numbered stepper, and keycap hints — the
// same language as the live run UI in internal/core/run/ui.

const (
	wizardFieldLabelWidth    = 18
	wizardOverrideLabelWidth = 12
)

var wizardStepLabels = []string{"Workflows", "Runtime", "Execution", "Overrides", "Review"}

// wizardChromeStyle frames the whole wizard. width is the full terminal width;
// lipgloss Width() here is the total output width, so the inner content area is
// width - GetHorizontalFrameSize() (border 2 + padding 2 = 4). Callers size
// header/body/footer lines to that inner width so the solid dividers never wrap.
func wizardChromeStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(charmtheme.TechBorder).
		BorderForeground(charmtheme.ColorAccentDeep).
		Foreground(charmtheme.ColorFgBright).
		Padding(0, 1).
		Width(max(1, width))
}

func wizardChevronStyle(active bool) lipgloss.Style {
	if active {
		return lipgloss.NewStyle().Foreground(charmtheme.ColorAccentDeep)
	}
	return lipgloss.NewStyle().Foreground(charmtheme.ColorDim)
}

func wizardDoneStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(charmtheme.ColorAccentDeep)
}

func wizardPendingStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(charmtheme.ColorDim)
}

func wizardSuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(charmtheme.ColorSuccess)
}

func wizardHR(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(charmtheme.ColorBorder).
		Render(strings.Repeat("─", width))
}

func wizardBrandLine(step taskRunWizardStep, width int) string {
	left := taskRunWizardTitleStyle().Render("COMPOZY // TASK RUN")
	right := taskRunWizardMutedStyle().Render(fmt.Sprintf("Step %d of %d", int(step)+1, len(wizardStepLabels)))
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	line := left + strings.Repeat(" ", gap) + right
	return taskRunWizardTruncate(line, width)
}

func wizardStepper(step taskRunWizardStep, width int) string {
	segments := make([]string, 0, len(wizardStepLabels))
	for i := range wizardStepLabels {
		label := wizardStepLabels[i]
		dot := "●"
		var style lipgloss.Style
		switch {
		case taskRunWizardStep(i) == step:
			style = taskRunWizardActiveStyle()
		case i < int(step):
			style = wizardDoneStyle()
		default:
			style = wizardPendingStyle()
			dot = "○"
		}
		segments = append(segments, style.Render(dot+" "+label))
	}
	return taskRunWizardTruncate(strings.Join(segments, "   "), width)
}

func wizardFooterHint(pairs [][2]string) string {
	if len(pairs) == 0 {
		return ""
	}
	muted := taskRunWizardMutedStyle()
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, charmtheme.Keycap(pair[0])+muted.Render(pair[1]))
	}
	return strings.Join(parts, muted.Render("  "))
}

func wizardSelectValue(label string, active bool) string {
	chevron := wizardChevronStyle(active)
	value := taskRunWizardSubtitleStyle()
	if active {
		value = taskRunWizardActiveStyle()
	}
	return chevron.Render("‹ ") + value.Render(label) + chevron.Render(" ›")
}

func wizardBoolValue(on bool) string {
	if on {
		return wizardSuccessStyle().Render("✓ on")
	}
	return wizardPendingStyle().Render("✗ off")
}

func wizardPadRight(s string, width int) string {
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func wizardField(label string, value string, active bool, labelWidth int) string {
	marker := "  "
	labelStyle := taskRunWizardMutedStyle()
	if active {
		marker = taskRunWizardActiveStyle().Render("▸ ")
		labelStyle = taskRunWizardActiveStyle()
	}
	return marker + labelStyle.Render(wizardPadRight(label, labelWidth)) + " " + value
}

func wizardPaneTitle(title string, focused bool, suffix string) string {
	styled := taskRunWizardMutedStyle().Render(title)
	if focused {
		styled = taskRunWizardActiveStyle().Render(title)
	}
	if suffix != "" {
		styled += " " + taskRunWizardMutedStyle().Render(suffix)
	}
	return styled
}

// wizardRenderPane draws content inside a focus-aware bordered panel of a fixed
// total footprint and row count so side-by-side panes align cleanly.
func wizardRenderPane(total int, rows int, focused bool, lines []string) string {
	inner := max(1, total-4)
	out := make([]string, 0, rows)
	for _, line := range lines {
		if len(out) >= rows {
			break
		}
		out = append(out, taskRunWizardTruncate(line, inner))
	}
	for len(out) < rows {
		out = append(out, "")
	}
	return charmtheme.TechPanelStyle(total, focused).Render(strings.Join(out, "\n"))
}

// wizardClampBody pads or trims the body to exactly height lines and truncates
// every line to width so the surrounding chrome stays a stable rectangle.
func wizardClampBody(body string, height int, width int) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, height)
	for _, line := range lines {
		if len(out) >= height {
			break
		}
		out = append(out, taskRunWizardTruncate(line, width))
	}
	for len(out) < height {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func wizardSummaryRow(label string, value string, width int) string {
	labelCell := taskRunWizardMutedStyle().Render(wizardPadRight(label, 12))
	return "  " + labelCell + " " + taskRunWizardTruncate(value, max(8, width-16))
}
