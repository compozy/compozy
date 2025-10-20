package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	cliutils "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/styles"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// GetCmd creates the workflow get command
func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <workflow-id>",
		Short: "Get detailed information about a workflow",
		Long:  "Display comprehensive workflow information including tasks, inputs, outputs, schedule, and statistics.",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowGet,
	}
	cmd.Flags().Bool("show-tasks", false, "Include detailed task information")
	return cmd
}

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

const reservedLines = 4 // Space reserved for header and footer in TUI

// runWorkflowGet handles the workflow get command execution
func runWorkflowGet(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: getJSONHandler,
		TUI:  getTUIHandler,
	}, args)
}

// getJSONHandler handles JSON mode for workflow get
func getJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return workflowGetJSONHandler(ctx, cobraCmd, executor.GetAuthClient(), args)
}

// getTUIHandler handles TUI mode for workflow get
func getTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return workflowGetTUIHandler(ctx, cobraCmd, executor.GetAuthClient(), args)
}

// workflowGetJSONHandler handles JSON output mode
func workflowGetJSONHandler(ctx context.Context, cmd *cobra.Command, client api.AuthClient, args []string) error {
	workflowID := core.ID(args[0])
	service := createAPIClient(client)
	workflow, err := service.Get(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}
	showTasks, err := cmd.Flags().GetBool("show-tasks")
	if err != nil {
		logger.FromContext(ctx).Debug("failed to get show-tasks flag", "error", err)
		showTasks = false
	}
	formatter := cliutils.NewJSONFormatter(true) // pretty print enabled
	outputData := workflow
	if !showTasks {
		workflowCopy := *workflow
		workflowCopy.Tasks = nil
		outputData = &workflowCopy
	}
	output, err := formatter.FormatSuccess(outputData, &cliutils.FormatterMetadata{
		Timestamp: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to format workflow data: %w", err)
	}
	fmt.Println(output)
	return nil
}

// workflowGetTUIHandler handles TUI output mode
func workflowGetTUIHandler(ctx context.Context, cmd *cobra.Command, client api.AuthClient, args []string) error {
	workflowID := core.ID(args[0])
	service := createAPIClient(client)
	workflow, err := service.Get(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}
	showTasks, err := cmd.Flags().GetBool("show-tasks")
	if err != nil {
		logger.FromContext(ctx).Debug("failed to get show-tasks flag", "error", err)
		showTasks = false
	}
	model := newWorkflowDetailModel(workflow, showTasks)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run workflow detail view: %w", err)
	}
	return nil
}

// workflowDetailModel represents the TUI model for workflow details
type workflowDetailModel struct {
	workflow     *api.WorkflowDetail
	showTasks    bool
	content      string
	scrollOffset int
	height       int
	ready        bool
}

// newWorkflowDetailModel creates a new workflow detail model
func newWorkflowDetailModel(workflow *api.WorkflowDetail, showTasks bool) *workflowDetailModel {
	return &workflowDetailModel{
		workflow:  workflow,
		showTasks: showTasks,
	}
}

// Init implements tea.Model
func (m *workflowDetailModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *workflowDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h := max(
			msg.Height-reservedLines,
			1)
		m.height = h
		m.content = m.renderContent()
		m.ready = true

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

// handleKeyMsg handles keyboard input messages
func (m *workflowDetailModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case "down", "j":
		m.handleScrollDown()
	case "pgup":
		m.handlePageUp()
	case "pgdn":
		m.handlePageDown()
	}
	return m, nil
}

// handleScrollDown handles scrolling down one line
func (m *workflowDetailModel) handleScrollDown() {
	if m.content != "" {
		contentLines := strings.Split(m.content, "\n")
		maxScroll := max(len(contentLines)-m.height, 0)
		if m.scrollOffset < maxScroll {
			m.scrollOffset++
		}
	}
}

// handlePageUp handles page up scrolling
func (m *workflowDetailModel) handlePageUp() {
	m.scrollOffset -= m.height / 2
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// handlePageDown handles page down scrolling
func (m *workflowDetailModel) handlePageDown() {
	if m.content != "" {
		contentLines := strings.Split(m.content, "\n")
		maxScroll := max(len(contentLines)-m.height, 0)
		m.scrollOffset += m.height / 2
		if m.scrollOffset > maxScroll {
			m.scrollOffset = maxScroll
		}
	}
}

// View implements tea.Model
func (m *workflowDetailModel) View() string {
	if !m.ready {
		return "Loading..."
	}
	header := styles.TitleStyle.Render(fmt.Sprintf("Workflow: %s", m.workflow.Name))
	lines := strings.Split(m.content, "\n")
	visibleLines := []string{}
	start := m.scrollOffset
	if start >= len(lines) {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := min(start+m.height, len(lines))
	if start < len(lines) {
		visibleLines = lines[start:end]
	}
	footer := styles.HelpStyle.Render("↑/↓: scroll • pgup/pgdn: page • q: quit")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		strings.Join(visibleLines, "\n"),
		footer,
	)
}

// renderContent renders the workflow details content
func (m *workflowDetailModel) renderContent() string {
	var sections []string
	sections = append(sections, m.renderBasicInfo())
	if len(m.workflow.Inputs) > 0 {
		sections = append(sections, m.renderInputs())
	}
	if len(m.workflow.Outputs) > 0 {
		sections = append(sections, m.renderOutputs())
	}
	if m.showTasks && len(m.workflow.Tasks) > 0 {
		sections = append(sections, m.renderTasks())
	}
	if m.workflow.Schedule != nil {
		sections = append(sections, m.renderSchedule())
	}
	if m.workflow.Statistics != nil {
		sections = append(sections, m.renderStatistics())
	}
	return strings.Join(sections, "\n\n")
}

// renderBasicInfo renders the basic workflow information
func (m *workflowDetailModel) renderBasicInfo() string {
	var lines []string
	lines = append(lines,
		styles.SubtitleStyle.Render("Basic Information"),
		fmt.Sprintf("ID:          %s", m.workflow.ID),
		fmt.Sprintf("Name:        %s", m.workflow.Name),
		fmt.Sprintf("Description: %s", m.workflow.Description),
		fmt.Sprintf("Version:     %s", m.workflow.Version),
		fmt.Sprintf("Status:      %s", m.renderStatus(m.workflow.Status)),
		fmt.Sprintf("Created:     %s", m.workflow.CreatedAt.Format("2006-01-02 15:04:05")),
		fmt.Sprintf("Updated:     %s", m.workflow.UpdatedAt.Format("2006-01-02 15:04:05")))
	if len(m.workflow.Tags) > 0 {
		lines = append(lines, fmt.Sprintf("Tags:        %s", strings.Join(m.workflow.Tags, ", ")))
	}
	return strings.Join(lines, "\n")
}

// renderStatus renders the workflow status with color
func (m *workflowDetailModel) renderStatus(status api.WorkflowStatus) string {
	switch status {
	case api.WorkflowStatusActive:
		return styles.SuccessStyle.Render(string(status))
	case api.WorkflowStatusInactive:
		return styles.WarningStyle.Render(string(status))
	case api.WorkflowStatusDeleted:
		return styles.ErrorStyle.Render(string(status))
	default:
		return string(status)
	}
}

// renderInputs renders the workflow inputs
func (m *workflowDetailModel) renderInputs() string {
	var lines []string
	lines = append(lines, styles.SubtitleStyle.Render("Inputs"))
	for _, input := range m.workflow.Inputs {
		required := ""
		if input.Required {
			required = styles.ErrorStyle.Render(" (required)")
		}
		lines = append(lines, fmt.Sprintf("  • %s [%s]%s", input.Name, input.Type, required))
		if input.Description != "" {
			lines = append(lines, fmt.Sprintf("    %s", input.Description))
		}
	}
	return strings.Join(lines, "\n")
}

// renderOutputs renders the workflow outputs
func (m *workflowDetailModel) renderOutputs() string {
	var lines []string
	lines = append(lines, styles.SubtitleStyle.Render("Outputs"))
	for _, output := range m.workflow.Outputs {
		lines = append(lines, fmt.Sprintf("  • %s [%s]", output.Name, output.Type))
		if output.Description != "" {
			lines = append(lines, fmt.Sprintf("    %s", output.Description))
		}
	}
	return strings.Join(lines, "\n")
}

// renderTasks renders the workflow tasks
func (m *workflowDetailModel) renderTasks() string {
	var lines []string
	lines = append(lines, styles.SubtitleStyle.Render("Tasks"))
	for i, task := range m.workflow.Tasks {
		lines = append(lines, fmt.Sprintf("  %d. %s [%s]", i+1, task.Name, task.Type))
		if task.Description != "" {
			lines = append(lines, fmt.Sprintf("     %s", task.Description))
		}
	}
	return strings.Join(lines, "\n")
}

// renderSchedule renders the workflow schedule
func (m *workflowDetailModel) renderSchedule() string {
	var lines []string
	lines = append(lines, styles.SubtitleStyle.Render("Schedule"))
	schedule := m.workflow.Schedule
	enabled := styles.ErrorStyle.Render("Disabled")
	if schedule.Enabled {
		enabled = styles.SuccessStyle.Render("Enabled")
	}
	lines = append(lines,
		fmt.Sprintf("Status:      %s", enabled),
		fmt.Sprintf("Expression:  %s", schedule.CronExpr),
		fmt.Sprintf("Timezone:    %s", schedule.Timezone),
		fmt.Sprintf("Next Run:    %s", schedule.NextRun.Format("2006-01-02 15:04:05")))
	if schedule.LastRun != nil {
		lines = append(lines, fmt.Sprintf("Last Run:    %s", schedule.LastRun.Format("2006-01-02 15:04:05")))
	}
	return strings.Join(lines, "\n")
}

// renderStatistics renders the workflow statistics
func (m *workflowDetailModel) renderStatistics() string {
	var lines []string
	lines = append(lines, styles.SubtitleStyle.Render("Statistics"))
	stats := m.workflow.Statistics
	lines = append(lines,
		fmt.Sprintf("Total Executions:      %d", stats.TotalExecutions),
		fmt.Sprintf("Successful:            %d", stats.SuccessfulExecutions),
		fmt.Sprintf("Failed:                %d", stats.FailedExecutions))
	if stats.TotalExecutions > 0 {
		successRate := float64(stats.SuccessfulExecutions) / float64(stats.TotalExecutions) * 100
		lines = append(lines, fmt.Sprintf("Success Rate:          %.1f%%", successRate))
	}
	lines = append(lines, fmt.Sprintf("Avg Execution Time:    %s", stats.AverageExecutionTime))
	return strings.Join(lines, "\n")
}
