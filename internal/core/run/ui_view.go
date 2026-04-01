package run

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *uiModel) renderRoot(content string) tea.View {
	v := tea.NewView(rootScreenStyle(m.width, m.height).Render(content))
	v.AltScreen = true
	return v
}

func (m *uiModel) renderSummaryView() tea.View {
	boxW := min(m.width-4, 80)
	sections := []string{m.renderSummaryMainBox(boxW)}
	if len(m.failures) > 0 {
		sections = append(sections, m.renderSummaryFailBox(boxW))
	}
	if m.aggregateUsage != nil && m.aggregateUsage.Total() > 0 {
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
		renderOwnedLine(innerW, bg, renderTechLabel("run.status", bg)),
		renderOwnedLine(innerW, bg, title),
		progress,
		renderOwnedLine(innerW, bg, ""),
	}
	for _, stat := range stats {
		lines = append(lines, renderOwnedLine(innerW, bg, stat))
	}

	return techPanelStyle(boxW, borderColor).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) renderSummaryFailBox(boxW int) string {
	bg := colorBgSurface
	lines := []string{renderOwnedLine(panelContentWidth(boxW), bg, renderTechLabel("run.failures", bg))}
	for _, f := range m.failures {
		entry := lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError).
			Background(bg).
			Render("FAIL " + f.codeFile)
		entry += renderStyledOnBackground(styleDimText, bg, fmt.Sprintf("  EXIT %d", f.exitCode))
		lines = append(lines, renderOwnedLine(panelContentWidth(boxW), bg, entry))
		if f.outLog != "" {
			lines = append(
				lines,
				renderOwnedLine(
					panelContentWidth(boxW),
					bg,
					renderStyledOnBackground(styleMutedText, bg, "  "+f.outLog),
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

	lines := []string{
		renderOwnedLine(panelContentWidth(boxW), bg, renderTechLabel("usage.tokens", bg)),
		renderOwnedLine(
			panelContentWidth(boxW),
			bg,
			renderStyledOnBackground(label, bg, "INPUT  ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.InputTokens)),
		),
		renderOwnedLine(
			panelContentWidth(boxW),
			bg,
			renderStyledOnBackground(label, bg, "OUTPUT ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.OutputTokens)),
		),
	}
	if u.CacheReadTokens > 0 {
		lines = append(
			lines,
			renderOwnedLine(
				panelContentWidth(boxW),
				bg,
				renderStyledOnBackground(label, bg, "CACHE  ")+
					renderGap(bg, 1)+
					renderStyledOnBackground(value, bg, formatNumber(u.CacheReadTokens)+" reads"),
			),
		)
	}
	totalValue := lipgloss.NewStyle().Bold(true).Foreground(colorBrand).Background(bg).Render(formatNumber(u.Total()))
	lines = append(
		lines,
		renderOwnedLine(
			panelContentWidth(boxW),
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
	return lipgloss.NewStyle().MarginTop(1).Render(renderOwnedLine(width, bg, line))
}

func (m *uiModel) View() tea.View {
	switch m.currentView {
	case uiViewSummary, uiViewFailures:
		return m.renderSummaryView()
	case uiViewJobs:
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderTitleBar(),
			m.renderHelp(),
			m.renderSeparator(),
			lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(), m.renderMainContent()),
		)
		return m.renderRoot(content)
	default:
		return tea.NewView("")
	}
}

func (m *uiModel) renderTitleBar() string {
	bg := colorBgBase
	title := renderStyledOnBackground(styleTitle, bg, "COMPOZY") +
		renderStyledOnBackground(styleTitleMeta, bg, " // AGENT LOOP")
	status := m.headerStatusText(bg)

	gap := max(m.width-lipgloss.Width(title)-lipgloss.Width(status)-2, 1)
	titleLine := renderGap(bg, 1) + title + renderGap(bg, gap) + status
	titleLine = renderOwnedLine(m.width, bg, titleLine)

	pct := 0.0
	if m.total > 0 {
		pct = float64(m.completed+m.failed) / float64(m.total)
	}
	pipelineLabel := renderTechLabel("sys.pipeline", bg)
	progressWidth := max(m.width-lipgloss.Width(pipelineLabel)-2, 10)
	m.progressBar.SetWidth(progressWidth)
	progressLine := renderGap(bg, 1) +
		pipelineLabel +
		renderGap(bg, 1) +
		renderOwnedBlock(progressWidth, bg, m.progressBar.ViewAs(pct))
	progressLine = renderOwnedLine(m.width, bg, progressLine)

	return renderOwnedLine(m.width, bg, "") + "\n" + titleLine + "\n" + progressLine
}

func (m *uiModel) headerStatusText(bg color.Color) string {
	complete := m.completed+m.failed >= m.total
	if !complete {
		if m.failed > 0 {
			return lipgloss.NewStyle().Bold(true).Foreground(colorWarning).Background(bg).Render(
				fmt.Sprintf("RUN %d/%d · %d FAIL", m.completed+m.failed, m.total, m.failed))
		}
		return renderStyledOnBackground(
			styleMutedText,
			bg,
			fmt.Sprintf("RUN %d/%d", m.completed+m.failed, m.total),
		)
	}
	if m.failed > 0 {
		return lipgloss.NewStyle().Bold(true).Foreground(colorWarning).Background(bg).Render(
			fmt.Sprintf("%d OK · %d FAIL", m.completed, m.failed))
	}
	return lipgloss.NewStyle().Bold(true).Foreground(colorSuccess).Background(bg).Render(
		fmt.Sprintf("ALL %d OK", m.total))
}

func (m *uiModel) renderHelp() string {
	bg := colorBgBase
	type kv struct{ key, desc string }

	pairs := []kv{
		{"↑↓/jk", "NAV"},
		{"pgup/pgdn", "SCROLL"},
	}
	if m.completed+m.failed >= m.total {
		pairs = append(pairs, kv{"s", "SUMMARY"})
	}
	pairs = append(pairs, kv{"q", "QUIT"})

	var parts []string
	for _, p := range pairs {
		parts = append(
			parts,
			renderKeycap(p.key, bg)+renderGap(bg, 1)+renderStyledOnBackground(styleMutedText, bg, p.desc),
		)
	}
	line := renderGap(bg, 1) + strings.Join(parts, renderGap(bg, 2))
	return renderOwnedLine(m.width, bg, line) + "\n" + renderOwnedLine(m.width, bg, "")
}

func (m *uiModel) renderSeparator() string {
	return renderOwnedLine(
		m.width,
		colorBgBase,
		renderStyledOnBackground(styleSeparator, colorBgBase, strings.Repeat("─", m.width)),
	)
}

func (m *uiModel) renderSidebar() string {
	sidebarWidth := m.sidebarWidth
	if sidebarWidth <= 0 {
		sidebarWidth = min(max(int(float64(m.width)*sidebarWidthRatio), sidebarMinWidth), sidebarMaxWidth)
	}

	var items []string
	for i := range m.jobs {
		items = append(items, m.renderSidebarItem(&m.jobs[i], i == m.selectedJob))
	}
	m.sidebarViewport.SetContent(strings.Join(items, "\n"))

	if m.selectedJob >= 0 && m.selectedJob < len(m.jobs) {
		lineOffset := m.selectedJob * 3
		sidebarOffset := m.sidebarViewport.YOffset()
		sidebarHeight := m.sidebarViewport.Height()
		if lineOffset > sidebarOffset+sidebarHeight-3 {
			m.sidebarViewport.SetYOffset(lineOffset - sidebarHeight + 3)
		} else if lineOffset < sidebarOffset {
			m.sidebarViewport.SetYOffset(lineOffset)
		}
	}

	content := renderOwnedBlock(m.sidebarViewport.Width(), colorBgSurface, m.sidebarViewport.View())
	return techSidebarStyle(sidebarWidth, colorBorder).Render(content)
}

func (m *uiModel) renderSidebarItem(job *uiJob, selected bool) string {
	bg := colorBgSurface
	statusColor := m.jobStateColor(job.state)
	icon := m.jobStateIcon(job.state)
	maxW := m.sidebarViewport.Width()

	marker := "  "
	markerRendered := renderGap(bg, lipgloss.Width(marker))
	if selected {
		marker = "▌ "
		markerRendered = lipgloss.NewStyle().Foreground(colorAccent).Background(bg).Render(marker)
	}
	iconRendered := lipgloss.NewStyle().Foreground(statusColor).Background(bg).Render(icon)

	var timeStr string
	switch job.state {
	case jobRunning:
		if !job.startedAt.IsZero() {
			timeStr = formatDuration(time.Since(job.startedAt))
		}
	case jobSuccess, jobFailed:
		if job.duration > 0 {
			timeStr = formatDuration(job.duration)
		}
	}

	leadWidth := lipgloss.Width(marker + icon + " ")
	nameWidth := maxW - leadWidth
	if timeStr != "" {
		nameWidth -= lipgloss.Width(timeStr) + 1
	}
	nameRaw := truncateString(job.safeName, max(nameWidth, 1))

	nameStyle := styleMutedText
	if selected {
		nameStyle = styleBodyText.Bold(true)
	}
	line1 := markerRendered +
		iconRendered +
		renderGap(bg, 1) +
		renderStyledOnBackground(nameStyle, bg, nameRaw)
	if timeStr != "" {
		timeStyled := lipgloss.NewStyle().Foreground(statusColor).Background(bg).Render(timeStr)
		gap := max(maxW-lipgloss.Width(line1)-lipgloss.Width(timeStyled), 1)
		line1 += renderGap(bg, gap) + timeStyled
	}
	line1 = renderOwnedLine(maxW, bg, line1)

	line2Raw := truncateString(fmt.Sprintf("    FILES %d · ISSUES %d", len(job.codeFiles), job.issues), maxW)
	metaStyle := styleDimText
	if selected {
		metaStyle = styleMutedText
	}
	line2 := renderOwnedLine(maxW, bg, renderStyledOnBackground(metaStyle, bg, line2Raw))
	row := line1 + "\n" + line2
	if selected {
		return selectedSidebarRowStyle(maxW).Render(row)
	}
	return row
}

func (m *uiModel) renderMainContent() string {
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		return lipgloss.NewStyle().
			Padding(2).
			Foreground(colorMuted).
			Background(colorBgBase).
			Render("Select a job from the sidebar")
	}

	job := &m.jobs[m.selectedJob]
	mainWidth, contentHeight := m.mainDimensions()
	bodyWidth := paddedContentWidth(mainWidth)

	metaCard := m.buildMetaCard(job, bodyWidth)
	logsHeader := m.renderLogsHeader(bodyWidth)
	metaHeight := lipgloss.Height(metaCard) + lipgloss.Height(logsHeader)

	m.viewport.SetHeight(max(contentHeight-metaHeight, logViewportMinHeight))
	m.viewport.SetWidth(bodyWidth)
	m.updateViewportForJob(job)

	logsView := renderOwnedBlock(bodyWidth, colorBgBase, m.viewport.View())
	body := lipgloss.JoinVertical(lipgloss.Left, metaCard, logsHeader, logsView)
	return lipgloss.NewStyle().
		Width(mainWidth).
		Height(contentHeight).
		Padding(0, 1).
		Background(colorBgBase).
		Render(body)
}

func (m *uiModel) buildMetaCard(job *uiJob, renderWidth int) string {
	innerW := panelContentWidth(renderWidth)
	labelStyle := styleDimText
	valueStyle := styleBodyText
	bg := colorBgSurface

	nameRaw := job.safeName
	elapsed := m.elapsedStr(job, bg)
	elapsedW := lipgloss.Width(elapsed)

	maxNameW := innerW
	if elapsed != "" {
		maxNameW = innerW - elapsedW - 1
	}
	nameRaw = truncateString(nameRaw, max(maxNameW, 1))
	nameStyled := lipgloss.NewStyle().Bold(true).Foreground(colorBrand).Background(bg).Render(nameRaw)

	titleLine := nameStyled
	if elapsed != "" {
		gap := max(innerW-lipgloss.Width(nameStyled)-elapsedW, 1)
		titleLine = nameStyled + renderGap(bg, gap) + elapsed
	}
	titleLine = renderOwnedLine(innerW, bg, titleLine)

	lines := []string{
		renderOwnedLine(innerW, bg, renderTechLabel("run.status", bg)),
		titleLine,
	}

	if len(job.codeFiles) > 0 {
		label := renderStyledOnBackground(labelStyle, bg, "FILES  ")
		maxLen := innerW - lipgloss.Width(label)
		files := truncateString(strings.Join(job.codeFiles, ", "), max(maxLen, 1))
		lines = append(
			lines,
			renderOwnedLine(
				innerW,
				bg,
				label+lipgloss.NewStyle().Foreground(colorAccentAlt).Background(bg).Render(files),
			),
		)
	}

	statusVal := m.getStateLabel(job.state)
	if job.state == jobFailed && job.exitCode != 0 {
		statusVal = fmt.Sprintf("FAILED (EXIT %d)", job.exitCode)
	}
	statusLine := renderStyledOnBackground(labelStyle, bg, "STATUS ") +
		lipgloss.NewStyle().Bold(true).Foreground(m.jobStateColor(job.state)).Background(bg).Render(statusVal) +
		renderStyledOnBackground(styleDimText, bg, "  ·  ") +
		renderStyledOnBackground(labelStyle, bg, "ISSUES ") +
		renderStyledOnBackground(valueStyle.Bold(true), bg, fmt.Sprintf("%d", job.issues))
	lines = append(lines, renderOwnedLine(innerW, bg, statusLine))

	if job.tokenUsage != nil && job.tokenUsage.Total() > 0 {
		u := job.tokenUsage
		tokenLine := renderStyledOnBackground(labelStyle, bg, "TOKENS ") +
			lipgloss.NewStyle().Foreground(colorAccentAlt).Background(bg).Render(
				fmt.Sprintf("%s IN / %s OUT", formatNumber(u.InputTokens), formatNumber(u.OutputTokens)))
		lines = append(lines, renderOwnedLine(innerW, bg, tokenLine))
	}

	return techPanelStyle(renderWidth, m.jobBorderColor(job)).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) elapsedStr(job *uiJob, bg color.Color) string {
	switch job.state {
	case jobRunning:
		if !job.startedAt.IsZero() {
			return renderStyledOnBackground(styleDimText, bg, formatDuration(time.Since(job.startedAt)))
		}
	case jobSuccess:
		d := job.duration
		if d == 0 && !job.startedAt.IsZero() {
			d = time.Since(job.startedAt)
		}
		if d > 0 {
			return lipgloss.NewStyle().Foreground(colorSuccess).Background(bg).Render("OK " + formatDuration(d))
		}
	case jobFailed:
		d := job.duration
		if d == 0 && !job.startedAt.IsZero() {
			d = time.Since(job.startedAt)
		}
		if d > 0 {
			return lipgloss.NewStyle().Foreground(colorError).Background(bg).Render("FAIL " + formatDuration(d))
		}
	}
	return ""
}

func (m *uiModel) renderLogsHeader(width int) string {
	bg := colorBgBase
	title := renderStyledOnBackground(styleLogHeader, bg, "SYS.STDOUT // LIVE LOGS")
	rightLen := max(width-lipgloss.Width(title)-1, 0)
	if rightLen == 0 {
		return renderOwnedLine(width, bg, title)
	}
	return renderOwnedLine(
		width,
		bg,
		title+
			renderGap(bg, 1)+
			renderStyledOnBackground(styleSeparator, bg, strings.Repeat("─", rightLen)),
	)
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
	maxW := m.viewport.Width()
	var content strings.Builder
	if len(job.lastOut) > 0 {
		for _, line := range job.lastOut {
			if line != "" {
				content.WriteString(truncateString(line, maxW))
				content.WriteString("\n")
			}
		}
	}
	if len(job.lastErr) > 0 {
		content.WriteString(styleStderrLabel.Render("[STDERR]"))
		content.WriteString("\n")
		for _, line := range job.lastErr {
			if line != "" {
				content.WriteString(truncateString(line, maxW))
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
		return "PENDING"
	case jobRunning:
		return "RUNNING"
	case jobSuccess:
		return "SUCCESS"
	case jobFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

func (m *uiModel) jobStateIcon(state jobState) string {
	switch state {
	case jobPending:
		return "⏸"
	case jobRunning:
		return spinnerFrames[m.frame%len(spinnerFrames)]
	case jobSuccess:
		return "✓"
	case jobFailed:
		return "✗"
	default:
		return "•"
	}
}

func (m *uiModel) jobStateColor(state jobState) color.Color {
	switch state {
	case jobPending:
		return colorMuted
	case jobRunning:
		return colorAccentAlt
	case jobSuccess:
		return colorSuccess
	case jobFailed:
		return colorError
	default:
		return colorInfo
	}
}

func (m *uiModel) jobBorderColor(job *uiJob) color.Color {
	switch job.state {
	case jobRunning:
		return colorBorderFocus
	case jobSuccess:
		return colorSuccess
	case jobFailed:
		return colorError
	default:
		return colorBorder
	}
}
