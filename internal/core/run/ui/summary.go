package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *uiModel) renderSummaryView() tea.View {
	boxW := min(m.width-4, 80)
	sections := []string{m.renderSummaryMainBox(boxW)}
	if len(m.failures) > 0 {
		sections = append(sections, m.renderSummaryFailBox(boxW))
	}
	if m.aggregateUsage != nil && hasUsage(*m.aggregateUsage) {
		sections = append(sections, m.renderSummaryTokenBox(boxW))
	}
	sections = append(sections, m.renderSummaryHelp(boxW))

	content := lipgloss.NewStyle().MarginTop(1).MarginLeft(1).Render(
		lipgloss.JoinVertical(lipgloss.Left, sections...))
	return m.renderRoot(content)
}

func (m *uiModel) renderSummaryMainBox(boxW int) string {
	innerW := panelContentWidth(boxW)
	label := styleDimText
	value := styleBodyText
	bg := colorBgSurface

	borderColor := colorBorderFocus
	headerColor := colorSuccess
	headerText := fmt.Sprintf("All Jobs Complete: %d/%d succeeded", m.completed, m.total)
	if m.failed > 0 {
		borderColor = colorWarning
		headerColor = colorWarning
		headerText = fmt.Sprintf(
			"Execution Complete: %d/%d succeeded, %d failed",
			m.completed, m.total, m.failed)
	}
	title := renderStyledOnBackground(
		lipgloss.NewStyle().Bold(true).Foreground(headerColor),
		bg,
		headerText,
	)

	pct := 0.0
	if m.total > 0 {
		pct = float64(m.completed+m.failed) / float64(m.total)
	}
	m.progressBar.SetWidth(max(innerW, 10))
	stats := []string{
		renderStyledOnBackground(label, bg, "SUCCEEDED") + renderGap(bg, 1) + lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSuccess).
			Background(bg).
			Render(fmt.Sprintf("%d", m.completed)),
		renderStyledOnBackground(label, bg, "FAILED    ") + renderGap(bg, 1) + lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError).
			Background(bg).
			Render(fmt.Sprintf("%d", m.failed)),
		renderStyledOnBackground(label, bg, "TOTAL     ") +
			renderGap(bg, 1) +
			renderStyledOnBackground(value.Bold(true), bg, fmt.Sprintf("%d", m.total)),
	}

	progress := renderOwnedBlock(innerW, bg, m.progressBar.ViewAs(pct))
	lines := []string{
		renderOwnedLineKnownOwned(innerW, bg, renderTechLabel("run.status", bg)),
		renderOwnedLineKnownOwned(innerW, bg, title),
		progress,
		renderOwnedLineKnownOwned(innerW, bg, ""),
	}
	for _, stat := range stats {
		lines = append(lines, renderOwnedLineKnownOwned(innerW, bg, stat))
	}

	return techPanelStyle(boxW, borderColor).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) renderSummaryFailBox(boxW int) string {
	bg := colorBgSurface
	lines := []string{renderOwnedLineKnownOwned(panelContentWidth(boxW), bg, renderTechLabel("run.failures", bg))}
	for _, f := range m.failures {
		entry := lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError).
			Background(bg).
			Render("FAIL " + f.CodeFile)
		entry += renderStyledOnBackground(styleDimText, bg, fmt.Sprintf("  EXIT %d", f.ExitCode))
		lines = append(lines, renderOwnedLineKnownOwned(panelContentWidth(boxW), bg, entry))
		if f.OutLog != "" {
			lines = append(
				lines,
				renderOwnedLineKnownOwned(
					panelContentWidth(boxW),
					bg,
					renderStyledOnBackground(styleMutedText, bg, "  "+f.OutLog),
				),
			)
		}
	}
	return techPanelStyle(boxW, colorError).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) renderSummaryTokenBox(boxW int) string {
	label := styleDimText
	value := styleBodyText
	u := m.aggregateUsage
	bg := colorBgSurface
	innerW := panelContentWidth(boxW)

	lines := []string{
		renderOwnedLineKnownOwned(innerW, bg, renderTechLabel("usage.tokens", bg)),
		renderOwnedLineKnownOwned(
			innerW,
			bg,
			renderStyledOnBackground(label, bg, "INPUT  ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.InputTokens)),
		),
		renderOwnedLineKnownOwned(
			innerW,
			bg,
			renderStyledOnBackground(label, bg, "OUTPUT ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.OutputTokens)),
		),
		renderOwnedLineKnownOwned(
			innerW,
			bg,
			renderStyledOnBackground(label, bg, "CACHER ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.CacheReads)),
		),
		renderOwnedLineKnownOwned(
			innerW,
			bg,
			renderStyledOnBackground(label, bg, "CACHEW ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.CacheWrites)),
		),
	}
	totalValue := lipgloss.NewStyle().Bold(true).Foreground(colorBrand).Background(bg).Render(formatNumber(u.Total()))
	lines = append(
		lines,
		renderOwnedLineKnownOwned(
			innerW,
			bg,
			renderStyledOnBackground(label, bg, "TOTAL  ")+renderGap(bg, 1)+totalValue,
		),
	)

	return techPanelStyle(boxW, colorBorder).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) renderSummaryHelp(width int) string {
	bg := colorBgBase
	parts := []string{
		renderKeycap("esc", bg) + renderGap(bg, 1) + renderStyledOnBackground(styleMutedText, bg, "BACK"),
		renderKeycap("q", bg) + renderGap(bg, 1) + renderStyledOnBackground(styleMutedText, bg, "QUIT"),
	}
	line := renderGap(bg, 1) + strings.Join(parts, renderGap(bg, 2))
	return lipgloss.NewStyle().MarginTop(1).Render(renderOwnedLineKnownOwned(width, bg, line))
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	mod := len(s) % 3
	if mod > 0 {
		result.WriteString(s[:mod])
		if len(s) > mod {
			result.WriteString(",")
		}
	}
	for i := mod; i < len(s); i += 3 {
		if i > mod {
			result.WriteString(",")
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}
