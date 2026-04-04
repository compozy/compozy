package run

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const timelineDetailIndent = "   "

type timelineRender struct {
	content string
	offsets []int
}

func (m *uiModel) renderRoot(content string) tea.View {
	v := tea.NewView(rootScreenStyle(m.width, m.height).Render(content))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *uiModel) View() tea.View {
	switch m.currentView {
	case uiViewSummary, uiViewFailures:
		return m.renderSummaryView()
	case uiViewJobs:
		body := m.renderJobsBody()
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderTitleBar(),
			m.renderSeparator(),
			body,
			m.renderHelp(),
		)
		return m.renderRoot(content)
	default:
		return tea.NewView("")
	}
}

func (m *uiModel) renderJobsBody() string {
	if m.layoutMode == uiLayoutResizeBlocked {
		return m.renderResizeGate()
	}

	sidebar := m.renderSidebar()
	main := m.renderMainPanels()
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
}

func (m *uiModel) renderResizeGate() string {
	message := []string{
		renderOwnedLine(m.width-4, colorBgSurface, renderTechLabel("ui.resize", colorBgSurface)),
		renderOwnedLine(m.width-4, colorBgSurface, "ACP cockpit needs at least 80x24."),
		renderOwnedLine(m.width-4, colorBgSurface, fmt.Sprintf("Current size: %dx%d", m.width, m.height)),
	}
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.contentHeight).
		Padding(1, 1).
		Background(colorBgBase).
		Render(techPanelStyle(max(m.width-2, 10), colorWarning).Render(strings.Join(message, "\n")))
}

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
	innerW := panelContentWidth(boxW)

	lines := []string{
		renderOwnedLine(innerW, bg, renderTechLabel("usage.tokens", bg)),
		renderOwnedLine(
			innerW,
			bg,
			renderStyledOnBackground(label, bg, "INPUT  ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.InputTokens)),
		),
		renderOwnedLine(
			innerW,
			bg,
			renderStyledOnBackground(label, bg, "OUTPUT ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.OutputTokens)),
		),
		renderOwnedLine(
			innerW,
			bg,
			renderStyledOnBackground(label, bg, "CACHER ")+
				renderGap(bg, 1)+
				renderStyledOnBackground(value, bg, formatNumber(u.CacheReads)),
		),
		renderOwnedLine(
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
		renderOwnedLine(
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
	return lipgloss.NewStyle().MarginTop(1).Render(renderOwnedLine(width, bg, line))
}

func (m *uiModel) elapsedStr(job *uiJob, bg color.Color) string {
	switch job.state {
	case jobRunning:
		if !job.startedAt.IsZero() {
			return renderStyledOnBackground(styleDimText, bg, formatDuration(time.Since(job.startedAt)))
		}
	case jobRetrying:
		if label := m.retryAttemptLabel(job); label != "" {
			return lipgloss.NewStyle().Foreground(colorWarning).Background(bg).Render("RETRY " + label)
		}
		return lipgloss.NewStyle().Foreground(colorWarning).Background(bg).Render("RETRY")
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

func (m *uiModel) renderTitleBar() string {
	bg := colorBgBase
	title := renderStyledOnBackground(styleTitle, bg, "COMPOZY") +
		renderStyledOnBackground(styleTitleMeta, bg, " // ACP COCKPIT")
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
		if m.shutdown.active() {
			return lipgloss.NewStyle().Bold(true).Foreground(colorWarning).Background(bg).Render(
				m.shutdownHeaderLabel(),
			)
		}
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

func (m *uiModel) shutdownHeaderLabel() string {
	progress := fmt.Sprintf("%d/%d", m.completed+m.failed, m.total)
	switch m.shutdown.Phase {
	case shutdownPhaseDraining:
		countdown := m.shutdownCountdownLabel()
		if countdown == "" {
			return "DRAINING " + progress
		}
		return fmt.Sprintf("DRAINING %s · %s", progress, countdown)
	case shutdownPhaseForcing:
		return "FORCING " + progress
	default:
		return "RUN " + progress
	}
}

func (m *uiModel) shutdownCountdownLabel() string {
	if m.shutdown.DeadlineAt.IsZero() {
		return ""
	}
	remaining := time.Until(m.shutdown.DeadlineAt)
	if remaining < 0 {
		remaining = 0
	}
	return remaining.Round(100 * time.Millisecond).String()
}

func (m *uiModel) renderSeparator() string {
	return renderOwnedLine(
		m.width,
		colorBgBase,
		renderStyledOnBackground(styleSeparator, colorBgBase, strings.Repeat("─", m.width)),
	)
}

func (m *uiModel) renderHelp() string {
	bg := colorBgBase
	paneLabel := strings.ToUpper(string(m.focusedPane))
	pairs := []string{}

	switch m.focusedPane {
	case uiPaneJobs:
		pairs = append(pairs,
			renderKeycap("↑↓/jk", bg)+renderGap(bg, 1)+renderStyledOnBackground(styleMutedText, bg, "JOB"),
			renderKeycap("tab", bg)+renderGap(bg, 1)+renderStyledOnBackground(styleMutedText, bg, "FOCUS"),
		)
	case uiPaneTimeline:
		pairs = append(pairs,
			renderKeycap("↑↓/jk", bg)+renderGap(bg, 1)+renderStyledOnBackground(styleMutedText, bg, "ENTRY"),
			renderKeycap("enter", bg)+renderGap(bg, 1)+renderStyledOnBackground(styleMutedText, bg, "EXPAND"),
			renderKeycap("pg/home/end", bg)+renderGap(bg, 1)+renderStyledOnBackground(styleMutedText, bg, "SCROLL"),
		)
	}
	if m.isRunComplete() {
		pairs = append(
			pairs,
			renderKeycap("s", bg)+renderGap(bg, 1)+renderStyledOnBackground(styleMutedText, bg, "SUMMARY"),
		)
	}
	quitLabel := "QUIT"
	switch m.shutdown.Phase {
	case shutdownPhaseDraining:
		quitLabel = "FORCE QUIT"
	case shutdownPhaseForcing:
		quitLabel = "FORCING"
	}
	pairs = append(
		pairs,
		renderKeycap("q", bg)+renderGap(bg, 1)+renderStyledOnBackground(styleMutedText, bg, quitLabel),
	)

	label := renderStyledOnBackground(styleDimText, bg, "FOCUS "+paneLabel)
	line := renderGap(bg, 1) + label + renderGap(bg, 2) + strings.Join(pairs, renderGap(bg, 2))
	return renderOwnedLine(m.width, bg, line) + "\n" + renderOwnedLine(m.width, bg, "")
}

func (m *uiModel) renderSidebar() string {
	borderColor := colorBorder
	if m.focusedPane == uiPaneJobs {
		borderColor = colorBorderFocus
	}
	content := renderOwnedBlock(m.sidebarViewport.Width(), colorBgSurface, m.sidebarViewport.View())
	return techSidebarStyle(m.sidebarWidth, borderColor).Render(content)
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
	case jobRetrying:
		timeStr = m.retryAttemptLabel(job)
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

	line2Raw := truncateString(m.sidebarMeta(job), maxW)
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

func (m *uiModel) renderMainPanels() string {
	job := m.currentJob()
	if job == nil {
		return lipgloss.NewStyle().Width(max(m.width-m.sidebarWidth, 1)).Render("")
	}

	return m.renderTimelinePanel(job, m.timelineWidth)
}

func (m *uiModel) renderTimelinePanel(job *uiJob, panelWidth int) string {
	contentWidth := panelContentWidth(panelWidth)
	m.transcriptViewport.SetWidth(contentWidth)
	m.transcriptViewport.SetHeight(max(m.contentHeight-4, logViewportMinHeight))
	rendered := m.buildTimelineContent(job, contentWidth)
	m.transcriptViewport.SetContent(rendered.content)
	m.restoreTranscriptViewport(job, rendered.offsets)

	lines := []string{
		renderOwnedLine(contentWidth, colorBgSurface, renderTechLabel("session.timeline", colorBgSurface)),
		renderOwnedLine(
			contentWidth,
			colorBgSurface,
			renderStyledOnBackground(styleDimText, colorBgSurface, m.timelineMeta(job)),
		),
		renderOwnedBlock(contentWidth, colorBgSurface, m.transcriptViewport.View()),
	}

	borderColor := colorBorder
	if m.focusedPane == uiPaneTimeline {
		borderColor = colorBorderFocus
	}
	return techPanelStyle(panelWidth, borderColor).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) timelineMeta(job *uiJob) string {
	total := len(job.snapshot.Entries)
	if total == 0 {
		return m.timelineAttemptMeta(job, "No ACP transcript yet")
	}
	selected := job.selectedEntry + 1
	return m.timelineAttemptMeta(job, fmt.Sprintf("%d entries · selected %d/%d", total, selected, total))
}

func (m *uiModel) timelineAttemptMeta(job *uiJob, base string) string {
	parts := []string{base}
	if attemptLabel := m.retryAttemptLabel(job); attemptLabel != "" {
		parts = append(parts, "attempt "+attemptLabel)
	}
	if job != nil && job.retrying && strings.TrimSpace(job.retryReason) != "" {
		parts = append(parts, "retrying: "+truncateString(job.retryReason, 72))
	}
	return strings.Join(parts, " · ")
}

func (m *uiModel) retryAttemptLabel(job *uiJob) string {
	if job == nil || job.maxAttempts <= 1 || job.attempt <= 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", job.attempt, job.maxAttempts)
}

func (m *uiModel) sidebarMeta(job *uiJob) string {
	parts := make([]string, 0, 4)
	if label := m.getStateLabel(job.state); label != "" {
		parts = append(parts, label)
	}
	if attempt := m.retryAttemptLabel(job); attempt != "" {
		parts = append(parts, "ATTEMPT "+attempt)
	}
	parts = append(parts,
		fmt.Sprintf("FILES %d", len(job.codeFiles)),
		fmt.Sprintf("ISSUES %d", job.issues),
	)
	return "    " + strings.Join(parts, " · ")
}

func (m *uiModel) buildTimelineContent(job *uiJob, width int) timelineRender {
	if job != nil && job.timelineCacheValid &&
		job.timelineCacheWidth == width &&
		job.timelineCacheRev == job.snapshot.Revision &&
		job.timelineCacheSel == job.selectedEntry &&
		job.timelineCacheExpand == job.expansionRevision {
		return job.timelineCache
	}

	if len(job.snapshot.Entries) == 0 {
		rendered := timelineRender{
			content: renderStyledOwnedLine(
				width,
				styleMutedText,
				colorBgSurface,
				"Waiting for ACP updates...",
			),
		}
		job.timelineCache = rendered
		job.timelineCacheWidth = width
		job.timelineCacheRev = job.snapshot.Revision
		job.timelineCacheSel = job.selectedEntry
		job.timelineCacheExpand = job.expansionRevision
		job.timelineCacheValid = true
		return rendered
	}

	var lines []string
	offsets := make([]int, 0, len(job.snapshot.Entries))
	for idx := range job.snapshot.Entries {
		entry := job.snapshot.Entries[idx]
		offsets = append(offsets, len(lines))
		entryLines := m.renderTimelineEntry(job, entry, idx, width)
		lines = append(lines, entryLines...)
		if idx < len(job.snapshot.Entries)-1 {
			lines = append(lines, renderOwnedLine(width, colorBgSurface, ""))
		}
	}
	rendered := timelineRender{
		content: strings.Join(lines, "\n"),
		offsets: offsets,
	}
	job.timelineCache = rendered
	job.timelineCacheWidth = width
	job.timelineCacheRev = job.snapshot.Revision
	job.timelineCacheSel = job.selectedEntry
	job.timelineCacheExpand = job.expansionRevision
	job.timelineCacheValid = true
	return rendered
}

func (m *uiModel) renderTimelineEntry(job *uiJob, entry TranscriptEntry, index int, width int) []string {
	selected := index == job.selectedEntry
	bg := colorBgSurface
	marker := "  "
	if selected {
		marker = "▌ "
	}

	title := m.timelineEntryTitle(entry)
	headerStyle := m.timelineEntryHeaderStyle(entry, selected)
	line := renderStyledOwnedLine(width, headerStyle, bg, truncateString(marker+title, width))
	lines := []string{line}

	preview := entry.Preview
	if preview != "" && m.shouldRenderEntryPreview(job, entry) {
		lines = append(
			lines,
			renderStyledOwnedLine(width, styleDimText, bg, truncateString(timelineDetailIndent+preview, width)),
		)
	}

	if m.isEntryExpanded(job, entry) {
		for _, detail := range m.renderEntryDetailLines(entry, width) {
			lines = append(lines, renderStyledOwnedLine(width, styleBodyText, bg, truncateString(detail, width)))
		}
	}

	return lines
}

func (m *uiModel) timelineEntryTitle(entry TranscriptEntry) string {
	switch entry.Kind {
	case transcriptEntryToolCall:
		return fmt.Sprintf(
			"%s %s [%s]",
			toolCallStateIcon(entry.ToolCallState),
			entry.Title,
			toolCallStateLabel(entry.ToolCallState),
		)
	case transcriptEntryAssistantThinking:
		return "Thinking"
	default:
		return entry.Title
	}
}

func (m *uiModel) timelineEntryHeaderStyle(entry TranscriptEntry, selected bool) lipgloss.Style {
	style := styleMutedText
	switch entry.Kind {
	case transcriptEntryAssistantMessage:
		style = styleBodyText
	case transcriptEntryAssistantThinking:
		style = styleDimText.Foreground(colorAccentAlt)
	case transcriptEntryToolCall:
		style = styleBodyText.Foreground(toolCallStateColor(entry.ToolCallState))
	case transcriptEntryRuntimeNotice:
		style = styleBodyText.Foreground(colorInfo)
	case transcriptEntryStderrEvent:
		style = styleBodyText.Foreground(colorError)
	}
	if selected {
		style = style.Bold(true)
	}
	return style
}

func (m *uiModel) shouldRenderEntryPreview(job *uiJob, entry TranscriptEntry) bool {
	if !m.isEntryExpanded(job, entry) {
		return true
	}
	return !isNarrativeEntryKind(entry.Kind)
}

func (m *uiModel) renderEntryDetailLines(entry TranscriptEntry, width int) []string {
	contentWidth := max(width-len(timelineDetailIndent), 1)
	var lines []string
	if isNarrativeEntryKind(entry.Kind) {
		lines = m.renderWrappedBlocksLines(entry.Blocks, contentWidth)
	} else {
		lines = m.renderBlocksLines(entry.Blocks, contentWidth)
	}
	if len(lines) == 0 {
		return nil
	}

	prefixed := make([]string, 0, len(lines))
	for _, line := range lines {
		prefixed = append(prefixed, timelineDetailIndent+line)
	}
	return prefixed
}

func (m *uiModel) renderBlocksLines(blocks []model.ContentBlock, width int) []string {
	if len(blocks) == 0 {
		return nil
	}
	outLines, errLines := renderContentBlocks(blocks)
	lines := make([]string, 0, len(outLines)+len(errLines)+1)
	lines = append(lines, truncateViewportLines(outLines, width)...)
	if len(errLines) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, truncateViewportLines(errLines, width)...)
	}
	return lines
}

func (m *uiModel) renderWrappedBlocksLines(blocks []model.ContentBlock, width int) []string {
	if len(blocks) == 0 {
		return nil
	}
	outLines, errLines := renderContentBlocks(blocks)
	lines := make([]string, 0, len(outLines)+len(errLines)+1)
	lines = append(lines, wrapViewportLines(outLines, width)...)
	if len(errLines) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, wrapViewportLines(errLines, width)...)
	}
	return lines
}

func (m *uiModel) restoreTranscriptViewport(job *uiJob, offsets []int) {
	if len(offsets) == 0 {
		m.transcriptViewport.GotoTop()
		job.transcriptYOffset = 0
		job.transcriptXOffset = 0
		job.transcriptFollowTail = true
		return
	}
	if job.transcriptFollowTail {
		m.transcriptViewport.GotoBottom()
	} else {
		m.transcriptViewport.SetYOffset(job.transcriptYOffset)
		m.transcriptViewport.SetXOffset(job.transcriptXOffset)
	}

	selectedLine := offsets[job.selectedEntry]
	yOffset := m.transcriptViewport.YOffset()
	height := max(m.transcriptViewport.Height(), 1)
	if selectedLine < yOffset {
		m.transcriptViewport.SetYOffset(selectedLine)
	} else if selectedLine >= yOffset+height {
		m.transcriptViewport.SetYOffset(max(selectedLine-height+1, 0))
	}

	job.transcriptYOffset = m.transcriptViewport.YOffset()
	job.transcriptXOffset = m.transcriptViewport.XOffset()
	job.transcriptFollowTail = m.transcriptViewport.AtBottom()
}

func toolCallStateLabel(state model.ToolCallState) string {
	switch state {
	case model.ToolCallStatePending:
		return "PENDING"
	case model.ToolCallStateInProgress:
		return "RUNNING"
	case model.ToolCallStateCompleted:
		return "COMPLETED"
	case model.ToolCallStateFailed:
		return "FAILED"
	case model.ToolCallStateWaitingForConfirmation:
		return "CONFIRM"
	default:
		return "READY"
	}
}

func toolCallStateIcon(state model.ToolCallState) string {
	switch state {
	case model.ToolCallStatePending:
		return "○"
	case model.ToolCallStateInProgress:
		return "●"
	case model.ToolCallStateCompleted:
		return "✓"
	case model.ToolCallStateFailed:
		return "✗"
	case model.ToolCallStateWaitingForConfirmation:
		return "!"
	default:
		return "•"
	}
}

func toolCallStateColor(state model.ToolCallState) color.Color {
	switch state {
	case model.ToolCallStatePending:
		return colorAccentAlt
	case model.ToolCallStateInProgress:
		return colorBrand
	case model.ToolCallStateCompleted:
		return colorSuccess
	case model.ToolCallStateFailed:
		return colorError
	case model.ToolCallStateWaitingForConfirmation:
		return colorWarning
	default:
		return colorInfo
	}
}

func (m *uiModel) isEntryExpanded(job *uiJob, entry TranscriptEntry) bool {
	if job == nil {
		return false
	}
	if job.expandedEntryIDs != nil {
		if expanded, ok := job.expandedEntryIDs[entry.ID]; ok {
			return expanded
		}
	}
	switch entry.Kind {
	case transcriptEntryAssistantMessage, transcriptEntryRuntimeNotice, transcriptEntryStderrEvent:
		return true
	case transcriptEntryToolCall:
		switch entry.ToolCallState {
		case model.ToolCallStateFailed, model.ToolCallStateWaitingForConfirmation:
			return true
		}
	}
	return false
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

func truncateViewportLines(lines []string, maxW int) []string {
	if len(lines) == 0 {
		return nil
	}
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, truncateString(line, maxW))
	}
	return rendered
}

func wrapViewportLines(lines []string, maxW int) []string {
	if len(lines) == 0 {
		return nil
	}
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			rendered = append(rendered, "")
			continue
		}
		rendered = append(rendered, splitRenderedText(lipgloss.Wrap(line, maxW, ""))...)
	}
	return rendered
}

func isNarrativeEntryKind(kind transcriptEntryKind) bool {
	switch kind {
	case transcriptEntryAssistantMessage,
		transcriptEntryAssistantThinking,
		transcriptEntryRuntimeNotice,
		transcriptEntryStderrEvent:
		return true
	default:
		return false
	}
}

func (m *uiModel) getStateLabel(state jobState) string {
	switch state {
	case jobPending:
		return "PENDING"
	case jobRunning:
		return "RUNNING"
	case jobRetrying:
		return "RETRY"
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
	case jobRetrying:
		return "↻"
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
	case jobRetrying:
		return colorWarning
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
	case jobRetrying:
		return colorWarning
	case jobSuccess:
		return colorSuccess
	case jobFailed:
		return colorError
	default:
		return colorBorder
	}
}
