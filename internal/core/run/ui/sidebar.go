package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *uiModel) elapsedStr(job *uiJob, bg color.Color) string {
	switch job.state {
	case jobRunning:
		if d := m.jobElapsedDuration(job); d > 0 {
			return renderStyledOnBackground(styleDimText, bg, formatDuration(d))
		}
	case jobRetrying:
		if label := m.retryAttemptLabel(job); label != "" {
			return lipgloss.NewStyle().Foreground(colorWarning).Background(bg).Render("RETRY " + label)
		}
		return lipgloss.NewStyle().Foreground(colorWarning).Background(bg).Render("RETRY")
	case jobSuccess:
		if d := m.jobElapsedDuration(job); d > 0 {
			return lipgloss.NewStyle().Foreground(colorSuccess).Background(bg).Render("OK " + formatDuration(d))
		}
	case jobFailed:
		if d := m.jobElapsedDuration(job); d > 0 {
			return lipgloss.NewStyle().Foreground(colorError).Background(bg).Render("FAIL " + formatDuration(d))
		}
	}
	return ""
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
	key := m.sidebarRowKey(job, selected)
	if job.sidebarCacheValid && job.sidebarCacheKey == key {
		return job.sidebarCacheRow
	}

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

	timeStr := m.sidebarTimeString(job)

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
	line1 = renderOwnedLineKnownOwned(maxW, bg, line1)

	line2Raw := truncateString(m.sidebarMeta(job), maxW)
	metaStyle := styleDimText
	if selected {
		metaStyle = styleMutedText
	}
	line2 := renderOwnedLineKnownOwned(maxW, bg, renderStyledOnBackground(metaStyle, bg, line2Raw))
	row := line1 + "\n" + line2
	if selected {
		row = selectedSidebarRowStyle(maxW).Render(row)
	}

	job.sidebarCacheKey = key
	job.sidebarCacheRow = row
	job.sidebarCacheValid = true
	return row
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
	case jobRunning:
		if job.startedAt.IsZero() {
			return 0
		}
		return m.currentTime().Sub(job.startedAt)
	case jobSuccess, jobFailed:
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
	case jobRetrying:
		return m.retryAttemptLabel(job)
	case jobSuccess, jobFailed:
		if d := m.jobElapsedDuration(job); d > 0 {
			return formatDuration(d)
		}
	}
	return ""
}

func (m *uiModel) sidebarRowKey(job *uiJob, selected bool) sidebarRowCacheKey {
	key := sidebarRowCacheKey{
		selected:    selected,
		width:       m.sidebarViewport.Width(),
		state:       job.state,
		safeName:    job.safeName,
		issues:      job.issues,
		fileCount:   len(job.codeFiles),
		attempt:     job.attempt,
		maxAttempts: job.maxAttempts,
		retrying:    job.retrying,
		retryReason: job.retryReason,
	}
	if d := m.jobElapsedDuration(job); d > 0 {
		key.elapsedSeconds = int64(d / time.Second)
	}
	if job.state == jobRunning && len(spinnerFrames) > 0 {
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
