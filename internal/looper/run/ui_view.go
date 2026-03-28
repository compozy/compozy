package run

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *uiModel) renderSummaryView() string {
	sections := []string{m.renderSummaryHeader(), m.renderSummaryCounts()}
	if len(m.failures) > 0 {
		sections = append(sections, m.renderSummaryFailures())
	}
	if m.aggregateUsage != nil && m.aggregateUsage.Total() > 0 {
		sections = append(sections, m.renderSummaryTokenUsage())
	}
	sections = append(sections, m.renderSummaryHelp())
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(strings.Repeat("─", m.width))
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return lipgloss.JoinVertical(lipgloss.Left, separator, content)
}

func (m *uiModel) renderFailuresView() string {
	if len(m.failures) == 0 {
		noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(2)
		return noteStyle.Render("No failures recorded. Return with 'esc'.")
	}
	rows := []string{"Failure Details:"}
	for _, f := range m.failures {
		rows = append(rows,
			fmt.Sprintf("• %s (exit %d)", f.codeFile, f.exitCode),
			fmt.Sprintf("  Logs: %s (out), %s (err)", f.outLog, f.errLog),
		)
	}
	block := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(strings.Join(rows, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, block, m.renderSummaryHelp())
}

func (m *uiModel) renderSummaryHeader() string {
	headerStyle := lipgloss.NewStyle().Bold(true).MarginTop(1).MarginBottom(1)
	if m.failed > 0 {
		headerStyle = headerStyle.Foreground(lipgloss.Color("220"))
		return headerStyle.Render(fmt.Sprintf("✓ Execution Complete: %d/%d succeeded, %d failed", m.completed, m.total, m.failed))
	}
	headerStyle = headerStyle.Foreground(lipgloss.Color("42"))
	return headerStyle.Render(fmt.Sprintf("✓ All Jobs Complete: %d/%d succeeded!", m.completed, m.total))
}

func (m *uiModel) renderSummaryCounts() string {
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("81")).MarginBottom(1)
	summaryText := fmt.Sprintf("Total Groups: %d\nSuccess: %d\nFailed: %d", m.total, m.completed, m.failed)
	return summaryStyle.Render(summaryText)
}

func (m *uiModel) renderSummaryFailures() string {
	failuresStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).MarginBottom(1)
	failureLines := []string{"Failures:"}
	for _, f := range m.failures {
		failureLines = append(failureLines,
			fmt.Sprintf("  • %s (exit code: %d)", f.codeFile, f.exitCode),
			fmt.Sprintf("    Logs: %s (out), %s (err)", f.outLog, f.errLog))
	}
	return failuresStyle.Render(strings.Join(failureLines, "\n"))
}

func (m *uiModel) renderSummaryTokenUsage() string {
	usageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("141")).MarginBottom(1)
	usageLines := []string{
		"Token Usage (Claude API - Aggregate):",
		fmt.Sprintf("  Input:  %s tokens", formatNumber(m.aggregateUsage.InputTokens)),
	}
	if m.aggregateUsage.CacheReadTokens > 0 {
		usageLines = append(usageLines, fmt.Sprintf("  Cache Reads: %s tokens", formatNumber(m.aggregateUsage.CacheReadTokens)))
	}
	if m.aggregateUsage.CacheCreationTokens > 0 {
		usageLines = append(usageLines, fmt.Sprintf("  Cache Creation: %s tokens", formatNumber(m.aggregateUsage.CacheCreationTokens)))
	}
	usageLines = append(usageLines,
		fmt.Sprintf("  Output: %s tokens", formatNumber(m.aggregateUsage.OutputTokens)),
		fmt.Sprintf("  Total:  %s tokens", formatNumber(m.aggregateUsage.Total())),
	)
	return usageStyle.Render(strings.Join(usageLines, "\n"))
}

func (m *uiModel) renderSummaryHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(2)
	return helpStyle.Render("Press 'esc' to return to job list • Press 'q' to exit")
}

func (m *uiModel) View() string {
	switch m.currentView {
	case uiViewSummary:
		return m.renderSummaryView()
	case uiViewFailures:
		return m.renderFailuresView()
	case uiViewJobs:
		header, headerStyle := m.renderHeader()
		helpText, helpStyle := m.renderHelp()
		separator := m.renderSeparator()
		splitView := lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(), m.renderMainContent())
		return lipgloss.JoinVertical(
			lipgloss.Left,
			headerStyle.Render(header),
			helpStyle.Render(helpText),
			separator,
			splitView,
		)
	default:
		return ""
	}
}

func (m *uiModel) renderHeader() (string, lipgloss.Style) {
	complete := m.completed+m.failed >= m.total
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")).MarginTop(1).MarginBottom(1)
	if !complete {
		msg := fmt.Sprintf("Processing Jobs: %d/%d completed, %d failed", m.completed, m.total, m.failed)
		return msg, style
	}
	if m.failed > 0 {
		style = style.Foreground(lipgloss.Color("220"))
		return fmt.Sprintf("✓ All Jobs Complete: %d/%d succeeded, %d failed", m.completed, m.total, m.failed), style
	}
	style = style.Foreground(lipgloss.Color("42"))
	return fmt.Sprintf("✓ All Jobs Complete: %d/%d succeeded!", m.completed, m.total), style
}

func (m *uiModel) renderHelp() (string, lipgloss.Style) {
	complete := m.completed+m.failed >= m.total
	text := "↑↓/jk navigate • pgup/pgdn scroll logs • q quit"
	if complete {
		text = "↑↓/jk navigate • pgup/pgdn scroll logs • press 's' to view summary • q quit"
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginBottom(1)
	return text, style
}

func (m *uiModel) renderSeparator() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(strings.Repeat("─", m.width))
}

func (m *uiModel) renderSidebar() string {
	sidebarWidth := m.sidebarWidth
	if sidebarWidth <= 0 {
		sidebarWidth = int(float64(m.width) * sidebarWidthRatio)
		if sidebarWidth < sidebarMinWidth {
			sidebarWidth = sidebarMinWidth
		}
		if sidebarWidth > sidebarMaxWidth {
			sidebarWidth = sidebarMaxWidth
		}
	}
	contentHeight := m.contentHeight
	if contentHeight < minContentHeight {
		contentHeight = minContentHeight
	}
	var items []string
	for i := range m.jobs {
		items = append(items, m.renderSidebarItem(&m.jobs[i], i == m.selectedJob))
	}
	m.sidebarViewport.SetContent(strings.Join(items, "\n"))
	if m.selectedJob >= 0 && m.selectedJob < len(m.jobs) {
		lineOffset := m.selectedJob * 3
		if lineOffset > m.sidebarViewport.YOffset+m.sidebarViewport.Height-3 {
			m.sidebarViewport.SetYOffset(lineOffset - m.sidebarViewport.Height + 3)
		} else if lineOffset < m.sidebarViewport.YOffset {
			m.sidebarViewport.SetYOffset(lineOffset)
		}
	}
	return lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(contentHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Render(m.sidebarViewport.View())
}

func (m *uiModel) renderSidebarItem(job *uiJob, selected bool) string {
	var icon string
	var color lipgloss.Color
	switch job.state {
	case jobPending:
		icon = "⏸"
		color = lipgloss.Color("245")
	case jobRunning:
		icon = spinnerFrames[m.frame%len(spinnerFrames)]
		color = lipgloss.Color("220")
	case jobSuccess:
		icon = "✓"
		color = lipgloss.Color("42")
	case jobFailed:
		icon = "✗"
		color = lipgloss.Color("196")
	}

	maxTextWidth := m.sidebarViewport.Width - 2
	line1 := truncateString(fmt.Sprintf("%s %s", icon, job.safeName), maxTextWidth)
	line2 := truncateString(fmt.Sprintf("  %d file(s), %d issue(s)", len(job.codeFiles), job.issues), maxTextWidth)

	style := lipgloss.NewStyle().Foreground(color)
	if selected {
		style = style.Bold(true).Background(lipgloss.Color("235")).Foreground(lipgloss.Color("255"))
	}
	if selected {
		line1 = "► " + line1
	} else {
		line1 = "  " + line1
	}
	line1 = truncateString(line1, maxTextWidth)
	return style.Render(line1) + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(line2)
}

func (m *uiModel) renderMainContent() string {
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		return lipgloss.NewStyle().Padding(2).Foreground(lipgloss.Color("245")).Render("Select a job from the sidebar")
	}
	job := &m.jobs[m.selectedJob]
	mainWidth, contentHeight := m.mainDimensions()
	metaBlock := m.buildMetaBlock(job)
	logsHeader := m.renderLogsHeader()
	m.viewport.Height = m.availableLogHeight(contentHeight, metaBlock, logsHeader)
	m.updateViewportForJob(job)
	body := lipgloss.JoinVertical(lipgloss.Left, metaBlock, logsHeader, m.viewport.View())
	return lipgloss.NewStyle().Width(mainWidth).Height(contentHeight).Padding(0, 1).Render(body)
}

func (m *uiModel) buildMetaBlock(job *uiJob) string {
	sections := []string{m.renderMainHeader(job)}
	if fileList := strings.TrimSpace(m.renderMainFileList(job)); fileList != "" {
		sections = append(sections, fileList)
	}
	sections = append(sections, m.renderMainStatus(job), m.renderRuntime(job))
	if usage := strings.TrimSpace(m.renderTokenUsage(job)); usage != "" {
		sections = append(sections, usage)
	}
	if paths := strings.TrimSpace(m.renderLogPaths(job)); paths != "" {
		sections = append(sections, paths)
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *uiModel) renderMainHeader(job *uiJob) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1).Render(fmt.Sprintf("Batch: %s", job.safeName))
}

func (m *uiModel) renderMainFileList(job *uiJob) string {
	if len(job.codeFiles) == 0 {
		return ""
	}
	maxTextWidth := m.viewport.Width - 4
	var b strings.Builder
	b.WriteString("Files:\n")
	for _, file := range job.codeFiles {
		b.WriteString("  • ")
		b.WriteString(truncateString(file, maxTextWidth))
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("212")).MarginBottom(1).Render(b.String())
}

func (m *uiModel) renderMainStatus(job *uiJob) string {
	statusLabel := m.getStateLabel(job.state)
	if job.state == jobFailed && job.exitCode != 0 {
		statusLabel = fmt.Sprintf("%s (exit %d)", statusLabel, job.exitCode)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("81")).MarginBottom(1).Render(fmt.Sprintf("Issues: %d  |  Status: %s", job.issues, statusLabel))
}

func (m *uiModel) renderLogsHeader() string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1).Render("Live Logs:")
}

func (m *uiModel) renderRuntime(job *uiJob) string {
	var label string
	var duration time.Duration
	switch job.state {
	case jobRunning:
		label = "Runtime"
		if !job.startedAt.IsZero() {
			duration = time.Since(job.startedAt)
		}
	case jobSuccess:
		label = "Completed in"
		if job.duration > 0 {
			duration = job.duration
		} else if !job.startedAt.IsZero() {
			duration = time.Since(job.startedAt)
		}
	case jobFailed:
		label = "Ran for"
		if job.duration > 0 {
			duration = job.duration
		} else if !job.startedAt.IsZero() {
			duration = time.Since(job.startedAt)
		}
	default:
		label = "Runtime"
		if !job.startedAt.IsZero() {
			duration = time.Since(job.startedAt)
		}
	}
	rendered := fmt.Sprintf("%s: --:--", label)
	if duration > 0 {
		rendered = fmt.Sprintf("%s: %s", label, formatDuration(duration))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("117")).MarginBottom(1).Render(rendered)
}

func (m *uiModel) renderLogPaths(job *uiJob) string {
	var lines []string
	if job.outLog != "" {
		lines = append(lines, fmt.Sprintf("  • stdout: %s", job.outLog))
	}
	if job.errLog != "" {
		lines = append(lines, fmt.Sprintf("  • stderr: %s", job.errLog))
	}
	if len(lines) == 0 {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginBottom(1).Render("Log Files:\n" + strings.Join(lines, "\n"))
}

func (m *uiModel) renderTokenUsage(job *uiJob) string {
	if job.tokenUsage == nil {
		return ""
	}
	usage := job.tokenUsage
	var lines []string
	lines = append(lines, "Token Usage (Claude API):", fmt.Sprintf("  Input:          %s tokens", formatNumber(usage.InputTokens)))
	if usage.CacheReadTokens > 0 {
		lines = append(lines, fmt.Sprintf("  Cache Reads:    %s tokens", formatNumber(usage.CacheReadTokens)))
	}
	if usage.CacheCreationTokens > 0 {
		lines = append(lines, fmt.Sprintf("  Cache Creation: %s tokens", formatNumber(usage.CacheCreationTokens)))
	}
	lines = append(lines,
		fmt.Sprintf("  Output:         %s tokens", formatNumber(usage.OutputTokens)),
		fmt.Sprintf("  Total:          %s tokens", formatNumber(usage.Total())),
	)
	return lipgloss.NewStyle().Foreground(lipgloss.Color("141")).MarginBottom(1).Render(strings.Join(lines, "\n"))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	hours := int(d / time.Hour)
	minutes := int((d % time.Hour) / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
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

func (m *uiModel) updateViewportForJob(job *uiJob) {
	var content strings.Builder
	if len(job.lastOut) > 0 {
		for _, line := range job.lastOut {
			if line != "" {
				content.WriteString(line)
				content.WriteString("\n")
			}
		}
	}
	if len(job.lastErr) > 0 {
		stderrLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("[stderr]")
		content.WriteString(stderrLabel)
		content.WriteString("\n")
		for _, line := range job.lastErr {
			if line != "" {
				content.WriteString(line)
				content.WriteString("\n")
			}
		}
	}
	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
	if len(job.lastOut) == 0 && len(job.lastErr) == 0 {
		m.viewport.GotoTop()
	}
}

func (m *uiModel) getStateLabel(state jobState) string {
	switch state {
	case jobPending:
		return "Pending"
	case jobRunning:
		return "Running"
	case jobSuccess:
		return "Success ✓"
	case jobFailed:
		return "Failed ✗"
	default:
		return "Unknown"
	}
}
