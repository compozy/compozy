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
