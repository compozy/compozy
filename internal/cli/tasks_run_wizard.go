package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/charmtheme"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
	"github.com/spf13/cobra"
)

const (
	taskRunWizardMinWidth  = 72
	taskRunWizardMinHeight = 22
)

const (
	taskRunWizardDefaultReasoning = "medium"
	taskRunWizardKeyDown          = "down"
	taskRunWizardKeyEnter         = "enter"
	taskRunWizardKeyEsc           = "esc"
	taskRunWizardKeySpace         = "space"
	taskRunWizardKeyTab           = "tab"
)

type taskRunWizardStep int

const (
	taskRunWizardStepWorkflows taskRunWizardStep = iota
	taskRunWizardStepRuntime
	taskRunWizardStepExecution
	taskRunWizardStepReview
)

type taskRunWizardField int

const (
	taskRunWizardFieldIDE taskRunWizardField = iota
	taskRunWizardFieldModel
	taskRunWizardFieldAddDirs
	taskRunWizardFieldReasoning
	taskRunWizardFieldAccessMode
	taskRunWizardRuntimeFieldCount
)

type taskRunWizardExecutionField int

const (
	taskRunWizardFieldTimeout taskRunWizardExecutionField = iota
	taskRunWizardFieldTailLines
	taskRunWizardFieldMaxRetries
	taskRunWizardFieldRetryBackoff
	taskRunWizardFieldDryRun
	taskRunWizardFieldAutoCommit
	taskRunWizardFieldIncludeCompleted
	taskRunWizardFieldRecursive
	taskRunWizardFieldDefineRuntime
	taskRunWizardExecutionFieldCount
)

type taskRunFormInputs struct {
	manualWorkflow         string
	selectedWorkflows      []string
	ide                    string
	model                  string
	addDirs                string
	reasoningEffort        string
	accessMode             string
	timeout                string
	tailLines              string
	maxRetries             string
	retryBackoffMultiplier string
	dryRun                 bool
	autoCommit             bool
	includeCompleted       bool
	recursive              bool
	defineTaskRuntime      bool
}

type taskRunWizardChoice struct {
	Label string
	Value string
}

type taskRunWizardTextInputs struct {
	manualWorkflow textinput.Model
	model          textinput.Model
	addDirs        textinput.Model
	timeout        textinput.Model
	tailLines      textinput.Model
	maxRetries     textinput.Model
	retryBackoff   textinput.Model
}

type taskRunWizardModel struct {
	inputs          taskRunFormInputs
	workflowOptions []string
	ideOptions      []taskRunWizardChoice
	reasoningOpts   []taskRunWizardChoice
	accessModeOpts  []taskRunWizardChoice
	textInputs      taskRunWizardTextInputs
	step            taskRunWizardStep
	workflowCursor  int
	runtimeCursor   taskRunWizardField
	execCursor      taskRunWizardExecutionField
	searchActive    bool
	searchQuery     string
	showHelp        bool
	width           int
	height          int
	message         string
	submitted       bool
	canceled        bool
}

var runTaskRunWizardProgram = defaultRunTaskRunWizardProgram

func collectTaskRunFormParams(cmd *cobra.Command, state *commandState) error {
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), renderFormIntro())

	inputs := newTaskRunFormInputsFromState(state)
	if err := runTaskRunWizardProgram(cmd, state, inputs); err != nil {
		return err
	}
	if err := inputs.apply(cmd, state); err != nil {
		return err
	}

	selectedSlugs := selectedTaskRunSlugs(state)
	if inputs.defineTaskRuntime {
		if err := collectTaskRunRuntimeFormForSlugs(cmd, state, selectedSlugs); err != nil {
			return err
		}
	} else {
		clearTaskRunRuntimeRules(state)
		markInputFlagChanged(cmd, "task-runtime")
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), renderFormSuccess())
	return nil
}

func defaultRunTaskRunWizardProgram(cmd *cobra.Command, state *commandState, inputs *taskRunFormInputs) error {
	if inputs == nil {
		return nil
	}
	program := tea.NewProgram(
		newTaskRunWizardModel(state, *inputs),
		tea.WithInput(resolveTaskRunWizardInput(cmd.InOrStdin())),
		tea.WithOutput(resolveTaskRunWizardOutput(cmd.OutOrStdout())),
		tea.WithoutSignalHandler(),
	)
	result, err := program.Run()
	if err != nil {
		return fmt.Errorf("run task workflow wizard: %w", err)
	}
	typed, ok := result.(*taskRunWizardModel)
	if !ok {
		return fmt.Errorf("unexpected task workflow wizard result type %T", result)
	}
	if typed.canceled || !typed.submitted {
		return errors.New("form canceled")
	}
	*inputs = typed.inputs
	return nil
}

func resolveTaskRunWizardInput(input io.Reader) io.Reader {
	if input != nil {
		return input
	}
	return nil
}

func resolveTaskRunWizardOutput(output io.Writer) io.Writer {
	if output != nil {
		return output
	}
	return io.Discard
}

func newTaskRunFormInputsFromState(state *commandState) *taskRunFormInputs {
	inputs := &taskRunFormInputs{}
	if state == nil {
		return inputs
	}

	inputs.manualWorkflow = strings.TrimSpace(state.name)
	inputs.selectedWorkflows = initialTaskRunWorkflowSelection(state)
	inputs.ide = state.ide
	inputs.model = state.model
	if len(state.addDirs) > 0 {
		inputs.addDirs = formatAddDirInput(state.addDirs)
	}
	inputs.reasoningEffort = state.reasoningEffort
	inputs.accessMode = state.accessMode
	inputs.timeout = state.timeout
	if state.tailLines > 0 {
		inputs.tailLines = strconv.Itoa(state.tailLines)
	}
	if state.maxRetries > 0 {
		inputs.maxRetries = strconv.Itoa(state.maxRetries)
	}
	if state.retryBackoffMultiplier > 0 {
		inputs.retryBackoffMultiplier = strconv.FormatFloat(state.retryBackoffMultiplier, 'f', -1, 64)
	}
	inputs.dryRun = state.dryRun
	inputs.autoCommit = state.autoCommit
	inputs.includeCompleted = state.includeCompleted
	inputs.recursive = state.recursive
	inputs.defineTaskRuntime = len(state.taskRuntimeRules()) > 0
	return inputs
}

func initialTaskRunWorkflowSelection(state *commandState) []string {
	if state == nil {
		return nil
	}
	if slugs, err := parseTaskRunWorkflowSelection(state.multiple); err == nil && len(slugs) > 0 {
		return slugs
	}
	if name := strings.TrimSpace(state.name); name != "" {
		return []string{name}
	}
	return nil
}

func newTaskRunWizardModel(state *commandState, inputs taskRunFormInputs) *taskRunWizardModel {
	baseDir := model.TasksBaseDirForWorkspace("")
	if state != nil {
		baseDir = model.TasksBaseDirForWorkspace(state.workspaceRoot)
	}
	m := &taskRunWizardModel{
		inputs:          inputs,
		workflowOptions: listTaskSubdirs(baseDir),
		ideOptions:      taskRunWizardIDEOptions(),
		reasoningOpts:   taskRunWizardReasoningOptions(),
		accessModeOpts:  taskRunWizardAccessModeOptions(),
		width:           taskRunWizardMinWidth,
		height:          taskRunWizardMinHeight,
	}
	m.textInputs = newTaskRunWizardTextInputs(inputs)
	m.ensureChoiceDefaults()
	m.syncTextFocus()
	return m
}

func newTaskRunWizardTextInputs(inputs taskRunFormInputs) taskRunWizardTextInputs {
	return taskRunWizardTextInputs{
		manualWorkflow: newTaskRunWizardInput("workflow", inputs.manualWorkflow),
		model:          newTaskRunWizardInput("auto", inputs.model),
		addDirs:        newTaskRunWizardInput("../shared, ../docs", inputs.addDirs),
		timeout:        newTaskRunWizardInput("10m", inputs.timeout),
		tailLines:      newTaskRunWizardInput("0", inputs.tailLines),
		maxRetries:     newTaskRunWizardInput("0", inputs.maxRetries),
		retryBackoff:   newTaskRunWizardInput("1.5", inputs.retryBackoffMultiplier),
	}
}

func newTaskRunWizardInput(placeholder, value string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	input.SetValue(value)
	input.SetWidth(48)
	return input
}

func taskRunWizardIDEOptions() []taskRunWizardChoice {
	entries := agent.DriverCatalog()
	options := make([]taskRunWizardChoice, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		label := strings.TrimSpace(entry.DisplayName)
		if label == "" {
			label = entry.IDE
		}
		options = append(options, taskRunWizardChoice{Label: label, Value: entry.IDE})
	}
	return options
}

func taskRunWizardReasoningOptions() []taskRunWizardChoice {
	return []taskRunWizardChoice{
		{Label: "Low", Value: "low"},
		{Label: "Medium", Value: taskRunWizardDefaultReasoning},
		{Label: "High", Value: "high"},
		{Label: "Extra High", Value: "xhigh"},
	}
}

func taskRunWizardAccessModeOptions() []taskRunWizardChoice {
	return []taskRunWizardChoice{
		{Label: "Full", Value: core.AccessModeFull},
		{Label: "Default", Value: core.AccessModeDefault},
	}
}

func (m *taskRunWizardModel) ensureChoiceDefaults() {
	if !taskRunWizardChoiceContains(m.ideOptions, m.inputs.ide) && len(m.ideOptions) > 0 {
		m.inputs.ide = m.ideOptions[0].Value
	}
	if !taskRunWizardChoiceContains(m.reasoningOpts, m.inputs.reasoningEffort) && len(m.reasoningOpts) > 0 {
		m.inputs.reasoningEffort = taskRunWizardDefaultReasoning
	}
	if !taskRunWizardChoiceContains(m.accessModeOpts, m.inputs.accessMode) && len(m.accessModeOpts) > 0 {
		m.inputs.accessMode = core.AccessModeFull
	}
	if len(m.workflowOptions) > 0 {
		m.inputs.selectedWorkflows = filterValidTaskRunWorkflowSelections(m.inputs.selectedWorkflows, m.workflowOptions)
	}
}

func taskRunWizardChoiceContains(options []taskRunWizardChoice, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func filterValidTaskRunWorkflowSelections(selected []string, options []string) []string {
	valid := make(map[string]struct{}, len(options))
	for _, option := range options {
		valid[option] = struct{}{}
	}
	filtered := make([]string, 0, len(selected))
	for _, value := range selected {
		trimmed := strings.TrimSpace(value)
		if _, ok := valid[trimmed]; ok && !slices.Contains(filtered, trimmed) {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered
}

func (m *taskRunWizardModel) Init() tea.Cmd {
	return nil
}

func (m *taskRunWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(typed.Width, taskRunWizardMinWidth)
		m.height = max(typed.Height, taskRunWizardMinHeight)
		m.syncTextWidths()
	case tea.KeyPressMsg:
		return m.handleKey(typed)
	}
	return m, nil
}

func (m *taskRunWizardModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if m.showHelp {
		switch key {
		case "?", taskRunWizardKeyEsc, "q":
			m.showHelp = false
		}
		return m, nil
	}
	if m.searchActive {
		return m.handleSearchKey(msg)
	}

	switch key {
	case "ctrl+c", "q":
		m.canceled = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "shift+tab", taskRunWizardKeyEsc:
		m.previousStep()
		return m, nil
	}

	switch m.step {
	case taskRunWizardStepWorkflows:
		return m.handleWorkflowKey(msg)
	case taskRunWizardStepRuntime:
		return m.handleRuntimeKey(msg)
	case taskRunWizardStepExecution:
		return m.handleExecutionKey(msg)
	case taskRunWizardStepReview:
		return m.handleReviewKey(msg)
	default:
		return m, nil
	}
}

func (m *taskRunWizardModel) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case taskRunWizardKeyEsc:
		m.searchActive = false
		m.searchQuery = ""
		m.workflowCursor = 0
	case taskRunWizardKeyEnter:
		m.searchActive = false
	case "backspace":
		if m.searchQuery != "" {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.workflowCursor = min(m.workflowCursor, max(len(m.filteredWorkflowOptions())-1, 0))
		}
	default:
		value := msg.String()
		if len(value) == 1 {
			m.searchQuery += value
			m.workflowCursor = min(m.workflowCursor, max(len(m.filteredWorkflowOptions())-1, 0))
		}
	}
	return m, nil
}

func (m *taskRunWizardModel) handleWorkflowKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if len(m.workflowOptions) == 0 {
		return m.updateManualWorkflowInput(msg)
	}
	filtered := m.filteredWorkflowOptions()
	switch strings.ToLower(msg.String()) {
	case "/":
		m.searchActive = true
		m.searchQuery = ""
		m.workflowCursor = 0
	case "up", "k":
		if m.workflowCursor > 0 {
			m.workflowCursor--
		}
	case taskRunWizardKeyDown, "j":
		if m.workflowCursor < len(filtered)-1 {
			m.workflowCursor++
		}
	case "home", "g":
		m.workflowCursor = 0
	case "end":
		m.workflowCursor = max(len(filtered)-1, 0)
	case " ", taskRunWizardKeySpace:
		if len(filtered) > 0 {
			m.toggleWorkflow(filtered[m.workflowCursor])
		}
	case "a":
		m.toggleAllFilteredWorkflows(filtered)
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		m.nextStep()
	}
	return m, nil
}

func (m *taskRunWizardModel) updateManualWorkflowInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		m.inputs.manualWorkflow = strings.TrimSpace(m.textInputs.manualWorkflow.Value())
		m.nextStep()
		return m, nil
	}
	var cmd tea.Cmd
	m.textInputs.manualWorkflow, cmd = m.textInputs.manualWorkflow.Update(msg)
	m.inputs.manualWorkflow = m.textInputs.manualWorkflow.Value()
	return m, cmd
}

func (m *taskRunWizardModel) handleRuntimeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "up", "k":
		if m.runtimeCursor > 0 {
			m.runtimeCursor--
		}
		m.syncTextFocus()
	case taskRunWizardKeyDown, "j":
		if m.runtimeCursor < taskRunWizardRuntimeFieldCount-1 {
			m.runtimeCursor++
		}
		m.syncTextFocus()
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		if m.runtimeCursor == taskRunWizardRuntimeFieldCount-1 {
			m.nextStep()
		} else {
			m.runtimeCursor++
			m.syncTextFocus()
		}
	case " ", taskRunWizardKeySpace:
		m.cycleRuntimeChoice(1)
	case "left", "h":
		m.cycleRuntimeChoice(-1)
	case "right", "l":
		m.cycleRuntimeChoice(1)
	default:
		return m.updateRuntimeText(msg)
	}
	return m, nil
}

func (m *taskRunWizardModel) handleExecutionKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "up", "k":
		if m.execCursor > 0 {
			m.execCursor--
		}
		m.syncTextFocus()
	case taskRunWizardKeyDown, "j":
		if m.execCursor < taskRunWizardExecutionFieldCount-1 {
			m.execCursor++
		}
		m.syncTextFocus()
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		if m.execCursor == taskRunWizardExecutionFieldCount-1 {
			m.nextStep()
		} else {
			m.execCursor++
			m.syncTextFocus()
		}
	case " ", taskRunWizardKeySpace:
		m.toggleExecutionBool()
	default:
		return m.updateExecutionText(msg)
	}
	return m, nil
}

func (m *taskRunWizardModel) handleReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case taskRunWizardKeyEnter:
		if err := m.validateSelection(); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.submitted = true
		return m, tea.Quit
	case taskRunWizardKeyTab:
		m.step = taskRunWizardStepWorkflows
		m.syncTextFocus()
	}
	return m, nil
}

func (m *taskRunWizardModel) updateRuntimeText(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.runtimeCursor {
	case taskRunWizardFieldModel:
		m.textInputs.model, cmd = m.textInputs.model.Update(msg)
		m.inputs.model = m.textInputs.model.Value()
	case taskRunWizardFieldAddDirs:
		m.textInputs.addDirs, cmd = m.textInputs.addDirs.Update(msg)
		m.inputs.addDirs = m.textInputs.addDirs.Value()
	}
	return m, cmd
}

func (m *taskRunWizardModel) updateExecutionText(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.execCursor {
	case taskRunWizardFieldTimeout:
		m.textInputs.timeout, cmd = m.textInputs.timeout.Update(msg)
		m.inputs.timeout = m.textInputs.timeout.Value()
	case taskRunWizardFieldTailLines:
		m.textInputs.tailLines, cmd = m.textInputs.tailLines.Update(msg)
		m.inputs.tailLines = m.textInputs.tailLines.Value()
	case taskRunWizardFieldMaxRetries:
		m.textInputs.maxRetries, cmd = m.textInputs.maxRetries.Update(msg)
		m.inputs.maxRetries = m.textInputs.maxRetries.Value()
	case taskRunWizardFieldRetryBackoff:
		m.textInputs.retryBackoff, cmd = m.textInputs.retryBackoff.Update(msg)
		m.inputs.retryBackoffMultiplier = m.textInputs.retryBackoff.Value()
	}
	return m, cmd
}

func (m *taskRunWizardModel) previousStep() {
	if m.step > taskRunWizardStepWorkflows {
		m.step--
	}
	m.message = ""
	m.syncTextFocus()
}

func (m *taskRunWizardModel) nextStep() {
	if err := m.validateCurrentStep(); err != nil {
		m.message = err.Error()
		return
	}
	if m.step < taskRunWizardStepReview {
		m.step++
	}
	m.message = ""
	m.syncTextFocus()
}

func (m *taskRunWizardModel) validateCurrentStep() error {
	switch m.step {
	case taskRunWizardStepWorkflows:
		return m.validateSelection()
	case taskRunWizardStepExecution:
		return m.validateExecutionInputs()
	default:
		return nil
	}
}

func (m *taskRunWizardModel) validateSelection() error {
	selected := selectedTaskRunWizardWorkflows(m.inputs)
	if len(selected) == 0 {
		return errors.New("select at least one workflow")
	}
	return nil
}

func (m *taskRunWizardModel) validateExecutionInputs() error {
	if _, ok := parseIntInput(m.inputs.tailLines); !ok {
		return errors.New("tail lines must be a number")
	}
	if _, ok := parseIntInput(m.inputs.maxRetries); !ok {
		return errors.New("max retries must be a number")
	}
	if strings.TrimSpace(m.inputs.retryBackoffMultiplier) != "" {
		if _, ok := parseFloatInput(m.inputs.retryBackoffMultiplier); !ok {
			return errors.New("retry backoff multiplier must be greater than 0")
		}
	}
	return nil
}

func (m *taskRunWizardModel) filteredWorkflowOptions() []string {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" {
		return append([]string(nil), m.workflowOptions...)
	}
	filtered := make([]string, 0, len(m.workflowOptions))
	for _, option := range m.workflowOptions {
		if strings.Contains(strings.ToLower(option), query) {
			filtered = append(filtered, option)
		}
	}
	return filtered
}

func (m *taskRunWizardModel) toggleWorkflow(slug string) {
	trimmed := strings.TrimSpace(slug)
	if trimmed == "" {
		return
	}
	idx := slices.Index(m.inputs.selectedWorkflows, trimmed)
	if idx >= 0 {
		m.inputs.selectedWorkflows = slices.Delete(m.inputs.selectedWorkflows, idx, idx+1)
		return
	}
	m.inputs.selectedWorkflows = append(m.inputs.selectedWorkflows, trimmed)
	slices.Sort(m.inputs.selectedWorkflows)
}

func (m *taskRunWizardModel) toggleAllFilteredWorkflows(filtered []string) {
	if len(filtered) == 0 {
		return
	}
	allSelected := true
	for _, option := range filtered {
		if !slices.Contains(m.inputs.selectedWorkflows, option) {
			allSelected = false
			break
		}
	}
	for _, option := range filtered {
		idx := slices.Index(m.inputs.selectedWorkflows, option)
		if allSelected && idx >= 0 {
			m.inputs.selectedWorkflows = slices.Delete(m.inputs.selectedWorkflows, idx, idx+1)
		} else if !allSelected && idx < 0 {
			m.inputs.selectedWorkflows = append(m.inputs.selectedWorkflows, option)
		}
	}
	slices.Sort(m.inputs.selectedWorkflows)
}

func (m *taskRunWizardModel) cycleRuntimeChoice(delta int) {
	switch m.runtimeCursor {
	case taskRunWizardFieldIDE:
		m.inputs.ide = cycleTaskRunWizardChoice(m.ideOptions, m.inputs.ide, delta)
	case taskRunWizardFieldReasoning:
		m.inputs.reasoningEffort = cycleTaskRunWizardChoice(m.reasoningOpts, m.inputs.reasoningEffort, delta)
	case taskRunWizardFieldAccessMode:
		m.inputs.accessMode = cycleTaskRunWizardChoice(m.accessModeOpts, m.inputs.accessMode, delta)
	}
}

func cycleTaskRunWizardChoice(options []taskRunWizardChoice, current string, delta int) string {
	if len(options) == 0 {
		return current
	}
	idx := 0
	for i, option := range options {
		if option.Value == current {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(options)) % len(options)
	return options[idx].Value
}

func (m *taskRunWizardModel) toggleExecutionBool() {
	switch m.execCursor {
	case taskRunWizardFieldDryRun:
		m.inputs.dryRun = !m.inputs.dryRun
	case taskRunWizardFieldAutoCommit:
		m.inputs.autoCommit = !m.inputs.autoCommit
	case taskRunWizardFieldIncludeCompleted:
		m.inputs.includeCompleted = !m.inputs.includeCompleted
	case taskRunWizardFieldRecursive:
		m.inputs.recursive = !m.inputs.recursive
	case taskRunWizardFieldDefineRuntime:
		m.inputs.defineTaskRuntime = !m.inputs.defineTaskRuntime
	}
}

func (m *taskRunWizardModel) syncTextWidths() {
	width := max(24, min(60, m.width-28))
	m.textInputs.manualWorkflow.SetWidth(width)
	m.textInputs.model.SetWidth(width)
	m.textInputs.addDirs.SetWidth(width)
	m.textInputs.timeout.SetWidth(width)
	m.textInputs.tailLines.SetWidth(width)
	m.textInputs.maxRetries.SetWidth(width)
	m.textInputs.retryBackoff.SetWidth(width)
}

func (m *taskRunWizardModel) syncTextFocus() {
	m.textInputs.manualWorkflow.Blur()
	m.textInputs.model.Blur()
	m.textInputs.addDirs.Blur()
	m.textInputs.timeout.Blur()
	m.textInputs.tailLines.Blur()
	m.textInputs.maxRetries.Blur()
	m.textInputs.retryBackoff.Blur()
	if m.step == taskRunWizardStepWorkflows && len(m.workflowOptions) == 0 {
		m.textInputs.manualWorkflow.Focus()
		return
	}
	if m.step == taskRunWizardStepRuntime {
		switch m.runtimeCursor {
		case taskRunWizardFieldModel:
			m.textInputs.model.Focus()
		case taskRunWizardFieldAddDirs:
			m.textInputs.addDirs.Focus()
		}
	}
	if m.step == taskRunWizardStepExecution {
		switch m.execCursor {
		case taskRunWizardFieldTimeout:
			m.textInputs.timeout.Focus()
		case taskRunWizardFieldTailLines:
			m.textInputs.tailLines.Focus()
		case taskRunWizardFieldMaxRetries:
			m.textInputs.maxRetries.Focus()
		case taskRunWizardFieldRetryBackoff:
			m.textInputs.retryBackoff.Focus()
		}
	}
}

func (m *taskRunWizardModel) View() tea.View {
	if m.width <= 0 {
		m.width = taskRunWizardMinWidth
	}
	if m.height <= 0 {
		m.height = taskRunWizardMinHeight
	}
	contentWidth := max(40, m.width-4)
	bodyHeight := max(8, m.height-7)
	sections := []string{
		m.renderHeader(contentWidth),
		m.renderBody(contentWidth, bodyHeight),
		m.renderFooter(contentWidth),
	}
	view := strings.Join(sections, "\n")
	if m.showHelp {
		view += "\n" + m.renderHelp(contentWidth)
	}
	return tea.NewView(view)
}

func (m *taskRunWizardModel) renderHeader(width int) string {
	crumbs := []string{"Workflows", "Runtime", "Execution", "Review"}
	for i := range crumbs {
		if taskRunWizardStep(i) == m.step {
			crumbs[i] = taskRunWizardActiveStyle().Render(crumbs[i])
		} else {
			crumbs[i] = taskRunWizardMutedStyle().Render(crumbs[i])
		}
	}
	line := taskRunWizardTitleStyle().Render("Task Run Wizard") +
		"  " + strings.Join(
		crumbs,
		taskRunWizardMutedStyle().Render(" > "),
	)
	return lipgloss.NewStyle().Width(width).Render(taskRunWizardTruncate(line, width))
}

func (m *taskRunWizardModel) renderBody(width int, height int) string {
	var body string
	switch m.step {
	case taskRunWizardStepWorkflows:
		body = m.renderWorkflowStep(width, height)
	case taskRunWizardStepRuntime:
		body = m.renderRuntimeStep(width)
	case taskRunWizardStepExecution:
		body = m.renderExecutionStep()
	case taskRunWizardStepReview:
		body = m.renderReviewStep(width)
	}
	return taskRunWizardBoxStyle(width).Height(height).Render(taskRunWizardClampLines(body, height, width-4))
}

func (m *taskRunWizardModel) renderWorkflowStep(width int, height int) string {
	if len(m.workflowOptions) == 0 {
		return strings.Join([]string{
			taskRunWizardSubtitleStyle().Render("No workflow directories were discovered; enter a slug manually."),
			"",
			m.renderField("Workflow", m.textInputs.manualWorkflow.View(), true),
		}, "\n")
	}
	filtered := m.filteredWorkflowOptions()
	lines := []string{
		taskRunWizardSubtitleStyle().Render("Select one or more workflows. Press / to filter and Space to toggle."),
	}
	if m.searchActive || m.searchQuery != "" {
		lines = append(lines, taskRunWizardMutedStyle().Render("Filter: ")+m.searchQuery)
	}
	visibleRows := max(3, height-5)
	start := 0
	if m.workflowCursor >= visibleRows {
		start = m.workflowCursor - visibleRows + 1
	}
	end := min(start+visibleRows, len(filtered))
	for idx := start; idx < end; idx++ {
		option := filtered[idx]
		cursor := "  "
		if idx == m.workflowCursor {
			cursor = taskRunWizardActiveStyle().Render("▸ ")
		}
		mark := "[ ]"
		if slices.Contains(m.inputs.selectedWorkflows, option) {
			mark = taskRunWizardActiveStyle().Render("[x]")
		}
		lines = append(lines, taskRunWizardTruncate(cursor+mark+" "+option, width-6))
	}
	if len(filtered) == 0 {
		lines = append(lines, taskRunWizardMutedStyle().Render("No workflows match the filter."))
	}
	selection := fmt.Sprintf("Selected: %s", formatTaskRunWizardSelection(m.inputs.selectedWorkflows))
	lines = append(lines, "", taskRunWizardMutedStyle().Render(selection))
	return strings.Join(lines, "\n")
}

func (m *taskRunWizardModel) renderRuntimeStep(width int) string {
	return strings.Join([]string{
		m.renderField(
			"Provider / IDE",
			taskRunWizardChoiceLabel(m.ideOptions, m.inputs.ide),
			m.runtimeCursor == taskRunWizardFieldIDE,
		),
		m.renderField("Model", m.textInputs.model.View(), m.runtimeCursor == taskRunWizardFieldModel),
		m.renderField("Additional dirs", m.textInputs.addDirs.View(), m.runtimeCursor == taskRunWizardFieldAddDirs),
		m.renderField(
			"Reasoning effort",
			taskRunWizardChoiceLabel(m.reasoningOpts, m.inputs.reasoningEffort),
			m.runtimeCursor == taskRunWizardFieldReasoning,
		),
		m.renderField(
			"Access mode",
			taskRunWizardChoiceLabel(m.accessModeOpts, m.inputs.accessMode),
			m.runtimeCursor == taskRunWizardFieldAccessMode,
		),
		"",
		taskRunWizardMutedStyle().Render(taskRunWizardTruncate(
			"Use left/right or Space to cycle select fields. Text fields edit in place.",
			width-6,
		)),
	}, "\n")
}

func (m *taskRunWizardModel) renderExecutionStep() string {
	return strings.Join([]string{
		m.renderField("Activity timeout", m.textInputs.timeout.View(), m.execCursor == taskRunWizardFieldTimeout),
		m.renderField("Tail lines", m.textInputs.tailLines.View(), m.execCursor == taskRunWizardFieldTailLines),
		m.renderField("Max retries", m.textInputs.maxRetries.View(), m.execCursor == taskRunWizardFieldMaxRetries),
		m.renderField(
			"Retry backoff",
			m.textInputs.retryBackoff.View(),
			m.execCursor == taskRunWizardFieldRetryBackoff,
		),
		m.renderField("Dry run", taskRunWizardBool(m.inputs.dryRun), m.execCursor == taskRunWizardFieldDryRun),
		m.renderField(
			"Auto commit",
			taskRunWizardBool(m.inputs.autoCommit),
			m.execCursor == taskRunWizardFieldAutoCommit,
		),
		m.renderField(
			"Include completed",
			taskRunWizardBool(m.inputs.includeCompleted),
			m.execCursor == taskRunWizardFieldIncludeCompleted,
		),
		m.renderField("Recursive", taskRunWizardBool(m.inputs.recursive), m.execCursor == taskRunWizardFieldRecursive),
		m.renderField(
			"Runtime per task",
			taskRunWizardBool(m.inputs.defineTaskRuntime),
			m.execCursor == taskRunWizardFieldDefineRuntime,
		),
	}, "\n")
}

func (m *taskRunWizardModel) renderReviewStep(width int) string {
	selected := selectedTaskRunWizardWorkflows(m.inputs)
	lines := []string{
		taskRunWizardSubtitleStyle().Render("Review selections before starting the daemon run."),
		m.renderSummary("Workflows", formatTaskRunWizardSelection(selected), width),
		m.renderSummary("Runtime", strings.Join([]string{
			taskRunWizardChoiceLabel(m.ideOptions, m.inputs.ide),
			"model=" + taskRunWizardBlank(m.inputs.model),
			"reasoning=" + taskRunWizardBlank(m.inputs.reasoningEffort),
		}, "  "), width),
		m.renderSummary("Execution", strings.Join([]string{
			"timeout=" + taskRunWizardBlank(m.inputs.timeout),
			"dry-run=" + strconv.FormatBool(m.inputs.dryRun),
			"auto-commit=" + strconv.FormatBool(m.inputs.autoCommit),
			"per-task=" + strconv.FormatBool(m.inputs.defineTaskRuntime),
		}, "  "), width),
		"",
		taskRunWizardActiveStyle().Render("Press Enter to continue."),
	}
	return strings.Join(lines, "\n")
}

func (m *taskRunWizardModel) renderSummary(label string, value string, width int) string {
	return taskRunWizardMutedStyle().Render(label+": ") + taskRunWizardTruncate(value, width-len(label)-8)
}

func (m *taskRunWizardModel) renderField(label string, value string, active bool) string {
	prefix := "  "
	style := taskRunWizardMutedStyle()
	if active {
		prefix = taskRunWizardActiveStyle().Render("▸ ")
		style = taskRunWizardActiveStyle()
	}
	return prefix + style.Render(label) + "  " + value
}

func (m *taskRunWizardModel) renderFooter(width int) string {
	msg := "[q]uit [?]help [Tab/Enter]next [Shift+Tab/Esc]back"
	if m.step == taskRunWizardStepWorkflows && len(m.workflowOptions) > 0 {
		msg = "[Space]toggle [a]all [/]filter [Enter]next [q]uit"
	}
	if m.message != "" {
		msg = taskRunWizardErrorStyle().Render(m.message) + "  " + msg
	}
	return taskRunWizardMutedStyle().Render(taskRunWizardTruncate(msg, width))
}

func (m *taskRunWizardModel) renderHelp(width int) string {
	lines := []string{
		"Keyboard",
		"  j/k or arrows: move",
		"  Space: toggle workflow or boolean/cycle select",
		"  /: filter workflows",
		"  Enter/Tab: advance",
		"  Shift+Tab/Esc: back",
		"  q/Ctrl+C: cancel",
	}
	return taskRunWizardBoxStyle(min(width, 64)).Render(strings.Join(lines, "\n"))
}

func taskRunWizardChoiceLabel(options []taskRunWizardChoice, value string) string {
	for _, option := range options {
		if option.Value == value {
			return option.Label
		}
	}
	return taskRunWizardBlank(value)
}

func taskRunWizardBool(value bool) string {
	if value {
		return taskRunWizardActiveStyle().Render("on")
	}
	return taskRunWizardMutedStyle().Render("off")
}

func taskRunWizardBlank(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "inherit"
	}
	return trimmed
}

func selectedTaskRunWizardWorkflows(inputs taskRunFormInputs) []string {
	if len(inputs.selectedWorkflows) > 0 {
		return append([]string(nil), inputs.selectedWorkflows...)
	}
	manual := strings.TrimSpace(inputs.manualWorkflow)
	if manual == "" {
		return nil
	}
	return []string{manual}
}

func formatTaskRunWizardSelection(slugs []string) string {
	if len(slugs) == 0 {
		return "none"
	}
	return strings.Join(slugs, ", ")
}

func taskRunWizardClampLines(value string, height int, width int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = taskRunWizardTruncate(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

func taskRunWizardTruncate(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func taskRunWizardTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorBrand)
}

func taskRunWizardSubtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(charmtheme.ColorFgBright)
}

func taskRunWizardActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorAccent)
}

func taskRunWizardMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(charmtheme.ColorMuted)
}

func taskRunWizardErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorError)
}

func taskRunWizardBoxStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(max(1, width-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(charmtheme.ColorBorder).
		Padding(0, 1)
}

func (inputs *taskRunFormInputs) apply(cmd *cobra.Command, state *commandState) error {
	if inputs == nil || state == nil {
		return nil
	}
	if err := inputs.applyWorkflowSelection(cmd, state); err != nil {
		return err
	}
	applyInput(cmd, "ide", inputs.ide, passThroughInput[string], func(value string) { state.ide = value })
	applyInput(cmd, "model", inputs.model, passThroughInput[string], func(value string) { state.model = value })
	applyInput(cmd, "add-dir", inputs.addDirs, parseStringSliceInput, func(value []string) { state.addDirs = value })
	applyInput(cmd, "reasoning-effort", inputs.reasoningEffort, passThroughInput[string], func(value string) {
		state.reasoningEffort = value
	})
	applyInput(
		cmd,
		"access-mode",
		inputs.accessMode,
		passThroughInput[string],
		func(value string) { state.accessMode = value },
	)
	applyInput(cmd, "timeout", inputs.timeout, passThroughInput[string], func(value string) { state.timeout = value })
	applyInput(cmd, "tail-lines", inputs.tailLines, parseIntInput, func(value int) { state.tailLines = value })
	applyInput(cmd, "max-retries", inputs.maxRetries, parseIntInput, func(value int) { state.maxRetries = value })
	applyInput(cmd, "retry-backoff-multiplier", inputs.retryBackoffMultiplier, parseFloatInput, func(value float64) {
		state.retryBackoffMultiplier = value
	})
	applyInput(cmd, "dry-run", inputs.dryRun, passThroughInput[bool], func(value bool) { state.dryRun = value })
	applyInput(
		cmd,
		"auto-commit",
		inputs.autoCommit,
		passThroughInput[bool],
		func(value bool) { state.autoCommit = value },
	)
	applyInput(cmd, "include-completed", inputs.includeCompleted, passThroughInput[bool], func(value bool) {
		state.includeCompleted = value
	})
	applyInput(cmd, "recursive", inputs.recursive, passThroughInput[bool], func(value bool) { state.recursive = value })
	return nil
}

func (inputs *taskRunFormInputs) applyWorkflowSelection(cmd *cobra.Command, state *commandState) error {
	selected := selectedTaskRunWizardWorkflows(*inputs)
	parsed, err := parseTaskRunWorkflowSelection(strings.Join(selected, ","))
	if err != nil {
		return fmt.Errorf("workflow selection: %w", err)
	}
	if len(parsed) == 0 {
		return errors.New("workflow selection: select at least one workflow")
	}

	if len(parsed) == 1 {
		state.name = parsed[0]
		state.multiple = ""
		markInputFlagChanged(cmd, "name")
		return nil
	}
	state.name = ""
	state.multiple = strings.Join(parsed, ",")
	markInputFlagChanged(cmd, "multiple")
	return nil
}

func parseTaskRunWorkflowSelection(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	return taskscore.ParseCommaSeparatedSlugs(trimmed)
}

func selectedTaskRunSlugs(state *commandState) []string {
	if state == nil {
		return nil
	}
	if slugs, err := parseTaskRunWorkflowSelection(state.multiple); err == nil && len(slugs) > 0 {
		return slugs
	}
	if name := strings.TrimSpace(state.name); name != "" {
		return []string{name}
	}
	return nil
}

func parseFloatInput(value string) (float64, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, true
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}
