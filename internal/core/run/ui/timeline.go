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

const (
	timelineDetailIndent     = "   "
	composerPromptGlyph      = "〉 "
	composerPausedTaskPrompt = "Message paused task"
	// timelineChromeRows is the vertical budget consumed around the transcript:
	// header box (2 content + 2 border), composer box (1 content + 2 border),
	// and the messages box border.
	timelineChromeRows = 9
)

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

// renderTimelinePanel renders the page content as a vertical stack of boxes:
// header, streaming messages, an actionable INTEGRATION pane, and the composer
// textbox. Routine parallel progress stays out of the transcript stack.
func (m *uiModel) renderTimelinePanel(job *uiJob, panelWidth int) string {
	contentWidth := panelContentWidth(panelWidth)
	integrationBox := m.renderIntegrationBox(panelWidth, contentWidth)
	integrationRows := 0
	if integrationBox != "" {
		integrationRows = lipgloss.Height(integrationBox)
	}
	cacheHit := m.timelineCacheHit(job, contentWidth)
	m.transcriptViewport.SetWidth(contentWidth)
	m.transcriptViewport.SetHeight(max(m.contentHeight-timelineChromeRows-integrationRows, logViewportMinHeight))
	rendered := m.buildTimelineContent(job, contentWidth)
	if !cacheHit || !m.timelineViewportMounted(job, contentWidth) {
		m.applyTranscriptViewportContent(rendered.content)
		m.restoreTranscriptViewport(job, rendered.offsets)
		m.timelineMounted = m.newTimelineMountState(job, contentWidth)
	}
	headerBox := techPanelStyle(panelWidth, colorBorder).
		Render(m.renderTimelineHeaderContent(job, contentWidth))
	messagesBox := techPanelStyle(panelWidth, m.timelineMessagesBorderColor()).
		Render(m.renderTimelineMessagesContent(contentWidth))
	composerBox := techPanelStyle(panelWidth, m.composerBorderColor(job)).
		Render(m.renderComposerContent(job, contentWidth))
	boxes := make([]string, 0, 4)
	boxes = append(boxes, headerBox, messagesBox)
	if integrationBox != "" {
		boxes = append(boxes, integrationBox)
	}
	boxes = append(boxes, composerBox)
	return lipgloss.JoinVertical(lipgloss.Left, boxes...)
}

// integrationBoxChromeRows is the vertical frame (top + bottom border) the
// INTEGRATION pane box adds around its content.
const integrationBoxChromeRows = 2

// renderIntegrationBox renders the persistent INTEGRATION pane as a bordered box,
// capping its content height so the transcript viewport keeps at least
// logViewportMinHeight rows. Returns "" for non-parallel runs.
func (m *uiModel) renderIntegrationBox(panelWidth, contentWidth int) string {
	content := m.renderIntegrationContent(contentWidth)
	if content == "" {
		return ""
	}
	maxRows := m.contentHeight - timelineChromeRows - logViewportMinHeight - integrationBoxChromeRows
	if maxRows < 1 {
		return ""
	}
	content = capLines(content, maxRows)
	return techPanelStyle(panelWidth, m.integrationBorderColor()).Render(content)
}

// capLines keeps the first maxRows lines of content, preserving the highest-signal
// rows (status line, banner, conflicted files) and dropping trailing resolver
// transcript lines first.
func capLines(content string, maxRows int) string {
	if maxRows < 1 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= maxRows {
		return content
	}
	return strings.Join(lines[:maxRows], "\n")
}

func (m *uiModel) renderTimelineHeaderContent(job *uiJob, contentWidth int) string {
	lines := []string{
		renderOwnedLineKnownOwned(contentWidth, m.renderTimelineHeader(job, contentWidth)),
		renderOwnedLineKnownOwned(
			contentWidth,
			styleDimText.Render(m.timelineMetaForWidth(job, contentWidth)),
		),
	}
	return strings.Join(lines, "\n")
}

func (m *uiModel) renderTimelineMessagesContent(contentWidth int) string {
	return renderOwnedBlock(contentWidth, m.transcriptViewport.View())
}

func (m *uiModel) timelineMessagesBorderColor() color.Color {
	borderColor := colorBorder
	if m.focusedPane == uiPaneTimeline {
		borderColor = colorBorderFocus
	}
	return borderColor
}

func (m *uiModel) renderComposerContent(job *uiJob, innerWidth int) string {
	enabled := m.composerEnabled(job)
	m.configureComposerAppearance()
	m.composer.SetWidth(innerWidth)
	m.composer.SetHeight(1)
	if enabled {
		m.composer.Placeholder = composerPausedTaskPrompt
		return renderOwnedLineKnownOwned(innerWidth, m.composer.View())
	}
	label := m.composerDisabledLabel(job)
	if m.composerBusy {
		label = "Sending message..."
	}
	if strings.TrimSpace(m.composerError) != "" {
		return renderOwnedLineKnownOwned(
			innerWidth,
			m.renderComposerPromptedText(m.composerError, innerWidth, styleBodyText.Foreground(colorError)),
		)
	}
	return renderOwnedLineKnownOwned(
		innerWidth,
		m.renderComposerPromptedText(label, innerWidth, styleDimText),
	)
}

func (m *uiModel) composerBorderColor(job *uiJob) color.Color {
	if strings.TrimSpace(m.composerError) != "" {
		return colorError
	}
	if m.focusedPane == uiPaneComposer && m.composerEnabled(job) {
		return colorBorderFocus
	}
	return colorBorder
}

func (m *uiModel) renderComposerPromptedText(label string, width int, labelStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	promptWidth := lipgloss.Width(composerPromptGlyph)
	if width <= promptWidth {
		return styleDimText.Render(truncateString(composerPromptGlyph, width))
	}
	return styleDimText.Render(composerPromptGlyph) +
		labelStyle.Render(truncateString(label, width-promptWidth))
}

func (m *uiModel) composerDisabledLabel(job *uiJob) string {
	if job == nil {
		return "No task selected"
	}
	switch job.state {
	case jobPaused:
		return composerPausedTaskPrompt
	case jobPausing:
		return "Pausing task..."
	case jobRunning, jobRetrying:
		return "Task running"
	case jobSuccess:
		return "Task completed"
	case jobFailed:
		return "Task failed"
	default:
		return "Task pending"
	}
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

// timelineRuntimeMeta renders the right-hand runtime strip:
//
//	Codex · gpt-5.5 · xhigh · 12.3k tok
//
// Provider/model fall back to the run config; reasoning effort and the ACP token
// total are appended only when present. The string is plain (no ANSI) so the
// caller can width-truncate it safely.
func (m *uiModel) timelineRuntimeMeta() string {
	if m == nil {
		return ""
	}
	current := m.currentJob()
	ide, modelName, reasoning := "", "", ""
	var usage *model.Usage
	if current != nil {
		ide = strings.TrimSpace(current.ide)
		modelName = strings.TrimSpace(current.model)
		reasoning = strings.TrimSpace(current.reasoningEffort)
		usage = current.tokenUsage
	}
	if ide == "" && m.cfg != nil {
		ide = strings.TrimSpace(m.cfg.IDE)
	}
	if modelName == "" && m.cfg != nil {
		modelName = strings.TrimSpace(m.cfg.Model)
	}
	parts := make([]string, 0, 4)
	if provider := strings.TrimSpace(agent.DisplayName(ide)); provider != "" {
		parts = append(parts, provider)
	}
	if modelName != "" {
		parts = append(parts, modelName)
	}
	if reasoning != "" {
		parts = append(parts, reasoning)
	}
	if total := formatUsageTotalLabel(usage); total != "" {
		parts = append(parts, total)
	}
	return strings.Join(parts, " · ")
}

// formatUsageTotalLabel renders the provider token total, returning "" until
// usage has accrued. model.Usage.Total prefers provider total_tokens when set,
// including cache token classes that are not visible in input/output splits.
func formatUsageTotalLabel(usage *model.Usage) string {
	if usage == nil {
		return ""
	}
	total := usage.Total()
	if total <= 0 {
		return ""
	}
	return formatTokens(total) + " tok"
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
		return renderTechLabel(label)
	}

	title := strings.ToUpper(strings.TrimSpace(job.taskTitle))
	taskType := strings.TrimSpace(job.taskType)
	if taskType == "" {
		return styleTimelineTitle.Render(truncateString(title, contentWidth))
	}

	badgeWidth := lipgloss.Width("[" + taskType + "]")
	titleWidth := max(contentWidth-badgeWidth-2, 1)
	title = truncateString(title, titleWidth)

	return styleTimelineTitle.Render(title) +
		renderGap(2) +
		styleMutedText.Render("[") +
		styleTimelineBadge.Render(taskType) +
		styleMutedText.Render("]")
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
			content: renderOwnedLineKnownOwned(
				width,
				styleMutedText.Render("Waiting for ACP updates..."),
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
			lines = append(lines, renderOwnedLineKnownOwned(width, ""))
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
	marker := "  "
	if selected {
		marker = "▌ "
	}

	title := m.timelineEntryTitle(entry)
	headerStyle := m.timelineEntryHeaderStyle(entry, selected)
	line := renderOwnedLineKnownOwned(width, headerStyle.Render(truncateString(marker+title, width)))
	lines := []string{line}

	preview := entry.Preview
	if preview != "" && m.shouldRenderEntryPreview(job, entry) {
		lines = append(
			lines,
			renderOwnedLineKnownOwned(width, styleDimText.Render(truncateString(timelineDetailIndent+preview, width))),
		)
	}

	if m.isEntryExpanded(job, entry) {
		for _, detail := range m.renderEntryDetailLines(entry, width) {
			lines = append(lines, renderOwnedLineKnownOwned(width, styleBodyText.Render(truncateString(detail, width))))
		}
	}

	return lines
}

func (m *uiModel) timelineEntryTitle(entry TranscriptEntry) string {
	switch entry.Kind {
	case transcriptEntryUserMessage:
		return "You"
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
	case transcriptEntryUserMessage:
		style = styleBodyText.Foreground(colorAccentDeep)
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
		return statusLabelRunning
	case model.ToolCallStateCompleted:
		return ""
	case model.ToolCallStateFailed:
		return statusLabelFailed
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
	case transcriptEntryUserMessage,
		transcriptEntryAssistantMessage,
		transcriptEntryRuntimeNotice,
		transcriptEntryStderrEvent:
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
	case transcriptEntryUserMessage,
		transcriptEntryAssistantMessage,
		transcriptEntryAssistantThinking,
		transcriptEntryRuntimeNotice,
		transcriptEntryStderrEvent:
		return true
	default:
		return false
	}
}
