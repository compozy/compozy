package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/charmtheme"
)

func (m *uiModel) renderSummaryView() tea.View {
	boxW := min(m.width-4, 80)
	sections := []string{m.renderSummaryMainBox(boxW)}
	if parked := m.parkedJobs(); len(parked) > 0 {
		sections = append(sections, m.renderSummaryParkedBox(boxW, parked))
	}
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

// summaryStatLabelWidth pads every stat label to one column so the values line up
// in a single column regardless of label length.
const summaryStatLabelWidth = 9

func (m *uiModel) renderSummaryMainBox(boxW int) string {
	innerW := panelContentWidth(boxW)
	borderColor := colorBorderFocus
	headerColor := colorSuccess
	if m.failed > 0 || m.parked > 0 {
		borderColor = colorWarning
		headerColor = colorWarning
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(headerColor).Render(m.summaryHeaderText())

	pct := 0.0
	if m.total > 0 {
		pct = float64(m.settledJobs()) / float64(m.total)
	}
	m.progressBar.SetWidth(max(innerW, 10))

	progress := renderOwnedBlock(innerW, m.progressBar.ViewAs(pct))
	lines := []string{
		renderOwnedLineKnownOwned(innerW, renderTechLabel("run.status")),
		renderOwnedLineKnownOwned(innerW, title),
		progress,
		renderOwnedLineKnownOwned(innerW, ""),
	}
	for _, stat := range m.summaryStatLines() {
		lines = append(lines, renderOwnedLineKnownOwned(innerW, stat))
	}

	return techPanelStyle(boxW, borderColor).Render(strings.Join(lines, "\n"))
}

// summaryHeaderText is the closing line the walked-away user reads first. It
// stays a plain success line for a clean run and names recovered and parked jobs
// only when there were any.
func (m *uiModel) summaryHeaderText() string {
	if m.failed == 0 && m.parked == 0 {
		if m.recovered > 0 {
			return fmt.Sprintf(
				"All Jobs Complete: %d/%d succeeded, %d recovered",
				m.completed, m.total, m.recovered)
		}
		return fmt.Sprintf("All Jobs Complete: %d/%d succeeded", m.completed, m.total)
	}
	segments := []string{fmt.Sprintf("Execution Complete: %d/%d succeeded", m.completed, m.total)}
	if m.recovered > 0 {
		segments = append(segments, fmt.Sprintf("%d recovered", m.recovered))
	}
	if m.parked > 0 {
		segments = append(segments, fmt.Sprintf("%d parked", m.parked))
	}
	if m.failed > 0 {
		segments = append(segments, fmt.Sprintf("%d failed", m.failed))
	}
	return strings.Join(segments, ", ")
}

// summaryStatLines renders the completed / recovered / parked / failed breakdown.
// Recovered and parked are always shown, at zero for a run with no stalls, so the
// closing box reads the same shape whether or not recovery happened.
func (m *uiModel) summaryStatLines() []string {
	return []string{
		summaryStatLine("SUCCEEDED", m.completed, colorSuccess),
		summaryStatLine("RECOVERED", m.recovered, colorInfo),
		summaryStatLine("PARKED", m.parked, colorAccent),
		summaryStatLine("FAILED", m.failed, colorError),
		summaryStatLine("TOTAL", m.total, colorFgBright),
	}
}

func summaryStatLine(label string, count int, valueColor color.Color) string {
	padded := label + strings.Repeat(" ", max(summaryStatLabelWidth-len(label), 0))
	value := lipgloss.NewStyle().Bold(true).Foreground(valueColor).Render(strconv.Itoa(count))
	return styleDimText.Render(padded) + renderGap(1) + value
}

// parkedDetailLabelWidth pads every parked-detail label so the values align in a
// single column beneath the job title.
const parkedDetailLabelWidth = 10

// parkedJobs returns the parked jobs in run order. Parked is terminal, so this is
// the closing set the returning user has to triage.
func (m *uiModel) parkedJobs() []*uiJob {
	var parked []*uiJob
	for i := range m.jobs {
		if m.jobs[i].state == jobParked {
			parked = append(parked, &m.jobs[i])
		}
	}
	return parked
}

// renderSummaryParkedBox is the triage panel. It answers, for every parked job,
// the four questions the user would otherwise have to dig for: why it parked,
// what the agent was last doing, how far it got, and where the preserved worktree
// and log live.
func (m *uiModel) renderSummaryParkedBox(boxW int, parked []*uiJob) string {
	innerW := panelContentWidth(boxW)
	lines := []string{renderOwnedLineKnownOwned(innerW, renderTechLabel("run.parked"))}
	for _, job := range parked {
		title := lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Render(jobIconParked + " " + statusLabelParked + " " + sidebarJobTitle(job))
		lines = append(lines, renderOwnedLineKnownOwned(innerW, title))
		for _, detail := range parkedDetailLines(job) {
			lines = append(lines, renderOwnedLineKnownOwned(innerW, detail))
		}
	}
	return techPanelStyle(boxW, colorAccent).Render(strings.Join(lines, "\n"))
}

// parkedDetailLines renders the JobParkedPayload detail carried on a parked job.
// Empty fields are skipped so a park that lacks, say, a log path never renders a
// dangling label.
func parkedDetailLines(job *uiJob) []string {
	details := []struct {
		label string
		value string
	}{
		{"reason", job.stallReason},
		{"last call", job.stallLastToolCall},
		{"progress", parkedProgressValue(job.parkProgressSeq)},
		{"worktree", job.worktreePath},
		{"log", job.parkLogPath},
	}
	lines := make([]string, 0, len(details))
	for _, detail := range details {
		value := strings.TrimSpace(detail.value)
		if value == "" {
			continue
		}
		padded := detail.label + strings.Repeat(" ", max(parkedDetailLabelWidth-len(detail.label), 0))
		lines = append(lines, "  "+styleDimText.Render(padded)+renderGap(1)+styleBodyText.Render(value))
	}
	return lines
}

// parkedProgressValue renders the journal high-water sequence reached before the
// stall. Sequence zero means no durable progress was recorded, which is not worth
// a line of its own.
func parkedProgressValue(seq uint64) string {
	if seq == 0 {
		return ""
	}
	return "seq " + strconv.FormatUint(seq, 10)
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
