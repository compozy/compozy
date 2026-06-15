package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/charmtheme"
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
	title := lipgloss.NewStyle().Bold(true).Foreground(headerColor).Render(headerText)

	pct := 0.0
	if m.total > 0 {
		pct = float64(m.completed+m.failed) / float64(m.total)
	}
	m.progressBar.SetWidth(max(innerW, 10))
	stats := []string{
		label.Render("SUCCEEDED") + renderGap(1) + lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSuccess).
			Render(fmt.Sprintf("%d", m.completed)),
		label.Render("FAILED    ") + renderGap(1) + lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError).
			Render(fmt.Sprintf("%d", m.failed)),
		label.Render("TOTAL     ") +
			renderGap(1) +
			value.Bold(true).Render(fmt.Sprintf("%d", m.total)),
	}

	progress := renderOwnedBlock(innerW, m.progressBar.ViewAs(pct))
	lines := []string{
		renderOwnedLineKnownOwned(innerW, renderTechLabel("run.status")),
		renderOwnedLineKnownOwned(innerW, title),
		progress,
		renderOwnedLineKnownOwned(innerW, ""),
	}
	for _, stat := range stats {
		lines = append(lines, renderOwnedLineKnownOwned(innerW, stat))
	}

	return techPanelStyle(boxW, borderColor).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) renderSummaryFailBox(boxW int) string {
	lines := []string{renderOwnedLineKnownOwned(panelContentWidth(boxW), renderTechLabel("run.failures"))}
	for _, f := range m.failures {
		entry := lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError).
			Render("FAIL " + f.CodeFile)
		entry += styleDimText.Render(fmt.Sprintf("  EXIT %d", f.ExitCode))
		lines = append(lines, renderOwnedLineKnownOwned(panelContentWidth(boxW), entry))
		if f.OutLog != "" {
			lines = append(
				lines,
				renderOwnedLineKnownOwned(
					panelContentWidth(boxW),
					styleMutedText.Render("  "+f.OutLog),
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
	innerW := panelContentWidth(boxW)

	lines := []string{
		renderOwnedLineKnownOwned(innerW, renderTechLabel("usage.tokens")),
		renderOwnedLineKnownOwned(
			innerW,
			label.Render("INPUT  ")+
				renderGap(1)+
				value.Render(formatNumber(u.InputTokens)),
		),
		renderOwnedLineKnownOwned(
			innerW,
			label.Render("OUTPUT ")+
				renderGap(1)+
				value.Render(formatNumber(u.OutputTokens)),
		),
		renderOwnedLineKnownOwned(
			innerW,
			label.Render("CACHER ")+
				renderGap(1)+
				value.Render(formatNumber(u.CacheReads)),
		),
		renderOwnedLineKnownOwned(
			innerW,
			label.Render("CACHEW ")+
				renderGap(1)+
				value.Render(formatNumber(u.CacheWrites)),
		),
	}
	totalValue := lipgloss.NewStyle().Bold(true).Foreground(colorBrand).Render(formatNumber(u.Total()))
	lines = append(
		lines,
		renderOwnedLineKnownOwned(
			innerW,
			label.Render("TOTAL  ")+renderGap(1)+totalValue,
		),
	)

	return techPanelStyle(boxW, colorBorder).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) renderSummaryHelp(width int) string {
	parts := []string{
		charmtheme.Keycap("esc") + renderGap(1) + styleMutedText.Render("BACK"),
		charmtheme.Keycap("q") + renderGap(1) + styleMutedText.Render("QUIT"),
	}
	line := renderGap(1) + strings.Join(parts, renderGap(2))
	return lipgloss.NewStyle().MarginTop(1).Render(renderOwnedLineKnownOwned(width, line))
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
