package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// renderSidebar draws the JOBS panel: a title followed by a stack of bordered job
// cards. The panel border stays muted regardless of focus so the only accent in
// the panel is the selected card. The panel carries no inner horizontal padding,
// so the cards sit close to the border (their own border is the inset).
func (m *uiModel) renderSidebar() string {
	width := m.sidebarViewport.Width()
	title := m.renderSidebarTitle(width)
	body := renderOwnedBlock(width, m.sidebarViewport.View())
	return techSidebarStyle(m.sidebarWidth, colorBorder).Render(title + "\n" + body)
}

// renderSidebarTitle draws the JOB status line and progress meter, mirroring the
// wireframe where run progress belongs to the task list rather than global chrome.
// Occupies sidebarHeaderRows rows.
func (m *uiModel) renderSidebarTitle(width int) string {
	statusLine := m.renderSidebarStatusLine(width)
	pct := 0.0
	if m.total > 0 {
		pct = float64(m.settledJobs()) / float64(m.total)
	}
	m.progressBar.SetWidth(max(width, 1))
	progressLine := renderOwnedLineKnownOwned(width, renderOwnedBlock(width, m.progressBar.ViewAs(pct)))
	return statusLine + "\n" + progressLine
}

func (m *uiModel) renderSidebarStatusLine(width int) string {
	leftRaw := m.sidebarStatusTextRaw()
	rightRaw := formatUsageTotalLabel(m.aggregateUsage)
	if rightRaw != "" && lipgloss.Width(leftRaw)+1+lipgloss.Width(rightRaw) <= width {
		leftLimit := max(width-lipgloss.Width(rightRaw)-1, 1)
		leftRaw = truncateString(leftRaw, leftLimit)
		gap := max(width-lipgloss.Width(leftRaw)-lipgloss.Width(rightRaw), 1)
		return renderOwnedLineKnownOwned(
			width,
			m.renderSidebarStatusLeft(leftLimit)+renderGap(gap)+styleDimText.Render(rightRaw),
		)
	}
	return renderOwnedLineKnownOwned(width, m.renderSidebarStatusLeft(max(width, 1)))
}

func (m *uiModel) sidebarStatusTextRaw() string {
	leftRaw := "JOB"
	if status := m.sidebarStatusText(); status != "" {
		leftRaw += " " + status
	}
	return leftRaw
}

func (m *uiModel) renderSidebarStatusLeft(width int) string {
	if width <= 0 {
		return ""
	}
	label := "JOB"
	status := m.sidebarStatusText()
	if status == "" || width <= lipgloss.Width(label) {
		return stylePanelLabel.Render(truncateString(label, width))
	}
	statusWidth := width - lipgloss.Width(label) - 1
	if statusWidth <= 0 {
		return stylePanelLabel.Render(truncateString(label, width))
	}
	return stylePanelLabel.Render(
		label,
	) + renderGap(
		1,
	) + m.sidebarStatusStyle().
		Render(truncateString(status, statusWidth))
}

func (m *uiModel) sidebarStatusText() string {
	if m == nil {
		return ""
	}
	if m.shutdown.Active() {
		return m.shutdownHeaderLabel()
	}
	progress := fmt.Sprintf("%d/%d", m.settledJobs(), m.total)
	segments := make([]string, 0, 4)
	segments = append(segments, progress)
	// Show the issue total when atomic grouping packed more issues than jobs, so
	// "1 job / 2 issues" never reads as if an issue were dropped.
	if issues := m.totalIssues(); issues > m.total {
		segments = append(segments, fmt.Sprintf("%d issues", issues))
	}
	if m.failed > 0 {
		segments = append(segments, fmt.Sprintf("%d FAIL", m.failed))
	}
	if m.parked > 0 {
		segments = append(segments, fmt.Sprintf("%d PARKED", m.parked))
	}
	return strings.Join(segments, " · ")
}

func (m *uiModel) sidebarStatusStyle() lipgloss.Style {
	if m == nil {
		return styleMutedText
	}
	if m.shutdown.Active() || m.failed > 0 || m.parked > 0 {
		return lipgloss.NewStyle().Bold(true).Foreground(colorWarning)
	}
	if m.total > 0 && m.isRunComplete() {
		return lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
	}
	return styleMutedText
}

// renderSidebarItem renders a single job as a bordered card:
//
//	┌────────────────────────────────┐
//	│ <icon> <NN>  <task title>       │
//	│ <task type> · <elapsed> · <tok> │
//	└────────────────────────────────┘
//
// The card stack keeps every border muted and uses foreground text to carry
// selection; refreshSidebarContent shares one separator border row between
// adjacent cards so there is no visual gap in the stack. The official task
// number (NN) comes from the task_NN.md filename, falling back to the slice
// position when there is none.
func (m *uiModel) renderSidebarItem(index int, job *uiJob, selected bool) string {
	key := m.sidebarRowKey(index, job, selected)
	if job.sidebarCacheValid && job.sidebarCacheKey == key {
		return job.sidebarCacheRow
	}

	cardWidth := m.sidebarViewport.Width()
	innerWidth := sidebarCardContentWidth(cardWidth)
	icon := m.jobStateIcon(job.state)
	iconRendered := lipgloss.NewStyle().Foreground(m.jobStateColor(job.state)).Render(icon)

	number := fmt.Sprintf("%02d", m.sidebarJobNumber(index, job))
	numberStyle := styleDimText
	titleStyle := styleMutedText
	if selected {
		numberStyle = styleMutedText
		titleStyle = styleBodyText.Bold(true)
	}

	// Width math runs on plain text; ANSI is applied afterwards per span so
	// truncation never lands inside an escape sequence. Lead = icon + space +
	// number + two spaces.
	leadWidth := lipgloss.Width(icon) + 1 + lipgloss.Width(number) + 2
	titleRaw := truncateString(sidebarJobTitle(job), max(innerWidth-leadWidth, 1))
	titleLine := iconRendered + renderGap(1) + numberStyle.Render(number) + renderGap(2) + titleStyle.Render(titleRaw)

	metaRaw := truncateString(m.sidebarMetaText(job), innerWidth)
	metaLine := styleDimText.Render(metaRaw)

	card := sidebarCardStyle(cardWidth, colorBorder).Render(titleLine + "\n" + metaLine)

	job.sidebarCacheKey = key
	job.sidebarCacheRow = card
	job.sidebarCacheValid = true
	return card
}

func renderSidebarStack(items []string, width int) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, sidebarRowLines+(len(items)-1)*sidebarRowStride)
	for i, item := range items {
		cardLines := normalizeSidebarCardLines(item, width)
		if i == 0 {
			lines = append(lines, cardLines[0])
		}
		lines = append(lines, cardLines[1:sidebarRowLines-1]...)
		if i == len(items)-1 {
			lines = append(lines, cardLines[sidebarRowLines-1])
			continue
		}
		lines = append(lines, sidebarCardSeparator(width, colorBorder))
	}
	return strings.Join(lines, "\n")
}

func normalizeSidebarCardLines(item string, width int) []string {
	cardLines := strings.Split(item, "\n")
	if len(cardLines) == sidebarRowLines {
		return cardLines
	}
	if len(cardLines) > sidebarRowLines {
		cardLines = append(cardLines[:sidebarRowLines-1], cardLines[len(cardLines)-1])
	}
	for len(cardLines) < sidebarRowLines {
		cardLines = append(cardLines, renderOwnedLineKnownOwned(width, ""))
	}
	for i := range cardLines {
		cardLines[i] = renderOwnedLineKnownOwned(width, cardLines[i])
	}
	return cardLines
}

// sidebarJobNumber returns the job's official task number, falling back to the
// 1-based slice position when the job has no canonical number (review/exec runs).
func (m *uiModel) sidebarJobNumber(index int, job *uiJob) int {
	if job.taskNumber > 0 {
		return job.taskNumber
	}
	return index + 1
}

// sidebarMetaText builds the muted card meta line: task type, elapsed/duration,
// and provider token total, joining only the parts that are present.
func (m *uiModel) sidebarMetaText(job *uiJob) string {
	parts := make([]string, 0, 3)
	if t := strings.TrimSpace(job.taskType); t != "" {
		parts = append(parts, t)
	}
	if ts := m.sidebarTimeString(job); ts != "" {
		parts = append(parts, ts)
	}
	if tokens := sidebarTokenLabel(job); tokens != "" {
		parts = append(parts, tokens)
	}
	return strings.Join(parts, " · ")
}

func sidebarTokenLabel(job *uiJob) string {
	if job == nil || job.tokenUsage == nil {
		return ""
	}
	return formatUsageTotalLabel(job.tokenUsage)
}

// sidebarJobTitle prefers the human task title, falling back to the safe name so
// rows are never blank before a queued event lands.
func sidebarJobTitle(job *uiJob) string {
	return firstNonEmpty(job.taskTitle, job.safeName)
}

// Job state glyphs. Shapes are distinct (not color-dependent) so status reads
// without color, and tests reference these instead of magic literals.
const (
	jobIconPending = "⏸"
	jobIconRetry   = "↻"
	jobIconStalled = "⧗"
	jobIconParked  = "⚑"
	jobIconSuccess = "✓"
	jobIconFailed  = "✗"
	jobIconUnknown = "•"
	// glyphActiveDot marks an in-progress/active item where a per-frame spinner is
	// not rendered (run tabs, in-progress tool calls).
	glyphActiveDot = "●"
)

func (m *uiModel) jobStateIcon(state jobState) string {
	switch state {
	case jobPending:
		return jobIconPending
	case jobRunning:
		return spinnerFrames[m.frame%len(spinnerFrames)]
	case jobPausing:
		return spinnerFrames[m.frame%len(spinnerFrames)]
	case jobPaused:
		return jobIconPending
	case jobStalled:
		return jobIconStalled
	case jobRetrying:
		return jobIconRetry
	case jobSuccess:
		return jobIconSuccess
	case jobFailed:
		return jobIconFailed
	case jobParked:
		return jobIconParked
	default:
		return jobIconUnknown
	}
}

func (m *uiModel) jobStateColor(state jobState) color.Color {
	switch state {
	case jobPending:
		return colorMuted
	case jobRunning:
		return colorAccentAlt
	case jobPausing:
		return colorWarning
	case jobPaused:
		return colorInfo
	case jobStalled:
		return colorWarning
	case jobRetrying:
		return colorWarning
	case jobSuccess:
		return colorSuccess
	case jobFailed:
		return colorError
	case jobParked:
		return colorAccent
	default:
		return colorInfo
	}
}

func (m *uiModel) jobBorderColor(job *uiJob) color.Color {
	switch job.state {
	case jobRunning:
		return colorBorderFocus
	case jobPausing:
		return colorWarning
	case jobPaused:
		return colorInfo
	case jobStalled:
		return colorWarning
	case jobRetrying:
		return colorWarning
	case jobSuccess:
		return colorSuccess
	case jobFailed:
		return colorError
	case jobParked:
		return colorAccent
	default:
		return colorBorder
	}
}

func (m *uiModel) currentTime() time.Time {
	if m != nil && !m.now.IsZero() {
		return m.now
	}
	return time.Now()
}

func (m *uiModel) jobElapsedDuration(job *uiJob) time.Duration {
	if job == nil {
		return 0
	}
	switch job.state {
	case jobRunning, jobPausing, jobStalled:
		if job.startedAt.IsZero() {
			return 0
		}
		return m.currentTime().Sub(job.startedAt)
	case jobPaused, jobSuccess, jobFailed, jobParked:
		if job.duration > 0 {
			return job.duration
		}
		if job.startedAt.IsZero() {
			return 0
		}
		return m.currentTime().Sub(job.startedAt)
	default:
		return 0
	}
}

func (m *uiModel) sidebarTimeString(job *uiJob) string {
	switch job.state {
	case jobRunning:
		if d := m.jobElapsedDuration(job); d > 0 {
			return formatDuration(d)
		}
	case jobPausing:
		return "pausing"
	case jobPaused:
		return "paused"
	case jobStalled:
		return "stalled"
	case jobRetrying:
		return m.retryAttemptLabel(job)
	case jobParked:
		return "parked"
	case jobSuccess, jobFailed:
		if d := m.jobElapsedDuration(job); d > 0 {
			return formatDuration(d)
		}
	}
	return ""
}

func (m *uiModel) sidebarRowKey(index int, job *uiJob, selected bool) sidebarRowCacheKey {
	key := sidebarRowCacheKey{
		selected:    selected,
		width:       m.sidebarViewport.Width(),
		index:       index,
		taskNumber:  job.taskNumber,
		state:       job.state,
		safeName:    job.safeName,
		taskTitle:   job.taskTitle,
		taskType:    job.taskType,
		attempt:     job.attempt,
		maxAttempts: job.maxAttempts,
	}
	if job.tokenUsage != nil {
		key.inputTokens = job.tokenUsage.InputTokens
		key.outputTokens = job.tokenUsage.OutputTokens
		key.totalTokens = job.tokenUsage.TotalTokens
	}
	if d := m.jobElapsedDuration(job); d > 0 {
		key.elapsedSeconds = int64(d / time.Second)
	}
	if (job.state == jobRunning || job.state == jobPausing) && len(spinnerFrames) > 0 {
		key.spinnerFrame = m.frame % len(spinnerFrames)
	}
	return key
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Truncate(time.Second)
	hours := int(d / time.Hour)
	minutes := int((d % time.Hour) / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// formatTokens renders a token count compactly: 850, 8.1k, 1.2M. Trailing ".0"
// is trimmed so round thousands read as "1k" rather than "1.0k".
func formatTokens(n int) string {
	if n < 0 {
		n = 0
	}
	if n < 1000 {
		return strconv.Itoa(n)
	}
	// Guard the k→M boundary: values like 999_999 would otherwise round to
	// "1000k" instead of rolling over to "1M".
	if k := float64(n) / 1000; k < 999.95 {
		return trimTrailingZero(fmt.Sprintf("%.1f", k)) + "k"
	}
	return trimTrailingZero(fmt.Sprintf("%.1f", float64(n)/1_000_000)) + "M"
}

func trimTrailingZero(s string) string {
	return strings.TrimSuffix(s, ".0")
}
