package ui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

const timelineDetailIndent = "   "

var setTranscriptViewportContent = func(vp *viewport.Model, content string) {
	vp.SetContent(content)
}

type timelineRender struct {
	content string
	offsets []int
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
	cacheHit := m.timelineCacheHit(job, contentWidth)
	m.transcriptViewport.SetWidth(contentWidth)
	transcriptHeight := max(m.contentHeight-4, logViewportMinHeight)
	if job != nil && strings.TrimSpace(job.taskTitle) != "" {
		transcriptHeight = max(transcriptHeight-1, logViewportMinHeight)
	}
	m.transcriptViewport.SetHeight(transcriptHeight)
	rendered := m.buildTimelineContent(job, contentWidth)
	if !cacheHit || !m.timelineViewportMounted(job, contentWidth) {
		m.applyTranscriptViewportContent(rendered.content)
		m.restoreTranscriptViewport(job, rendered.offsets)
		m.timelineMounted = m.newTimelineMountState(job, contentWidth)
	}

	lines := []string{
		renderOwnedLineKnownOwned(contentWidth, colorBgSurface, m.renderTimelineHeader(job, contentWidth)),
		renderOwnedLineKnownOwned(
			contentWidth,
			colorBgSurface,
			renderStyledOnBackground(styleDimText, colorBgSurface, m.timelineMetaForWidth(job, contentWidth)),
		),
	}
	if job != nil && strings.TrimSpace(job.taskTitle) != "" {
		lines = append(lines, renderOwnedLineKnownOwned(contentWidth, colorBgSurface, ""))
	}
	lines = append(lines, renderOwnedBlock(contentWidth, colorBgSurface, m.transcriptViewport.View()))

	borderColor := colorBorder
	if m.focusedPane == uiPaneTimeline {
		borderColor = colorBorderFocus
	}
	return techPanelStyle(panelWidth, borderColor).Render(strings.Join(lines, "\n"))
}

func invalidTimelineMountState() timelineMountState {
	return timelineMountState{
		jobIndex:          -1,
		selectedEntry:     -1,
		expansionRevision: -1,
	}
}

func (m *uiModel) newTimelineMountState(job *uiJob, width int) timelineMountState {
	if job == nil {
		return invalidTimelineMountState()
	}
	return timelineMountState{
		jobIndex:          m.selectedJob,
		width:             width,
		revision:          job.snapshot.Revision,
		selectedEntry:     job.selectedEntry,
		expansionRevision: job.expansionRevision,
		valid:             true,
	}
}

func (m *uiModel) timelineViewportMounted(job *uiJob, width int) bool {
	if m == nil || job == nil || !m.timelineMounted.valid {
		return false
	}
	return m.timelineMounted.jobIndex == m.selectedJob &&
		m.timelineMounted.width == width &&
		m.timelineMounted.revision == job.snapshot.Revision &&
		m.timelineMounted.selectedEntry == job.selectedEntry &&
		m.timelineMounted.expansionRevision == job.expansionRevision
}

func (m *uiModel) timelineCacheHit(job *uiJob, width int) bool {
	return job != nil && job.timelineCacheValid &&
		job.timelineCacheWidth == width &&
		job.timelineCacheRev == job.snapshot.Revision &&
		job.timelineCacheSel == job.selectedEntry &&
		job.timelineCacheExpand == job.expansionRevision
}

func (m *uiModel) timelineMeta(job *uiJob) string {
	return m.timelineMetaForWidth(job, panelContentWidth(m.timelineWidth))
}

func (m *uiModel) timelineMetaForWidth(job *uiJob, contentWidth int) string {
	left := m.timelineEntryMeta(job)
	right := m.timelineRuntimeMeta()
	if right == "" {
		return truncateString(left, contentWidth)
	}
	rightWidth := lipgloss.Width(right)
	if rightWidth >= contentWidth {
		return truncateString(right, contentWidth)
	}
	left = truncateString(left, max(contentWidth-rightWidth-1, 0))
	padding := max(contentWidth-lipgloss.Width(left)-rightWidth, 1)
	return left + strings.Repeat(" ", padding) + right
}

func (m *uiModel) timelineEntryMeta(job *uiJob) string {
	if job == nil {
		return m.timelineAttemptMeta(nil, "No ACP transcript yet")
	}
	total := len(job.snapshot.Entries)
	if total == 0 {
		return m.timelineAttemptMeta(job, "No ACP transcript yet")
	}
	selected := job.selectedEntry + 1
	return m.timelineAttemptMeta(job, fmt.Sprintf("%d entries · selected %d/%d", total, selected, total))
}

func (m *uiModel) timelineRuntimeMeta() string {
	if m == nil {
		return ""
	}
	current := m.currentJob()
	ide := ""
	modelName := ""
	if current != nil {
		ide = strings.TrimSpace(current.ide)
		modelName = strings.TrimSpace(current.model)
	}
	if ide == "" && m.cfg != nil {
		ide = strings.TrimSpace(m.cfg.IDE)
	}
	if modelName == "" && m.cfg != nil {
		modelName = strings.TrimSpace(m.cfg.Model)
	}
	provider := strings.TrimSpace(agent.DisplayName(ide))
	switch {
	case provider != "" && modelName != "":
		return provider + " · " + modelName
	case provider != "":
		return provider
	default:
		return modelName
	}
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

func timelineHeaderLabel(job *uiJob) string {
	if job == nil || strings.TrimSpace(job.taskTitle) == "" {
		return "session.timeline"
	}
	title := strings.ToUpper(strings.TrimSpace(job.taskTitle))
	taskType := strings.TrimSpace(job.taskType)
	if taskType == "" {
		return title
	}
	return title + "  [" + taskType + "]"
}

func (m *uiModel) renderTimelineHeader(job *uiJob, contentWidth int) string {
	label := timelineHeaderLabel(job)
	if label == "session.timeline" {
		return renderTechLabel(label, colorBgSurface)
	}

	title := strings.ToUpper(strings.TrimSpace(job.taskTitle))
	taskType := strings.TrimSpace(job.taskType)
	if taskType == "" {
		return renderStyledOnBackground(styleTimelineTitle, colorBgSurface, truncateString(title, contentWidth))
	}

	badgeWidth := lipgloss.Width("[" + taskType + "]")
	titleWidth := max(contentWidth-badgeWidth-2, 1)
	title = truncateString(title, titleWidth)

	return renderStyledOnBackground(styleTimelineTitle, colorBgSurface, title) +
		renderGap(colorBgSurface, 2) +
		renderStyledOnBackground(styleMutedText, colorBgSurface, "[") +
		renderStyledOnBackground(styleTimelineBadge, colorBgSurface, taskType) +
		renderStyledOnBackground(styleMutedText, colorBgSurface, "]")
}

func (m *uiModel) retryAttemptLabel(job *uiJob) string {
	if job == nil || job.maxAttempts <= 1 || job.attempt <= 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", job.attempt, job.maxAttempts)
}

func (m *uiModel) buildTimelineContent(job *uiJob, width int) timelineRender {
	if m.timelineCacheHit(job, width) {
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
		label := toolCallStateLabel(entry.ToolCallState)
		if label == "" {
			return fmt.Sprintf("%s %s", toolCallStateIcon(entry.ToolCallState), entry.Title)
		}
		return fmt.Sprintf("%s %s [%s]", toolCallStateIcon(entry.ToolCallState), entry.Title, label)
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
		return ""
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
