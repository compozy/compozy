package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/compozy/compozy/internal/core/tasks"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ValidationDecision string

const (
	ValidationContinued ValidationDecision = "continued"
	ValidationAborted   ValidationDecision = "aborted"
)

const (
	validationFormMinWidth  = 60
	validationFormDefaultW  = 100
	validationFormDefaultH  = 28
	validationFormMaxWidth  = 120
	validationFormMinHeight = 18
)

type validationFormModel struct {
	report              tasks.Report
	fixPrompt           string
	stderr              io.Writer
	clipboardWrite      func(string) error
	width               int
	height              int
	decision            ValidationDecision
	shouldCopyFixPrompt bool
	issueViewport       viewport.Model
}

var _ tea.Model = (*validationFormModel)(nil)

func newValidationFormModel(
	report tasks.Report,
	registry *tasks.TypeRegistry,
	stderr io.Writer,
	clipboardWrite func(string) error,
) *validationFormModel {
	issueViewport := viewport.New(
		viewport.WithWidth(validationFormDefaultW),
		viewport.WithHeight(1),
	)
	issueViewport.FillHeight = true

	model := &validationFormModel{
		report:         report,
		fixPrompt:      tasks.FixPrompt(report, registry),
		stderr:         stderr,
		clipboardWrite: clipboardWrite,
		width:          validationFormDefaultW,
		height:         validationFormDefaultH,
		issueViewport:  issueViewport,
	}
	model.syncIssueViewport()
	return model
}

func RunValidationFormWithIO(
	report tasks.Report,
	registry *tasks.TypeRegistry,
	stderr io.Writer,
	input io.Reader,
	output io.Writer,
	clipboardWrite func(string) error,
) (ValidationDecision, error) {
	model := newValidationFormModel(report, registry, stderr, clipboardWrite)
	program := tea.NewProgram(
		model,
		tea.WithInput(resolveValidationFormInput(input)),
		tea.WithOutput(resolveValidationFormOutput(output)),
		tea.WithoutSignalHandler(),
	)
	result, err := program.Run()
	if err != nil {
		return "", fmt.Errorf("run validation preflight form: %w", err)
	}

	typed, ok := result.(*validationFormModel)
	if !ok {
		return "", fmt.Errorf("unexpected validation form result type %T", result)
	}
	if typed.shouldCopyFixPrompt {
		if err := typed.copyFixPrompt(); err != nil {
			return "", err
		}
	}
	if typed.decision == "" {
		return ValidationAborted, nil
	}
	return typed.decision, nil
}

func (m *validationFormModel) Init() tea.Cmd {
	return nil
}

func (m *validationFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(typed.Width, validationFormMinWidth)
		m.height = max(typed.Height, validationFormMinHeight)
		m.syncIssueViewport()
	case tea.KeyPressMsg:
		switch strings.ToLower(typed.String()) {
		case "up", "k":
			m.issueViewport.ScrollUp(1)
		case "down", "j":
			m.issueViewport.ScrollDown(1)
		case "pgup":
			m.issueViewport.PageUp()
		case "pgdown":
			m.issueViewport.PageDown()
		case "home":
			m.issueViewport.GotoTop()
		case "end":
			m.issueViewport.GotoBottom()
		case "c":
			m.decision = ValidationContinued
			return m, tea.Quit
		case "a", "esc", "ctrl+c":
			m.decision = ValidationAborted
			return m, tea.Quit
		case "p":
			m.shouldCopyFixPrompt = true
			m.decision = ValidationAborted
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *validationFormModel) View() tea.View {
	return tea.NewView(lipgloss.Place(
		max(m.width, validationFormMinWidth),
		max(m.height, validationFormMinHeight),
		lipgloss.Center,
		lipgloss.Center,
		m.renderPanel(),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(colorBgBase)),
	))
}

func (m *validationFormModel) syncIssueViewport() {
	panel := m.panelStyle()
	contentWidth := m.contentWidth()
	availableBodyHeight := max(m.height-panel.GetVerticalFrameSize(), 1)
	reservedHeight := lipgloss.Height(m.renderTitle()) +
		lipgloss.Height(m.renderSummary(contentWidth)) +
		lipgloss.Height(m.renderHelp(contentWidth)) +
		3
	issueHeight := max(availableBodyHeight-reservedHeight, 1)

	m.issueViewport.SetWidth(contentWidth)
	m.issueViewport.SetHeight(issueHeight)
	m.issueViewport.SetContent(renderValidationIssueList(m.report.Issues, contentWidth))
}

func (m *validationFormModel) panelWidth() int {
	return clamp(m.width-6, validationFormMinWidth, validationFormMaxWidth)
}

func (m *validationFormModel) panelStyle() lipgloss.Style {
	return techPanelStyle(m.panelWidth(), colorWarning).Padding(1, 2)
}

func (m *validationFormModel) contentWidth() int {
	return max(m.panelWidth()-m.panelStyle().GetHorizontalFrameSize(), 1)
}

func (m *validationFormModel) renderPanel() string {
	contentWidth := m.contentWidth()
	return m.panelStyle().Render(m.renderBody(contentWidth))
}

func (m *validationFormModel) renderBody(contentWidth int) string {
	bg := colorBgSurface
	spacer := renderOwnedLine(contentWidth, bg, "")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		renderOwnedLine(contentWidth, bg, m.renderTitle()),
		spacer,
		renderOwnedBlock(contentWidth, bg, m.renderSummary(contentWidth)),
		spacer,
		renderOwnedBlock(contentWidth, bg, m.issueViewport.View()),
		spacer,
		renderOwnedBlock(contentWidth, bg, m.renderHelp(contentWidth)),
	)
}

func (m *validationFormModel) renderTitle() string {
	return renderStyledOnBackground(
		lipgloss.NewStyle().Bold(true).Foreground(colorWarning),
		colorBgSurface,
		"Task Metadata Validation Required",
	)
}

func (m *validationFormModel) renderSummary(contentWidth int) string {
	return styleBodyText.Background(colorBgSurface).Width(contentWidth).Render(
		fmt.Sprintf(
			"%d issue(s) across %d file(s) were found before task execution. Choose how to proceed.",
			len(m.report.Issues),
			distinctValidationIssuePaths(m.report.Issues),
		),
	)
}

func (m *validationFormModel) renderHelp(contentWidth int) string {
	return styleMutedText.Background(colorBgSurface).Width(contentWidth).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			renderKeycap("c", colorBgSurface)+" Continue anyway",
			"   ",
			renderKeycap("a", colorBgSurface)+" Abort",
			"   ",
			renderKeycap("p", colorBgSurface)+" Copy fix prompt",
		),
	)
}

func renderValidationIssueList(issues []tasks.Issue, width int) string {
	if len(issues) == 0 {
		return styleMutedText.Background(colorBgSurface).Width(width).Render("No validation issues.")
	}

	lines := make([]string, 0, len(issues)*2)
	currentPath := ""
	for _, issue := range issues {
		if issue.Path != currentPath {
			currentPath = issue.Path
			lines = append(
				lines,
				styleTitleMeta.Background(colorBgSurface).Width(width).Render(filepath.Clean(currentPath)),
			)
		}
		lines = append(
			lines,
			styleBodyText.Background(colorBgSurface).Width(width).Render(
				fmt.Sprintf("- %s: %s", issue.Field, issue.Message),
			),
		)
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

func resolveValidationFormInput(input io.Reader) io.Reader {
	if input != nil {
		return input
	}
	return os.Stdin
}

func resolveValidationFormOutput(output io.Writer) io.Writer {
	if output != nil {
		return output
	}
	return os.Stdout
}

func (m *validationFormModel) copyFixPrompt() error {
	if strings.TrimSpace(m.fixPrompt) == "" {
		return nil
	}

	clipboardWriter := m.clipboardWrite
	if clipboardWriter == nil {
		clipboardWriter = clipboard.WriteAll
	}
	err := clipboardWriter(m.fixPrompt)
	if err == nil {
		if m.stderr == nil {
			return nil
		}
		if _, writeErr := fmt.Fprintln(m.stderr, "Fix prompt copied to clipboard."); writeErr != nil {
			return fmt.Errorf("write clipboard confirmation: %w", writeErr)
		}
		return nil
	}
	if m.stderr != nil {
		if _, writeErr := fmt.Fprintf(
			m.stderr,
			"Unable to copy fix prompt to clipboard: %v\n\nFix prompt:\n%s\n",
			err,
			m.fixPrompt,
		); writeErr != nil {
			return fmt.Errorf("write validation fix prompt fallback: %w", writeErr)
		}
		return nil
	}

	return fmt.Errorf("copy validation fix prompt to clipboard: %w", err)
}
