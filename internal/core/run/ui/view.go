package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *uiModel) renderRoot(content string) tea.View {
	v := tea.NewView(rootScreenStyle(m.width, m.height).Render(content))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
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
		renderOwnedLineKnownOwned(m.width-4, colorBgSurface, renderTechLabel("ui.resize", colorBgSurface)),
		renderOwnedLineKnownOwned(m.width-4, colorBgSurface, "ACP cockpit needs at least 80x24."),
		renderOwnedLineKnownOwned(m.width-4, colorBgSurface, fmt.Sprintf("Current size: %dx%d", m.width, m.height)),
	}
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.contentHeight).
		Padding(1, 1).
		Background(colorBgBase).
		Render(techPanelStyle(max(m.width-2, 10), colorWarning).Render(strings.Join(message, "\n")))
}

func (m *uiModel) renderTitleBar() string {
	bg := colorBgBase
	title := renderStyledOnBackground(styleTitle, bg, "COMPOZY") +
		renderStyledOnBackground(styleTitleMeta, bg, " // ACP COCKPIT")
	status := m.headerStatusText(bg)

	gap := max(m.width-lipgloss.Width(title)-lipgloss.Width(status)-2, 1)
	titleLine := renderGap(bg, 1) + title + renderGap(bg, gap) + status
	titleLine = renderOwnedLineKnownOwned(m.width, bg, titleLine)

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
	progressLine = renderOwnedLineKnownOwned(m.width, bg, progressLine)

	return renderOwnedLineKnownOwned(m.width, bg, "") + "\n" + titleLine + "\n" + progressLine
}

func (m *uiModel) headerStatusText(bg color.Color) string {
	complete := m.completed+m.failed >= m.total
	if !complete {
		if m.shutdown.Active() {
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
	remaining := m.shutdown.DeadlineAt.Sub(m.currentTime())
	if remaining < 0 {
		remaining = 0
	}
	return remaining.Truncate(time.Second).String()
}

func (m *uiModel) renderSeparator() string {
	return renderOwnedLineKnownOwned(
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
	if !m.isRunComplete() {
		quitLabel = "EXIT"
	}
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
	return renderOwnedLineKnownOwned(m.width, bg, line) + "\n" + renderOwnedLineKnownOwned(m.width, bg, "")
}

func (m *uiModel) renderQuitDialogView() tea.View {
	panel := m.renderQuitDialogPanel()
	content := lipgloss.Place(
		max(m.width, 1),
		max(m.height, 1),
		lipgloss.Center,
		lipgloss.Center,
		panel,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(colorBgBase)),
	)
	return m.renderRoot(content)
}

func (m *uiModel) renderQuitDialogPanel() string {
	availableWidth := max(m.width-4, 1)
	panelWidth := min(availableWidth, quitDialogMaxWidth)
	panelStyle := techPanelStyle(panelWidth, colorBorderFocus).Padding(1, 2)
	innerWidth := max(panelWidth-panelStyle.GetHorizontalFrameSize(), 1)
	bg := colorBgSurface

	lines := []string{
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				lipgloss.NewStyle().Bold(true).Foreground(colorAccentDeep),
				bg,
				truncateString("Leave Active Run?", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(styleBodyText, bg, truncateString("This run is still active.", innerWidth)),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleMutedText,
				bg,
				truncateString("Close the TUI and keep the run running.", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleMutedText,
				bg,
				truncateString("Choose Stop Run only if you want to end it now.", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedBlock(innerWidth, bg, m.renderQuitDialogActions(innerWidth, bg)),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleDimText,
				bg,
				truncateString("[enter/q] confirm  [tab/left/right] choice  [esc] back", innerWidth),
			),
		),
	}

	return panelStyle.Render(strings.Join(lines, "\n"))
}

func (m *uiModel) renderQuitDialogActions(width int, bg color.Color) string {
	actions := []string{
		m.renderQuitDialogAction("Close TUI", quitDialogActionClose),
		m.renderQuitDialogAction("Stop Run", quitDialogActionStop),
		m.renderQuitDialogAction("Cancel", quitDialogActionCancel),
	}
	if width < 44 {
		return strings.Join(actions, "\n")
	}
	return strings.Join(actions, renderGap(bg, 1))
}

func (m *uiModel) renderQuitDialogAction(label string, action quitDialogAction) string {
	baseStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	if m.quitDialog.Selected == action {
		return baseStyle.Foreground(colorBgSurface).Background(colorAccent).Render(label)
	}
	return baseStyle.Foreground(colorFgBright).Background(colorBgBase).Render(label)
}
