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
		return headerStyle.Render(
			fmt.Sprintf("Execution Complete: %d/%d succeeded, %d failed", m.completed, m.total, m.failed),
		)
	}
	headerStyle = headerStyle.Foreground(lipgloss.Color("42"))
	return headerStyle.Render(fmt.Sprintf("All Jobs Complete: %d/%d succeeded", m.completed, m.total))
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

func (m *uiModel) renderSummaryHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(2)
	return helpStyle.Render("Press 'esc' to return to jobs • Press 'q' to exit")
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
	mode := "navigate"
	if m.mode == modeTerminal {
		mode = "terminal"
	}
	if !complete {
		msg := fmt.Sprintf(
			"Processing Jobs: %d/%d completed, %d failed • Mode: %s",
			m.completed,
			m.total,
			m.failed,
			mode,
		)
		return msg, style
	}
	if m.failed > 0 {
		style = style.Foreground(lipgloss.Color("220"))
		return fmt.Sprintf(
			"All Jobs Complete: %d/%d succeeded, %d failed • Mode: %s",
			m.completed,
			m.total,
			m.failed,
			mode,
		), style
	}
	style = style.Foreground(lipgloss.Color("42"))
	return fmt.Sprintf("All Jobs Complete: %d/%d succeeded • Mode: %s", m.completed, m.total, mode), style
}

func (m *uiModel) renderHelp() (string, lipgloss.Style) {
	complete := m.completed+m.failed >= m.total
	var text string
	if m.mode == modeTerminal {
		text = "terminal mode • all keys go to the active PTY • esc return to navigate"
	} else {
		text = "navigate mode • ↑↓/jk select • enter open terminal • q quit"
		if complete {
			text += " • s summary"
		}
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

	items := []string{m.renderModeIndicator()}
	for i := range m.jobs {
		items = append(items, m.renderSidebarItem(&m.jobs[i], i == m.selectedJob))
	}
	m.sidebarViewport.SetContent(strings.Join(items, "\n"))
	if m.selectedJob >= 0 && m.selectedJob < len(m.jobs) {
		lineOffset := 2 + (m.selectedJob * 3)
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

func (m *uiModel) renderModeIndicator() string {
	label := "Navigate"
	color := lipgloss.Color("81")
	if m.mode == modeTerminal {
		label = "Terminal"
		color = lipgloss.Color("220")
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		MarginBottom(1).
		Render(fmt.Sprintf("Mode: %s", label))
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
	if job.statusText != "" {
		line2 = truncateString("  "+job.statusText, maxTextWidth)
	}

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
	terminalHeader := m.renderTerminalHeader()
	terminalHeight := m.availableLogHeight(contentHeight, metaBlock, terminalHeader)
	terminalWidth := m.viewport.Width
	if terminalWidth <= 0 {
		terminalWidth = mainWidth - mainHorizontalPadding
	}
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		metaBlock,
		terminalHeader,
		m.renderTerminalPane(job, terminalWidth, terminalHeight),
	)
	return lipgloss.NewStyle().Width(mainWidth).Height(contentHeight).Padding(0, 1).Render(body)
}

func (m *uiModel) buildMetaBlock(job *uiJob) string {
	sections := []string{m.renderMainHeader(job)}
	if fileList := strings.TrimSpace(m.renderMainFileList(job)); fileList != "" {
		sections = append(sections, fileList)
	}
	sections = append(sections, m.renderMainStatus(job), m.renderRuntime(job))
	if paths := strings.TrimSpace(m.renderLogPaths(job)); paths != "" {
		sections = append(sections, paths)
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *uiModel) renderMainHeader(job *uiJob) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		MarginBottom(1).
		Render(fmt.Sprintf("Batch: %s", job.safeName))
}

func (m *uiModel) renderMainFileList(job *uiJob) string {
	if len(job.codeFiles) == 0 {
		return ""
	}
	maxTextWidth := m.viewport.Width - 4
	if maxTextWidth < 10 {
		maxTextWidth = 10
	}
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

	fields := []string{
		fmt.Sprintf("Issues: %d", job.issues),
		fmt.Sprintf("Status: %s", statusLabel),
	}
	if job.statusText != "" {
		fields = append(fields, fmt.Sprintf("Detail: %s", job.statusText))
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("81")).
		MarginBottom(1).
		Render(strings.Join(fields, "  |  "))
}

func (m *uiModel) renderTerminalHeader() string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1).Render("Terminal:")
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
		lines = append(lines, fmt.Sprintf("  • output: %s", job.outLog))
	}
	if job.errLog != "" {
		lines = append(lines, fmt.Sprintf("  • error: %s", job.errLog))
	}
	if len(lines) == 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		MarginBottom(1).
		Render("Log Files:\n" + strings.Join(lines, "\n"))
}

func (m *uiModel) renderTerminalPane(job *uiJob, width, height int) string {
	term := m.currentTerminal()
	if term == nil {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1).
			Foreground(lipgloss.Color("245")).
			Render("Terminal not started yet.")
	}

	term.Resize(width, height)
	screen := term.Render()
	if strings.TrimSpace(screen) == "" {
		screen = "Waiting for terminal output..."
	}

	if job.state == jobSuccess && !term.IsAlive() && strings.TrimSpace(screen) == "" {
		screen = "Terminal completed."
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Render(screen)
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

func (m *uiModel) getStateLabel(state jobState) string {
	switch state {
	case jobPending:
		return "pending"
	case jobRunning:
		return "running"
	case jobSuccess:
		return "done"
	case jobFailed:
		return "failed"
	default:
		return "unknown"
	}
}
