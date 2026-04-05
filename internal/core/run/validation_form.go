package run

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/tasks"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	validationFormMinWidth  = 60
	validationFormDefaultW  = 100
	validationFormDefaultH  = 28
	validationFormMaxWidth  = 120
	validationFormMinHeight = 18
)

type validationFormModel struct {
	report    tasks.Report
	fixPrompt string
	stderr    io.Writer
	width     int
	height    int
	decision  PreflightDecision
}

var _ tea.Model = (*validationFormModel)(nil)

func newValidationFormModel(report tasks.Report, registry *tasks.TypeRegistry, stderr io.Writer) *validationFormModel {
	return &validationFormModel{
		report:    report,
		fixPrompt: tasks.FixPrompt(report, registry),
		stderr:    stderr,
		width:     validationFormDefaultW,
		height:    validationFormDefaultH,
	}
}

func (m *validationFormModel) Init() tea.Cmd {
	return nil
}

func (m *validationFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(typed.Width, validationFormMinWidth)
		m.height = max(typed.Height, validationFormMinHeight)
	case tea.KeyPressMsg:
		switch strings.ToLower(typed.String()) {
		case "c":
			m.decision = PreflightContinued
			return m, tea.Quit
		case "a", "esc", "ctrl+c":
			m.decision = PreflightAborted
			return m, tea.Quit
		case "p":
			if err := m.writeFixPrompt(); err != nil {
				return m, func() tea.Msg { return err }
			}
			m.decision = PreflightAborted
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *validationFormModel) View() tea.View {
	panelWidth := clampInt(m.width-6, validationFormMinWidth, validationFormMaxWidth)
	panel := techPanelStyle(panelWidth, colorWarning).Padding(1, 2)
	contentWidth := max(panelWidth-panel.GetHorizontalFrameSize(), 1)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorWarning).
		Width(contentWidth).
		Render("Task Metadata Validation Required")
	summary := styleBodyText.Width(contentWidth).Render(
		fmt.Sprintf(
			"%d issue(s) across %d file(s) were found before task execution. Choose how to proceed.",
			len(m.report.Issues),
			distinctValidationIssuePaths(m.report.Issues),
		),
	)
	issues := renderValidationIssueList(m.report.Issues, contentWidth)
	help := styleMutedText.Width(contentWidth).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			renderKeycap("c", colorBgSurface)+" Continue anyway",
			"   ",
			renderKeycap("a", colorBgSurface)+" Abort",
			"   ",
			renderKeycap("p", colorBgSurface)+" Copy fix prompt",
		),
	)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		summary,
		"",
		issues,
		"",
		help,
	)

	return tea.NewView(lipgloss.Place(
		max(m.width, validationFormMinWidth),
		max(m.height, validationFormMinHeight),
		lipgloss.Center,
		lipgloss.Center,
		panel.Render(body),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(colorBgBase)),
	))
}

func (m *validationFormModel) writeFixPrompt() error {
	if m.stderr == nil || strings.TrimSpace(m.fixPrompt) == "" {
		return nil
	}
	if _, err := fmt.Fprintf(m.stderr, "%s\n", m.fixPrompt); err != nil {
		return fmt.Errorf("write validation fix prompt: %w", err)
	}
	return nil
}

func renderValidationIssueList(issues []tasks.Issue, width int) string {
	if len(issues) == 0 {
		return styleMutedText.Width(width).Render("No validation issues.")
	}

	lines := make([]string, 0, len(issues)*2)
	currentPath := ""
	for _, issue := range issues {
		if issue.Path != currentPath {
			currentPath = issue.Path
			lines = append(lines, styleTitleMeta.Width(width).Render(filepath.Clean(currentPath)))
		}
		lines = append(lines, styleBodyText.Width(width).Render(fmt.Sprintf("- %s: %s", issue.Field, issue.Message)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func distinctValidationIssuePaths(issues []tasks.Issue) int {
	paths := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		paths[issue.Path] = struct{}{}
	}
	return len(paths)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
