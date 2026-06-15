package ui

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/charmtheme"
)

func (m *uiModel) renderRoot(content string) tea.View {
	v := tea.NewView(rootScreenStyle(m.width, m.height).Render(content))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.Cursor = m.composerTerminalCursor()
	return v
}

func (m *uiModel) composerTerminalCursor() *tea.Cursor {
	if m == nil ||
		m.currentView != uiViewJobs ||
		m.layoutMode != uiLayoutSplit ||
		m.focusedPane != uiPaneComposer ||
		!m.composerEnabled(m.currentJob()) {
		return nil
	}
	cursor := m.composer.Cursor()
	if cursor == nil {
		return nil
	}
	cursor.X += m.sidebarWidth + styleTechPanelBase.GetBorderLeftSize() + styleTechPanelBase.GetPaddingLeft()
	cursor.Y += m.jobsBodyTopOffset() + timelineHeaderBoxHeight() + timelineMessagesBoxHeight(
		m.contentHeight,
	) +
		styleTechPanelBase.GetBorderTopSize() + styleTechPanelBase.GetPaddingTop()
	return cursor
}

func (m *uiModel) jobsBodyTopOffset() int {
	if m == nil || m.headerHidden {
		return 0
	}
	return headerSectionHeight + separatorSectionHeight
}

func timelineHeaderBoxHeight() int {
	return 2 + styleTechPanelBase.GetVerticalFrameSize()
}

func timelineMessagesBoxHeight(contentHeight int) int {
	return max(contentHeight-timelineChromeRows, logViewportMinHeight) + styleTechPanelBase.GetVerticalFrameSize()
}

func (m *uiModel) View() tea.View {
	if m.quitDialog.Active {
		return m.renderQuitDialogView()
	}
	switch m.currentView {
	case uiViewSummary, uiViewFailures:
		return m.renderSummaryView()
	case uiViewJobs:
		body := m.renderJobsBody()
		sections := make([]string, 0, 5)
		// Embedded children render inside the tabbed shells, which own the brand+tabs
		// row and the divider beneath it; skip them here to avoid a duplicated header.
		if !m.headerHidden {
			sections = append(sections, m.renderTitleBar(), m.renderSeparator())
		}
		sections = append(sections, body, m.renderSeparator(), m.renderHelp())
		content := lipgloss.JoinVertical(lipgloss.Left, sections...)
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
		renderOwnedLineKnownOwned(m.width-4, renderTechLabel("ui.resize")),
		renderOwnedLineKnownOwned(m.width-4, "Compozy needs at least 80x24."),
		renderOwnedLineKnownOwned(m.width-4, fmt.Sprintf("Current size: %dx%d", m.width, m.height)),
	}
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.contentHeight).
		Padding(1, 1).
		Render(techPanelStyle(max(m.width-2, 10), colorWarning).Render(strings.Join(message, "\n")))
}

// renderTitleBar draws the single top row: the COMPOZY brand plus the workflow
// shown as one chip, so the standalone cockpit matches the tabbed shells where the
// brand and tabs always share one row. Run progress lives with the jobs list so the
// global chrome stays out of the task surface.
func (m *uiModel) renderTitleBar() string {
	return renderBrandTabsRow(m.width, []string{m.workflowChip()}, "")
}

// workflowChip renders the single-run workflow as a tab chip: a status glyph, the
// workflow name, and the run status word.
func (m *uiModel) workflowChip() string {
	name := "workflow"
	if m.cfg != nil && strings.TrimSpace(m.cfg.Name) != "" {
		name = strings.TrimSpace(m.cfg.Name)
	}
	glyph, status, statusColor := m.runChipStatus()
	label := fmt.Sprintf("%s %s %s", glyph, name, status)
	return tabChipStyle(statusColor, false).Render(truncateString(label, 36))
}

// runChipStatus maps the aggregate run state to a glyph, status word, and color for
// the single-run workflow chip.
func (m *uiModel) runChipStatus() (string, string, color.Color) {
	switch strings.TrimSpace(m.runStatus) {
	case remoteRunStatusRunning, remoteRunStatusPausing, remoteRunStatusPaused, remoteRunStatusRetrying:
		return glyphActiveDot, statusLabelRunning, colorAccentAlt
	case remoteRunStatusFailed:
		return jobIconFailed, statusLabelFailed, colorError
	case remoteRunStatusCrashed:
		return jobIconFailed, statusLabelCrashed, colorError
	case remoteRunStatusCanceled:
		return jobIconFailed, statusLabelCanceled, colorWarning
	case remoteRunStatusCompleted:
		return jobIconSuccess, statusLabelDone, colorSuccess
	}
	switch {
	case m.shutdown.Active():
		return glyphActiveDot, "STOPPING", colorWarning
	case m.isRunComplete() && m.failed > 0:
		return jobIconFailed, statusLabelFailed, colorError
	case m.isRunComplete():
		return jobIconSuccess, statusLabelDone, colorSuccess
	default:
		return glyphActiveDot, statusLabelRunning, colorAccentAlt
	}
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
	remaining := m.shutdown.DeadlineAt.Sub(m.currentTime())
	if remaining < 0 {
		remaining = 0
	}
	return remaining.Truncate(time.Second).String()
}

func (m *uiModel) renderSeparator() string {
	return renderOwnedLineKnownOwned(m.width, styleSeparator.Render(strings.Repeat("─", m.width)))
}

func (m *uiModel) renderHelp() string {
	paneLabel := strings.ToUpper(string(m.focusedPane))
	left := renderGap(1) +
		styleDimText.Render("FOCUS "+paneLabel) +
		renderGap(2) +
		strings.Join(m.helpPairs(), renderGap(2))
	right := m.renderHelpWorkdir(lipgloss.Width(left))
	if right == "" {
		return renderOwnedLineKnownOwned(m.width, left)
	}
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return renderOwnedLineKnownOwned(m.width, left+renderGap(gap)+right)
}

// helpPairs builds the keycap/label hints for the focused pane. The pause shortcut
// is advertised whenever the selected job can be paused, and the composer pane
// surfaces send/cancel so the paused-task message flow is discoverable.
func (m *uiModel) helpPairs() []string {
	pairs := []string{}
	switch m.focusedPane {
	case uiPaneJobs:
		pairs = append(pairs,
			charmtheme.Keycap("↑↓/jk")+renderGap(1)+styleMutedText.Render("JOB"),
			charmtheme.Keycap(keyTab)+renderGap(1)+styleMutedText.Render("FOCUS"),
		)
		pairs = m.appendPauseHint(pairs)
	case uiPaneTimeline:
		pairs = append(pairs,
			charmtheme.Keycap("↑↓/jk")+renderGap(1)+styleMutedText.Render("ENTRY"),
			charmtheme.Keycap(keyEnter)+renderGap(1)+styleMutedText.Render("EXPAND"),
			charmtheme.Keycap("pg/home/end")+renderGap(1)+styleMutedText.Render("SCROLL"),
			charmtheme.Keycap(keyTab)+renderGap(1)+styleMutedText.Render("FOCUS"),
		)
		pairs = m.appendPauseHint(pairs)
	case uiPaneComposer:
		pairs = append(pairs,
			charmtheme.Keycap(keyEnter)+renderGap(1)+styleMutedText.Render("SEND"),
			charmtheme.Keycap(keyEscape)+renderGap(1)+styleMutedText.Render("CANCEL"),
			charmtheme.Keycap(keyTab)+renderGap(1)+styleMutedText.Render("FOCUS"),
		)
	}
	if m.isRunComplete() {
		pairs = append(pairs, charmtheme.Keycap("s")+renderGap(1)+styleMutedText.Render("SUMMARY"))
	}
	pairs = append(pairs, charmtheme.Keycap("q")+renderGap(1)+styleMutedText.Render(m.quitLabel()))
	return pairs
}

func (m *uiModel) appendPauseHint(pairs []string) []string {
	if m.jobCanPause(m.currentJob()) {
		pairs = append(pairs, charmtheme.Keycap("p")+renderGap(1)+styleMutedText.Render("PAUSE"))
	}
	return pairs
}

func (m *uiModel) quitLabel() string {
	switch m.shutdown.Phase {
	case shutdownPhaseDraining:
		return "FORCE QUIT"
	case shutdownPhaseForcing:
		return "FORCING"
	}
	if m.isRunComplete() {
		return "QUIT"
	}
	return "EXIT"
}

// renderHelpWorkdir returns the right-aligned working-directory label, truncated
// from the left (the path tail is the most meaningful part) to whatever space the
// help line leaves. Returns "" when there is no cwd or no room for it.
func (m *uiModel) renderHelpWorkdir(leftWidth int) string {
	dir := m.formatWorkdir()
	if dir == "" {
		return ""
	}
	available := m.width - leftWidth - 1
	if available < 4 {
		return ""
	}
	return styleDimText.Render(truncateStringLeft(dir, available))
}

// formatWorkdir renders the run's workspace root for display, abbreviating the
// user's home directory to "~". Returns "" when no workspace root is set.
func (m *uiModel) formatWorkdir() string {
	if m.cfg == nil {
		return ""
	}
	dir := strings.TrimSpace(m.cfg.WorkspaceRoot)
	if dir == "" {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		switch {
		case dir == home:
			return "~"
		case strings.HasPrefix(dir, home+string(os.PathSeparator)):
			return "~" + dir[len(home):]
		}
	}
	return dir
}

func (m *uiModel) renderQuitDialogView() tea.View {
	panel := m.renderQuitDialogPanel()
	content := lipgloss.Place(
		max(m.width, 1),
		max(m.height, 1),
		lipgloss.Center,
		lipgloss.Center,
		panel,
	)
	return m.renderRoot(content)
}

func (m *uiModel) renderQuitDialogPanel() string {
	availableWidth := max(m.width-4, 1)
	panelWidth := min(availableWidth, quitDialogMaxWidth)
	panelStyle := techPanelStyle(panelWidth, colorBorderFocus).Padding(1, 2)
	innerWidth := max(panelWidth-panelStyle.GetHorizontalFrameSize(), 1)

	lines := []string{
		renderOwnedLineKnownOwned(
			innerWidth,
			lipgloss.NewStyle().Bold(true).Foreground(colorAccentDeep).Render(
				truncateString("Leave Active Run?", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(innerWidth, ""),
		renderOwnedLineKnownOwned(
			innerWidth,
			styleBodyText.Render(truncateString("This run is still active.", innerWidth)),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			styleMutedText.Render(truncateString("Close the TUI and keep the run running.", innerWidth)),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			styleMutedText.Render(truncateString("Choose Stop Run only if you want to end it now.", innerWidth)),
		),
		renderOwnedLineKnownOwned(innerWidth, ""),
		renderOwnedBlock(innerWidth, m.renderQuitDialogActions(innerWidth)),
		renderOwnedLineKnownOwned(innerWidth, ""),
		renderOwnedLineKnownOwned(
			innerWidth,
			styleDimText.Render(
				truncateString("[enter/q] confirm  [tab/left/right] choice  [esc] back", innerWidth),
			),
		),
	}

	return panelStyle.Render(strings.Join(lines, "\n"))
}

func (m *uiModel) renderQuitDialogActions(width int) string {
	actions := []string{
		m.renderQuitDialogAction("Close TUI", quitDialogActionClose),
		m.renderQuitDialogAction("Stop Run", quitDialogActionStop),
		m.renderQuitDialogAction("Cancel", quitDialogActionCancel),
	}
	if width < 44 {
		return strings.Join(actions, "\n")
	}
	return strings.Join(actions, renderGap(1))
}

func (m *uiModel) renderQuitDialogAction(label string, action quitDialogAction) string {
	baseStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	if m.quitDialog.Selected == action {
		return baseStyle.Foreground(colorBgSurface).Background(colorAccent).Render(label)
	}
	return baseStyle.Foreground(colorFgBright).Render(label)
}
