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
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/spf13/cobra"
)

const (
	taskRunWizardMinWidth  = 72
	taskRunWizardMinHeight = 22
)

const (
	taskRunWizardKindType = "type"
	taskRunWizardKindTask = "task"
)

const (
	taskRunWizardDefaultReasoning = "medium"
	taskRunWizardKeyDown          = "down"
	taskRunWizardKeyEnter         = "enter"
	taskRunWizardKeyEsc           = "esc"
	taskRunWizardKeyEnd           = "end"
	taskRunWizardKeyHome          = "home"
	taskRunWizardKeyLeft          = "left"
	taskRunWizardKeyRight         = "right"
	taskRunWizardKeySpace         = "space"
	taskRunWizardKeyTab           = "tab"
)

type taskRunWizardStep int

const (
	taskRunWizardStepWorkflows taskRunWizardStep = iota
	taskRunWizardStepRuntime
	taskRunWizardStepExecution
	taskRunWizardStepOverrides
	taskRunWizardStepReview
)

type taskRunWizardWorkflowFocus int

const (
	taskRunWizardWorkflowFocusList taskRunWizardWorkflowFocus = iota
	taskRunWizardWorkflowFocusOrder
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
	taskRunWizardFieldRecoveryEnabled
	taskRunWizardFieldRecoveryIDE
	taskRunWizardFieldRecoveryModel
	taskRunWizardFieldRecoveryReasoning
	taskRunWizardFieldDefineRuntime
	taskRunWizardExecutionFieldCount
)

type taskRunWizardOverrideFocus int

const (
	taskRunWizardOverrideFocusTargets taskRunWizardOverrideFocus = iota
	taskRunWizardOverrideFocusEditor
)

type taskRunWizardOverrideEditorField int

const (
	taskRunWizardOverrideFieldIDE taskRunWizardOverrideEditorField = iota
	taskRunWizardOverrideFieldModel
	taskRunWizardOverrideFieldReasoning
	taskRunWizardOverrideFieldCount
)

type taskRunWizardOverrideTargetKind int

const (
	taskRunWizardOverrideTargetType taskRunWizardOverrideTargetKind = iota
	taskRunWizardOverrideTargetTask
)

type taskRunWizardOverrideTarget struct {
	Kind     taskRunWizardOverrideTargetKind
	Key      string
	Workflow string
	Label    string
}

type taskRunWizardOverridesLoadedMsg struct {
	Key  string
	Form *taskRunRuntimeForm
	Err  error
}

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
	recoveryEnabled        bool
	recoveryIDE            string
	recoveryModel          string
	recoveryReasoning      string
	defineTaskRuntime      bool
	taskRuntimeRules       []model.TaskRuntimeRule
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
	recoveryModel  textinput.Model
}

type taskRunWizardModel struct {
	state                  *commandState
	inputs                 taskRunFormInputs
	workflowOptions        []string
	ideOptions             []taskRunWizardChoice
	reasoningOpts          []taskRunWizardChoice
	accessModeOpts         []taskRunWizardChoice
	textInputs             taskRunWizardTextInputs
	step                   taskRunWizardStep
	workflowCursor         int
	workflowFocus          taskRunWizardWorkflowFocus
	orderCursor            int
	runtimeCursor          taskRunWizardField
	execCursor             taskRunWizardExecutionField
	runtimeForm            *taskRunRuntimeForm
	overridesLoading       bool
	overridesLoadKey       string
	overridesLoadErr       string
	overrideFocus          taskRunWizardOverrideFocus
	overrideWorkflowCursor int
	overrideTargetCursor   int
	overrideEditorCursor   taskRunWizardOverrideEditorField
	overrideModelInput     textinput.Model
	searchActive           bool
	searchQuery            string
	showHelp               bool
	width                  int
	height                 int
	message                string
	submitted              bool
	canceled               bool
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
	typed.syncTaskRuntimeRulesFromForm()
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
	inputs.recoveryEnabled = state.recoveryEnabled
	inputs.recoveryIDE = state.recoveryIDE
	inputs.recoveryModel = state.recoveryModel
	inputs.recoveryReasoning = state.recoveryReasoningEffort
	inputs.taskRuntimeRules = model.CloneTaskRuntimeRules(state.taskRuntimeRules())
	inputs.defineTaskRuntime = len(inputs.taskRuntimeRules) > 0
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
		state:           state,
		inputs:          inputs,
		workflowOptions: listTaskSubdirs(baseDir),
		ideOptions:      taskRunWizardIDEOptions(),
		reasoningOpts:   taskRunWizardReasoningOptions(),
		accessModeOpts:  taskRunWizardAccessModeOptions(),
		width:           taskRunWizardMinWidth,
		height:          taskRunWizardMinHeight,
	}
	m.textInputs = newTaskRunWizardTextInputs(inputs)
	m.overrideModelInput = newTaskRunWizardInput("inherit default", "")
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
		recoveryModel:  newTaskRunWizardInput(workspace.DefaultRecoveryModel, inputs.recoveryModel),
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
	if !taskRunWizardChoiceContains(m.ideOptions, m.inputs.recoveryIDE) && len(m.ideOptions) > 0 {
		m.inputs.recoveryIDE = workspace.DefaultRecoveryIDE
	}
	if strings.TrimSpace(m.inputs.recoveryModel) == "" {
		m.inputs.recoveryModel = workspace.DefaultRecoveryModel
		m.textInputs.recoveryModel.SetValue(m.inputs.recoveryModel)
	}
	if !taskRunWizardChoiceContains(m.reasoningOpts, m.inputs.recoveryReasoning) && len(m.reasoningOpts) > 0 {
		m.inputs.recoveryReasoning = workspace.DefaultRecoveryReasoningEffort
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
	case taskRunWizardOverridesLoadedMsg:
		m.handleOverridesLoaded(typed)
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

	textInputFocused := m.textInputFocused()
	switch key {
	case "ctrl+c":
		m.canceled = true
		return m, tea.Quit
	case "q":
		if !textInputFocused {
			m.canceled = true
			return m, tea.Quit
		}
	case "?":
		if !textInputFocused {
			m.showHelp = true
			return m, nil
		}
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
	case taskRunWizardStepOverrides:
		return m.handleOverridesKey(msg)
	case taskRunWizardStepReview:
		return m.handleReviewKey(msg)
	default:
		return m, nil
	}
}

func (m *taskRunWizardModel) textInputFocused() bool {
	return m.textInputs.manualWorkflow.Focused() ||
		m.textInputs.model.Focused() ||
		m.textInputs.addDirs.Focused() ||
		m.textInputs.timeout.Focused() ||
		m.textInputs.tailLines.Focused() ||
		m.textInputs.maxRetries.Focused() ||
		m.textInputs.retryBackoff.Focused() ||
		m.overrideModelInput.Focused()
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
	key := strings.ToLower(msg.String())
	originalKey := msg.String()
	if key == taskRunWizardKeyRight || key == "l" {
		if len(m.inputs.selectedWorkflows) > 0 {
			m.workflowFocus = taskRunWizardWorkflowFocusOrder
			m.clampOrderCursor()
		}
		return m, nil
	}
	if key == taskRunWizardKeyLeft || key == "h" {
		m.workflowFocus = taskRunWizardWorkflowFocusList
		return m, nil
	}
	if m.workflowFocus == taskRunWizardWorkflowFocusOrder {
		return m.handleWorkflowOrderKey(key, originalKey)
	}
	return m.handleWorkflowListKey(filtered, key, originalKey)
}

func (m *taskRunWizardModel) handleWorkflowOrderKey(key string, originalKey string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.orderCursor > 0 {
			m.orderCursor--
		}
	case taskRunWizardKeyDown, "j":
		if m.orderCursor < len(m.inputs.selectedWorkflows)-1 {
			m.orderCursor++
		}
	case taskRunWizardKeyHome, "g":
		m.orderCursor = 0
	case taskRunWizardKeyEnd:
		m.orderCursor = max(len(m.inputs.selectedWorkflows)-1, 0)
	case "u":
		m.moveSelectedWorkflow(-1)
	case "d":
		m.moveSelectedWorkflow(1)
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		return m, m.nextStep()
	}
	if originalKey == "G" {
		m.orderCursor = max(len(m.inputs.selectedWorkflows)-1, 0)
	}
	return m, nil
}

func (m *taskRunWizardModel) handleWorkflowListKey(
	filtered []string,
	key string,
	originalKey string,
) (tea.Model, tea.Cmd) {
	switch key {
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
	case taskRunWizardKeyHome, "g":
		m.workflowCursor = 0
	case taskRunWizardKeyEnd:
		m.workflowCursor = max(len(filtered)-1, 0)
	case " ", taskRunWizardKeySpace:
		if len(filtered) > 0 {
			m.toggleWorkflow(filtered[m.workflowCursor])
		}
	case "a":
		m.toggleAllFilteredWorkflows(filtered)
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		return m, m.nextStep()
	}
	if originalKey == "G" {
		m.workflowCursor = max(len(filtered)-1, 0)
	}
	return m, nil
}

func (m *taskRunWizardModel) updateManualWorkflowInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		m.inputs.manualWorkflow = strings.TrimSpace(m.textInputs.manualWorkflow.Value())
		return m, m.nextStep()
	}
	var cmd tea.Cmd
	m.textInputs.manualWorkflow, cmd = m.textInputs.manualWorkflow.Update(msg)
	m.inputs.manualWorkflow = m.textInputs.manualWorkflow.Value()
	return m, cmd
}

func (m *taskRunWizardModel) handleRuntimeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if m.runtimeTextFieldFocused() && !taskRunWizardTextFieldNavigationKey(key) {
		return m.updateRuntimeText(msg)
	}
	switch key {
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
			return m, m.nextStep()
		}
		m.runtimeCursor++
		m.syncTextFocus()
	case " ", taskRunWizardKeySpace:
		m.cycleRuntimeChoice(1)
	case taskRunWizardKeyLeft, "h":
		m.cycleRuntimeChoice(-1)
	case taskRunWizardKeyRight, "l":
		m.cycleRuntimeChoice(1)
	default:
		return m.updateRuntimeText(msg)
	}
	return m, nil
}

func (m *taskRunWizardModel) handleExecutionKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if m.executionTextFieldFocused() && !taskRunWizardTextFieldNavigationKey(key) {
		return m.updateExecutionText(msg)
	}
	fields := m.executionFields()
	switch key {
	case "up", "k":
		idx := slices.Index(fields, m.execCursor)
		if idx > 0 {
			m.execCursor = fields[idx-1]
		}
		m.syncTextFocus()
	case taskRunWizardKeyDown, "j":
		idx := slices.Index(fields, m.execCursor)
		if idx >= 0 && idx < len(fields)-1 {
			m.execCursor = fields[idx+1]
		}
		m.syncTextFocus()
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		idx := slices.Index(fields, m.execCursor)
		if idx == len(fields)-1 {
			return m, m.nextStep()
		}
		if idx >= 0 {
			m.execCursor = fields[idx+1]
		}
		m.syncTextFocus()
	case " ", taskRunWizardKeySpace:
		m.toggleExecutionBool()
	case taskRunWizardKeyLeft, "h":
		m.cycleExecutionChoice(-1)
	case taskRunWizardKeyRight, "l":
		m.cycleExecutionChoice(1)
	default:
		return m.updateExecutionText(msg)
	}
	return m, nil
}

func (m *taskRunWizardModel) executionFields() []taskRunWizardExecutionField {
	fields := []taskRunWizardExecutionField{
		taskRunWizardFieldTimeout,
		taskRunWizardFieldTailLines,
		taskRunWizardFieldMaxRetries,
		taskRunWizardFieldRetryBackoff,
		taskRunWizardFieldDryRun,
		taskRunWizardFieldAutoCommit,
		taskRunWizardFieldIncludeCompleted,
		taskRunWizardFieldRecursive,
		taskRunWizardFieldRecoveryEnabled,
	}
	if m.inputs.recoveryEnabled {
		fields = append(fields,
			taskRunWizardFieldRecoveryIDE,
			taskRunWizardFieldRecoveryModel,
			taskRunWizardFieldRecoveryReasoning,
		)
	}
	fields = append(fields, taskRunWizardFieldDefineRuntime)
	return fields
}

func (m *taskRunWizardModel) clampExecutionCursor() {
	fields := m.executionFields()
	if len(fields) == 0 {
		m.execCursor = taskRunWizardFieldTimeout
		return
	}
	if slices.Contains(fields, m.execCursor) {
		return
	}
	m.execCursor = fields[len(fields)-1]
}

func (m *taskRunWizardModel) handleOverridesKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if m.overridesLoading {
		return m, nil
	}
	if m.runtimeForm == nil {
		if m.overridesLoadErr != "" {
			return m, nil
		}
		switch key {
		case taskRunWizardKeyEnter, taskRunWizardKeyTab:
			return m, m.nextStep()
		}
		return m, nil
	}
	if m.overrideTextFieldFocused() && !taskRunWizardTextFieldNavigationKey(key) {
		return m.handleOverrideEditorKey(msg)
	}
	switch key {
	case "[":
		m.cycleOverrideWorkflow(-1)
		return m, nil
	case "]":
		m.cycleOverrideWorkflow(1)
		return m, nil
	case taskRunWizardKeyLeft, "h":
		m.overrideFocus = taskRunWizardOverrideFocusTargets
		m.syncTextFocus()
		return m, nil
	case taskRunWizardKeyRight, "l":
		if m.overrideFocus == taskRunWizardOverrideFocusTargets {
			if target, ok := m.currentOverrideTarget(); ok {
				if !m.overrideTargetSelected(target) {
					m.toggleOverrideTarget(target)
				}
				m.overrideFocus = taskRunWizardOverrideFocusEditor
				m.syncOverrideModelInputFromEditor()
				m.syncTextFocus()
			}
			return m, nil
		}
	}
	if m.overrideFocus == taskRunWizardOverrideFocusEditor {
		return m.handleOverrideEditorKey(msg)
	}
	return m.handleOverrideTargetKey(msg)
}

func (m *taskRunWizardModel) handleOverrideTargetKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	targets := m.overrideTargets()
	if len(targets) == 0 {
		return m, nil
	}
	switch strings.ToLower(msg.String()) {
	case "up", "k":
		if m.overrideTargetCursor > 0 {
			m.overrideTargetCursor--
			m.syncOverrideModelInputFromEditor()
		}
	case taskRunWizardKeyDown, "j":
		if m.overrideTargetCursor < len(targets)-1 {
			m.overrideTargetCursor++
			m.syncOverrideModelInputFromEditor()
		}
	case taskRunWizardKeyHome, "g":
		m.overrideTargetCursor = 0
		m.syncOverrideModelInputFromEditor()
	case taskRunWizardKeyEnd:
		m.overrideTargetCursor = len(targets) - 1
		m.syncOverrideModelInputFromEditor()
	case " ", taskRunWizardKeySpace:
		m.toggleOverrideTarget(targets[m.overrideTargetCursor])
		m.syncOverrideModelInputFromEditor()
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		return m, m.nextStep()
	}
	return m, nil
}

func (m *taskRunWizardModel) handleOverrideEditorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if m.overrideTextFieldFocused() && !taskRunWizardTextFieldNavigationKey(key) {
		return m.updateOverrideText(msg)
	}
	switch key {
	case "up", "k":
		if m.overrideEditorCursor > 0 {
			m.overrideEditorCursor--
		}
		m.syncTextFocus()
	case taskRunWizardKeyDown, "j":
		if m.overrideEditorCursor < taskRunWizardOverrideFieldCount-1 {
			m.overrideEditorCursor++
		}
		m.syncTextFocus()
	case " ", taskRunWizardKeySpace, taskRunWizardKeyRight, "l":
		m.cycleOverrideChoice(1)
	case taskRunWizardKeyLeft, "h":
		m.cycleOverrideChoice(-1)
	case taskRunWizardKeyEnter, taskRunWizardKeyTab:
		if m.overrideEditorCursor == taskRunWizardOverrideFieldCount-1 {
			return m, m.nextStep()
		}
		m.overrideEditorCursor++
		m.syncTextFocus()
	default:
		return m.updateOverrideText(msg)
	}
	return m, nil
}

func taskRunWizardTextFieldNavigationKey(key string) bool {
	switch key {
	case "up", "down", "enter", "tab":
		return true
	default:
		return false
	}
}

func (m *taskRunWizardModel) runtimeTextFieldFocused() bool {
	return m.step == taskRunWizardStepRuntime &&
		(m.runtimeCursor == taskRunWizardFieldModel || m.runtimeCursor == taskRunWizardFieldAddDirs)
}

func (m *taskRunWizardModel) executionTextFieldFocused() bool {
	if m.step != taskRunWizardStepExecution {
		return false
	}
	switch m.execCursor {
	case taskRunWizardFieldTimeout,
		taskRunWizardFieldTailLines,
		taskRunWizardFieldMaxRetries,
		taskRunWizardFieldRetryBackoff,
		taskRunWizardFieldRecoveryModel:
		return true
	default:
		return false
	}
}

func (m *taskRunWizardModel) overrideTextFieldFocused() bool {
	return m.step == taskRunWizardStepOverrides &&
		m.overrideFocus == taskRunWizardOverrideFocusEditor &&
		m.overrideEditorCursor == taskRunWizardOverrideFieldModel
}

func (m *taskRunWizardModel) handleReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case taskRunWizardKeyEnter:
		if err := m.validateSelection(); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.syncTaskRuntimeRulesFromForm()
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
	case taskRunWizardFieldRecoveryModel:
		m.textInputs.recoveryModel, cmd = m.textInputs.recoveryModel.Update(msg)
		m.inputs.recoveryModel = m.textInputs.recoveryModel.Value()
	}
	return m, cmd
}

func (m *taskRunWizardModel) updateOverrideText(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.overrideEditorCursor != taskRunWizardOverrideFieldModel {
		return m, nil
	}
	editor := m.currentOverrideEditor()
	if editor == nil {
		return m, nil
	}
	var cmd tea.Cmd
	m.overrideModelInput, cmd = m.overrideModelInput.Update(msg)
	editor.Model = m.overrideModelInput.Value()
	return m, cmd
}

func (m *taskRunWizardModel) loadOverridesCmd() tea.Cmd {
	slugs := selectedTaskRunWizardWorkflows(m.inputs)
	key := m.overrideLoadKey(slugs)
	if m.runtimeForm != nil && m.overridesLoadKey == key && m.overridesLoadErr == "" {
		m.overridesLoading = false
		m.syncOverrideModelInputFromEditor()
		return nil
	}
	m.overridesLoading = true
	m.overridesLoadKey = key
	m.overridesLoadErr = ""
	m.runtimeForm = nil
	state := m.runtimeFormState()
	return func() tea.Msg {
		form, err := newTaskRunRuntimeFormForSlugs(state, slugs)
		return taskRunWizardOverridesLoadedMsg{Key: key, Form: form, Err: err}
	}
}

func (m *taskRunWizardModel) handleOverridesLoaded(msg taskRunWizardOverridesLoadedMsg) {
	if msg.Key != m.overridesLoadKey {
		return
	}
	m.overridesLoading = false
	if msg.Err != nil {
		m.overridesLoadErr = msg.Err.Error()
		m.runtimeForm = nil
		return
	}
	m.runtimeForm = msg.Form
	m.overrideFocus = taskRunWizardOverrideFocusTargets
	m.overrideWorkflowCursor = 0
	m.overrideTargetCursor = 0
	m.overrideEditorCursor = taskRunWizardOverrideFieldIDE
	m.clampOverrideCursors()
	m.syncOverrideModelInputFromEditor()
	m.syncTextFocus()
}

func (m *taskRunWizardModel) overrideLoadKey(slugs []string) string {
	parts := []string{
		strings.Join(slugs, "\x00"),
		strconv.FormatBool(m.inputs.includeCompleted),
		strconv.FormatBool(m.inputs.recursive),
		strings.TrimSpace(m.inputs.ide),
		strings.TrimSpace(m.inputs.model),
		strings.TrimSpace(m.inputs.reasoningEffort),
	}
	return strings.Join(parts, "|")
}

func (m *taskRunWizardModel) runtimeFormState() *commandState {
	state := &commandState{kind: commandKindTasksRun, mode: core.ModePRDTasks}
	if m.state != nil {
		state.workspaceRoot = m.state.workspaceRoot
		state.tasksDir = m.state.tasksDir
		state.configuredTaskRuntimeRules = model.CloneTaskRuntimeRules(m.state.configuredTaskRuntimeRules)
	}
	state.ide = m.inputs.ide
	state.model = m.inputs.model
	state.reasoningEffort = m.inputs.reasoningEffort
	state.includeCompleted = m.inputs.includeCompleted
	state.recursive = m.inputs.recursive
	state.executionTaskRuntimeRules = model.CloneTaskRuntimeRules(m.inputs.taskRuntimeRules)
	state.replaceConfiguredTaskRunRules = true
	return state
}

func (m *taskRunWizardModel) syncTaskRuntimeRulesFromForm() {
	if !m.inputs.defineTaskRuntime {
		m.inputs.taskRuntimeRules = nil
		return
	}
	if m.runtimeForm == nil {
		if len(m.inputs.taskRuntimeRules) == 0 {
			m.inputs.taskRuntimeRules = nil
		}
		return
	}
	state := m.runtimeFormState()
	state.executionTaskRuntimeRules = nil
	m.runtimeForm.apply(state)
	m.inputs.taskRuntimeRules = model.CloneTaskRuntimeRules(state.executionTaskRuntimeRules)
}

func (m *taskRunWizardModel) overrideTargets() []taskRunWizardOverrideTarget {
	if m.runtimeForm == nil {
		return nil
	}
	workflow := m.activeOverrideWorkflow()
	targets := make([]taskRunWizardOverrideTarget, 0, len(m.runtimeForm.typeOptions)+len(m.runtimeForm.taskOptions))
	for _, option := range m.runtimeForm.typeOptions {
		if !taskRunWizardOverrideMatchesWorkflow(option.Workflow, workflow) {
			continue
		}
		targets = append(targets, taskRunWizardOverrideTarget{
			Kind:     taskRunWizardOverrideTargetType,
			Key:      option.Key,
			Workflow: option.Workflow,
			Label:    option.Label,
		})
	}
	for _, option := range m.runtimeForm.taskOptions {
		if !taskRunWizardOverrideMatchesWorkflow(option.Workflow, workflow) {
			continue
		}
		targets = append(targets, taskRunWizardOverrideTarget{
			Kind:     taskRunWizardOverrideTargetTask,
			Key:      option.Key,
			Workflow: option.Workflow,
			Label:    option.Label,
		})
	}
	return targets
}

func taskRunWizardOverrideMatchesWorkflow(optionWorkflow, activeWorkflow string) bool {
	trimmedOption := strings.TrimSpace(optionWorkflow)
	if trimmedOption == "" {
		return true
	}
	return trimmedOption == strings.TrimSpace(activeWorkflow)
}

func (m *taskRunWizardModel) activeOverrideWorkflow() string {
	slugs := selectedTaskRunWizardWorkflows(m.inputs)
	if len(slugs) == 0 {
		return ""
	}
	idx := min(max(m.overrideWorkflowCursor, 0), len(slugs)-1)
	return slugs[idx]
}

func (m *taskRunWizardModel) cycleOverrideWorkflow(delta int) {
	slugs := selectedTaskRunWizardWorkflows(m.inputs)
	if len(slugs) == 0 {
		return
	}
	m.overrideWorkflowCursor = (m.overrideWorkflowCursor + delta + len(slugs)) % len(slugs)
	m.overrideTargetCursor = 0
	m.overrideFocus = taskRunWizardOverrideFocusTargets
	m.clampOverrideCursors()
	m.syncOverrideModelInputFromEditor()
	m.syncTextFocus()
}

func (m *taskRunWizardModel) currentOverrideTarget() (taskRunWizardOverrideTarget, bool) {
	targets := m.overrideTargets()
	if len(targets) == 0 {
		return taskRunWizardOverrideTarget{}, false
	}
	idx := min(max(m.overrideTargetCursor, 0), len(targets)-1)
	return targets[idx], true
}

func (m *taskRunWizardModel) clampOverrideCursors() {
	slugs := selectedTaskRunWizardWorkflows(m.inputs)
	if len(slugs) == 0 {
		m.overrideWorkflowCursor = 0
		m.overrideTargetCursor = 0
		return
	}
	m.overrideWorkflowCursor = min(max(m.overrideWorkflowCursor, 0), len(slugs)-1)
	targets := m.overrideTargets()
	if len(targets) == 0 {
		m.overrideTargetCursor = 0
		return
	}
	m.overrideTargetCursor = min(max(m.overrideTargetCursor, 0), len(targets)-1)
}

func (m *taskRunWizardModel) overrideTargetSelected(target taskRunWizardOverrideTarget) bool {
	if m.runtimeForm == nil {
		return false
	}
	switch target.Kind {
	case taskRunWizardOverrideTargetType:
		return slices.Contains(m.runtimeForm.selectedTypes, target.Key)
	case taskRunWizardOverrideTargetTask:
		return slices.Contains(m.runtimeForm.selectedTasks, target.Key)
	default:
		return false
	}
}

func (m *taskRunWizardModel) toggleOverrideTarget(target taskRunWizardOverrideTarget) {
	if m.runtimeForm == nil {
		return
	}
	switch target.Kind {
	case taskRunWizardOverrideTargetType:
		m.runtimeForm.selectedTypes = taskRunWizardToggleString(m.runtimeForm.selectedTypes, target.Key)
		if _, ok := m.runtimeForm.typeEditors[target.Key]; !ok {
			m.runtimeForm.typeEditors[target.Key] = &taskRuntimeEditor{}
		}
	case taskRunWizardOverrideTargetTask:
		m.runtimeForm.selectedTasks = taskRunWizardToggleString(m.runtimeForm.selectedTasks, target.Key)
		if _, ok := m.runtimeForm.taskEditors[target.Key]; !ok {
			m.runtimeForm.taskEditors[target.Key] = &taskRuntimeEditor{}
		}
	}
}

func taskRunWizardToggleString(values []string, value string) []string {
	idx := slices.Index(values, value)
	if idx >= 0 {
		return slices.Delete(values, idx, idx+1)
	}
	return append(values, value)
}

func (m *taskRunWizardModel) currentOverrideEditor() *taskRuntimeEditor {
	target, ok := m.currentOverrideTarget()
	if !ok || !m.overrideTargetSelected(target) || m.runtimeForm == nil {
		return nil
	}
	switch target.Kind {
	case taskRunWizardOverrideTargetType:
		return m.runtimeForm.typeEditors[target.Key]
	case taskRunWizardOverrideTargetTask:
		return m.runtimeForm.taskEditors[target.Key]
	default:
		return nil
	}
}

func (m *taskRunWizardModel) syncOverrideModelInputFromEditor() {
	editor := m.currentOverrideEditor()
	if editor == nil {
		m.overrideModelInput.SetValue("")
		return
	}
	m.overrideModelInput.SetValue(editor.Model)
}

func (m *taskRunWizardModel) cycleOverrideChoice(delta int) {
	editor := m.currentOverrideEditor()
	if editor == nil {
		return
	}
	switch m.overrideEditorCursor {
	case taskRunWizardOverrideFieldIDE:
		editor.IDE = cycleTaskRunWizardChoice(m.overrideIDEOptions(), editor.IDE, delta)
	case taskRunWizardOverrideFieldReasoning:
		editor.ReasoningEffort = cycleTaskRunWizardChoice(m.overrideReasoningOptions(), editor.ReasoningEffort, delta)
	}
	m.syncOverrideModelInputFromEditor()
}

func (m *taskRunWizardModel) overrideIDEOptions() []taskRunWizardChoice {
	options := []taskRunWizardChoice{{Label: "Inherit default", Value: ""}}
	options = append(options, m.ideOptions...)
	return options
}

func (m *taskRunWizardModel) overrideReasoningOptions() []taskRunWizardChoice {
	options := []taskRunWizardChoice{{Label: "Inherit default", Value: ""}}
	options = append(options, m.reasoningOpts...)
	return options
}

func (m *taskRunWizardModel) previousStep() {
	if m.step > taskRunWizardStepWorkflows {
		if m.step == taskRunWizardStepOverrides {
			m.syncTaskRuntimeRulesFromForm()
		}
		if m.step == taskRunWizardStepReview && !m.inputs.defineTaskRuntime {
			m.step = taskRunWizardStepExecution
			m.message = ""
			m.syncTextFocus()
			return
		}
		m.step--
	}
	m.message = ""
	m.syncTextFocus()
}

func (m *taskRunWizardModel) nextStep() tea.Cmd {
	if err := m.validateCurrentStep(); err != nil {
		m.message = err.Error()
		return nil
	}
	switch m.step {
	case taskRunWizardStepExecution:
		if m.inputs.defineTaskRuntime {
			m.step = taskRunWizardStepOverrides
			m.message = ""
			m.syncTextFocus()
			return m.loadOverridesCmd()
		}
		m.syncTaskRuntimeRulesFromForm()
		m.step = taskRunWizardStepReview
	case taskRunWizardStepOverrides:
		m.syncTaskRuntimeRulesFromForm()
		m.step = taskRunWizardStepReview
	default:
		m.step++
	}
	m.message = ""
	m.syncTextFocus()
	return nil
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
		m.clampOrderCursor()
		return
	}
	m.inputs.selectedWorkflows = append(m.inputs.selectedWorkflows, trimmed)
	m.clampOrderCursor()
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
	m.clampOrderCursor()
}

func (m *taskRunWizardModel) clampOrderCursor() {
	if len(m.inputs.selectedWorkflows) == 0 {
		m.orderCursor = 0
		m.workflowFocus = taskRunWizardWorkflowFocusList
		return
	}
	m.orderCursor = min(max(m.orderCursor, 0), len(m.inputs.selectedWorkflows)-1)
}

func (m *taskRunWizardModel) moveSelectedWorkflow(delta int) {
	if len(m.inputs.selectedWorkflows) < 2 {
		return
	}
	from := m.orderCursor
	to := from + delta
	if to < 0 || to >= len(m.inputs.selectedWorkflows) {
		return
	}
	m.inputs.selectedWorkflows[from], m.inputs.selectedWorkflows[to] =
		m.inputs.selectedWorkflows[to], m.inputs.selectedWorkflows[from]
	m.orderCursor = to
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

func (m *taskRunWizardModel) cycleExecutionChoice(delta int) {
	switch m.execCursor {
	case taskRunWizardFieldRecoveryIDE:
		m.inputs.recoveryIDE = cycleTaskRunWizardChoice(m.ideOptions, m.inputs.recoveryIDE, delta)
	case taskRunWizardFieldRecoveryReasoning:
		m.inputs.recoveryReasoning = cycleTaskRunWizardChoice(m.reasoningOpts, m.inputs.recoveryReasoning, delta)
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
	case taskRunWizardFieldRecoveryEnabled:
		m.inputs.recoveryEnabled = !m.inputs.recoveryEnabled
		m.clampExecutionCursor()
	case taskRunWizardFieldDefineRuntime:
		m.inputs.defineTaskRuntime = !m.inputs.defineTaskRuntime
		if !m.inputs.defineTaskRuntime {
			m.runtimeForm = nil
			m.overridesLoading = false
			m.overridesLoadErr = ""
			m.inputs.taskRuntimeRules = nil
		}
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
	m.textInputs.recoveryModel.SetWidth(width)
	m.overrideModelInput.SetWidth(width)
}

func (m *taskRunWizardModel) syncTextFocus() {
	m.textInputs.manualWorkflow.Blur()
	m.textInputs.model.Blur()
	m.textInputs.addDirs.Blur()
	m.textInputs.timeout.Blur()
	m.textInputs.tailLines.Blur()
	m.textInputs.maxRetries.Blur()
	m.textInputs.retryBackoff.Blur()
	m.textInputs.recoveryModel.Blur()
	m.overrideModelInput.Blur()
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
		case taskRunWizardFieldRecoveryModel:
			m.textInputs.recoveryModel.Focus()
		}
	}
	if m.step == taskRunWizardStepOverrides &&
		m.overrideFocus == taskRunWizardOverrideFocusEditor &&
		m.overrideEditorCursor == taskRunWizardOverrideFieldModel {
		m.overrideModelInput.Focus()
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
	bodyHeight := max(6, m.height-7)
	body := m.renderBody(contentWidth, bodyHeight)
	if m.showHelp {
		body = m.renderHelpOverlay(contentWidth, bodyHeight)
	}
	inner := strings.Join([]string{
		m.renderHeader(contentWidth),
		wizardHR(contentWidth),
		wizardClampBody(body, bodyHeight, contentWidth),
		wizardHR(contentWidth),
		m.renderFooter(contentWidth),
	}, "\n")
	view := tea.NewView(wizardChromeStyle(m.width).Render(inner))
	view.AltScreen = true
	return view
}

func (m *taskRunWizardModel) renderHeader(width int) string {
	return wizardBrandLine(m.step, width) + "\n" + wizardStepper(m.step, width)
}

func (m *taskRunWizardModel) renderBody(width int, height int) string {
	switch m.step {
	case taskRunWizardStepWorkflows:
		return m.renderWorkflowStep(width, height)
	case taskRunWizardStepRuntime:
		return m.renderRuntimeStep(width)
	case taskRunWizardStepExecution:
		return m.renderExecutionStep()
	case taskRunWizardStepOverrides:
		return m.renderOverridesStep(width, height)
	case taskRunWizardStepReview:
		return m.renderReviewStep(width)
	default:
		return ""
	}
}

func (m *taskRunWizardModel) renderHelpOverlay(width int, height int) string {
	helpWidth := min(width, 60)
	box := charmtheme.TechPanelStyle(helpWidth, true).Render(m.helpContent())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
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
	if width >= 96 {
		return m.renderWorkflowDualPane(filtered, width, height)
	}
	return m.renderWorkflowCompact(filtered, width, height)
}

func (m *taskRunWizardModel) renderWorkflowDualPane(filtered []string, width int, height int) string {
	gap := 1
	leftTotal := max(30, (width-gap)*3/5)
	rightTotal := max(24, width-gap-leftTotal)
	rows := max(3, height-4)
	listFocused := m.workflowFocus == taskRunWizardWorkflowFocusList
	left := wizardRenderPane(leftTotal, rows, listFocused, m.workflowListLines(filtered, leftTotal-4, rows-1))
	right := wizardRenderPane(rightTotal, rows, !listFocused, m.workflowOrderLines(rightTotal-4, rows-1))
	panes := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	return strings.Join([]string{
		taskRunWizardSubtitleStyle().Render("Select workflows and shape the run queue."),
		panes,
		m.workflowStatusLine(),
	}, "\n")
}

func (m *taskRunWizardModel) renderWorkflowCompact(filtered []string, width int, height int) string {
	rows := max(2, (height-5)/2)
	listFocused := m.workflowFocus == taskRunWizardWorkflowFocusList
	list := wizardRenderPane(width, rows, listFocused, m.workflowListLines(filtered, width-4, rows-1))
	order := wizardRenderPane(width, rows, !listFocused, m.workflowOrderLines(width-4, rows-1))
	return strings.Join([]string{list, order, m.workflowStatusLine()}, "\n")
}

func (m *taskRunWizardModel) workflowStatusLine() string {
	status := taskRunWizardActiveStyle().Render(fmt.Sprintf("%d selected", len(m.inputs.selectedWorkflows)))
	if m.searchActive || m.searchQuery != "" {
		status += taskRunWizardMutedStyle().Render("   filter: ") + m.searchQuery
	}
	return status
}

func (m *taskRunWizardModel) workflowListLines(filtered []string, width int, visibleRows int) []string {
	focused := m.workflowFocus == taskRunWizardWorkflowFocusList
	suffix := ""
	if m.searchActive || m.searchQuery != "" {
		suffix = "/" + m.searchQuery
	}
	lines := []string{wizardPaneTitle("WORKFLOWS", focused, suffix)}
	if len(filtered) == 0 {
		return append(lines, taskRunWizardMutedStyle().Render("No workflows match the filter."))
	}
	start := 0
	if m.workflowCursor >= visibleRows {
		start = m.workflowCursor - visibleRows + 1
	}
	end := min(start+visibleRows, len(filtered))
	for idx := start; idx < end; idx++ {
		option := filtered[idx]
		cursor := "  "
		if idx == m.workflowCursor && focused {
			cursor = taskRunWizardActiveStyle().Render("▸ ")
		}
		mark := taskRunWizardMutedStyle().Render("[ ]")
		if slices.Contains(m.inputs.selectedWorkflows, option) {
			mark = taskRunWizardActiveStyle().Render("[x]")
		}
		lines = append(lines, taskRunWizardTruncate(cursor+mark+" "+option, width))
	}
	return lines
}

func (m *taskRunWizardModel) workflowOrderLines(width int, visibleRows int) []string {
	focused := m.workflowFocus == taskRunWizardWorkflowFocusOrder
	lines := []string{wizardPaneTitle("RUN ORDER", focused, "")}
	if len(m.inputs.selectedWorkflows) == 0 {
		return append(lines, taskRunWizardMutedStyle().Render("No workflows selected."))
	}
	start := 0
	if m.orderCursor >= visibleRows {
		start = m.orderCursor - visibleRows + 1
	}
	end := min(start+visibleRows, len(m.inputs.selectedWorkflows))
	for idx := start; idx < end; idx++ {
		cursor := "  "
		num := taskRunWizardMutedStyle().Render(fmt.Sprintf("%02d", idx+1))
		if idx == m.orderCursor && focused {
			cursor = taskRunWizardActiveStyle().Render("▸ ")
			num = taskRunWizardActiveStyle().Render(fmt.Sprintf("%02d", idx+1))
		}
		lines = append(lines, taskRunWizardTruncate(cursor+num+" "+m.inputs.selectedWorkflows[idx], width))
	}
	return lines
}

func (m *taskRunWizardModel) renderRuntimeStep(width int) string {
	ideActive := m.runtimeCursor == taskRunWizardFieldIDE
	reasoningActive := m.runtimeCursor == taskRunWizardFieldReasoning
	accessActive := m.runtimeCursor == taskRunWizardFieldAccessMode
	return strings.Join([]string{
		taskRunWizardSubtitleStyle().Render("Default runtime applied to every selected workflow."),
		"",
		m.renderField(
			"Provider / IDE",
			wizardSelectValue(taskRunWizardChoiceLabel(m.ideOptions, m.inputs.ide), ideActive),
			ideActive,
		),
		m.renderField("Model", m.textInputs.model.View(), m.runtimeCursor == taskRunWizardFieldModel),
		m.renderField("Additional dirs", m.textInputs.addDirs.View(), m.runtimeCursor == taskRunWizardFieldAddDirs),
		m.renderField(
			"Reasoning effort",
			wizardSelectValue(taskRunWizardChoiceLabel(m.reasoningOpts, m.inputs.reasoningEffort), reasoningActive),
			reasoningActive,
		),
		m.renderField(
			"Access mode",
			wizardSelectValue(taskRunWizardChoiceLabel(m.accessModeOpts, m.inputs.accessMode), accessActive),
			accessActive,
		),
		"",
		taskRunWizardMutedStyle().Render(taskRunWizardTruncate(
			"Use ←/→ or Space to cycle ‹ options ›. Text fields edit in place.",
			width-2,
		)),
	}, "\n")
}

func (m *taskRunWizardModel) renderExecutionStep() string {
	recoveryIDEActive := m.execCursor == taskRunWizardFieldRecoveryIDE
	recoveryReasoningActive := m.execCursor == taskRunWizardFieldRecoveryReasoning
	lines := []string{
		taskRunWizardSubtitleStyle().Render("Tune retry, timeout, and run behavior."),
		"",
		m.renderField("Activity timeout", m.textInputs.timeout.View(), m.execCursor == taskRunWizardFieldTimeout),
		m.renderField("Tail lines", m.textInputs.tailLines.View(), m.execCursor == taskRunWizardFieldTailLines),
		m.renderField("Max retries", m.textInputs.maxRetries.View(), m.execCursor == taskRunWizardFieldMaxRetries),
		m.renderField(
			"Retry backoff",
			m.textInputs.retryBackoff.View(),
			m.execCursor == taskRunWizardFieldRetryBackoff,
		),
		m.renderField("Dry run", wizardBoolValue(m.inputs.dryRun), m.execCursor == taskRunWizardFieldDryRun),
		m.renderField(
			"Auto commit",
			wizardBoolValue(m.inputs.autoCommit),
			m.execCursor == taskRunWizardFieldAutoCommit,
		),
		m.renderField(
			"Include completed",
			wizardBoolValue(m.inputs.includeCompleted),
			m.execCursor == taskRunWizardFieldIncludeCompleted,
		),
		m.renderField("Recursive", wizardBoolValue(m.inputs.recursive), m.execCursor == taskRunWizardFieldRecursive),
		m.renderField(
			"Recovery",
			wizardBoolValue(m.inputs.recoveryEnabled),
			m.execCursor == taskRunWizardFieldRecoveryEnabled,
		),
	}
	if m.inputs.recoveryEnabled {
		lines = append(lines,
			m.renderField(
				"Recovery IDE",
				wizardSelectValue(taskRunWizardChoiceLabel(m.ideOptions, m.inputs.recoveryIDE), recoveryIDEActive),
				recoveryIDEActive,
			),
			m.renderField(
				"Recovery model",
				m.textInputs.recoveryModel.View(),
				m.execCursor == taskRunWizardFieldRecoveryModel,
			),
			m.renderField(
				"Recovery reasoning",
				wizardSelectValue(
					taskRunWizardChoiceLabel(m.reasoningOpts, m.inputs.recoveryReasoning),
					recoveryReasoningActive,
				),
				recoveryReasoningActive,
			),
		)
	}
	lines = append(lines,
		m.renderField(
			"Runtime per task",
			wizardBoolValue(m.inputs.defineTaskRuntime),
			m.execCursor == taskRunWizardFieldDefineRuntime,
		),
	)
	return strings.Join(lines, "\n")
}

func (m *taskRunWizardModel) renderOverridesStep(width int, height int) string {
	if m.overridesLoading {
		return strings.Join([]string{
			taskRunWizardSubtitleStyle().Render("Loading task runtime targets..."),
			"",
			taskRunWizardMutedStyle().Render("Reading task files for the selected workflows."),
		}, "\n")
	}
	if m.overridesLoadErr != "" {
		return strings.Join([]string{
			taskRunWizardErrorStyle().Render("Could not load task runtime targets."),
			"",
			taskRunWizardTruncate(m.overridesLoadErr, width-6),
		}, "\n")
	}
	if m.runtimeForm == nil {
		return strings.Join([]string{
			taskRunWizardSubtitleStyle().Render("No pending task targets were found for runtime overrides."),
			"",
			taskRunWizardMutedStyle().Render("Press Enter to continue to review."),
		}, "\n")
	}
	targets := m.overrideTargets()
	subtitle := taskRunWizardSubtitleStyle().Render("Override runtime per task type or task, scoped by workflow.")
	header := taskRunWizardMutedStyle().Render("Workflow ") + m.renderOverrideWorkflowTabs(width-12)
	targetsFocused := m.overrideFocus == taskRunWizardOverrideFocusTargets
	if width >= 104 {
		gap := 1
		leftTotal := max(34, (width-gap)/2)
		rightTotal := max(30, width-gap-leftTotal)
		rows := max(3, height-4)
		left := wizardRenderPane(leftTotal, rows, targetsFocused, m.overrideTargetLines(targets, leftTotal-4, rows-1))
		right := wizardRenderPane(rightTotal, rows, !targetsFocused, m.overrideEditorLines(rightTotal-4))
		panes := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
		return strings.Join([]string{subtitle, header, panes}, "\n")
	}
	rows := max(2, (height-6)/2)
	targetsPane := wizardRenderPane(width, rows, targetsFocused, m.overrideTargetLines(targets, width-4, rows-1))
	editorPane := wizardRenderPane(width, rows, !targetsFocused, m.overrideEditorLines(width-4))
	return strings.Join([]string{subtitle, header, targetsPane, editorPane}, "\n")
}

func (m *taskRunWizardModel) renderOverrideWorkflowTabs(width int) string {
	slugs := selectedTaskRunWizardWorkflows(m.inputs)
	parts := make([]string, 0, len(slugs))
	for i, slug := range slugs {
		label := taskRunWizardMutedStyle().Render(slug)
		if i == m.overrideWorkflowCursor {
			label = taskRunWizardActiveStyle().Render("[" + slug + "]")
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return taskRunWizardMutedStyle().Render(noneValue)
	}
	return taskRunWizardTruncate(strings.Join(parts, taskRunWizardMutedStyle().Render(" ")), width)
}

func (m *taskRunWizardModel) overrideTargetLines(
	targets []taskRunWizardOverrideTarget,
	width int,
	visibleRows int,
) []string {
	focused := m.overrideFocus == taskRunWizardOverrideFocusTargets
	lines := []string{wizardPaneTitle("TARGETS", focused, "")}
	if len(targets) == 0 {
		return append(lines, taskRunWizardMutedStyle().Render("No targets in this workflow."))
	}
	start := 0
	if m.overrideTargetCursor >= visibleRows {
		start = m.overrideTargetCursor - visibleRows + 1
	}
	end := min(start+visibleRows, len(targets))
	for idx := start; idx < end; idx++ {
		target := targets[idx]
		cursor := "  "
		if idx == m.overrideTargetCursor && focused {
			cursor = taskRunWizardActiveStyle().Render("▸ ")
		}
		mark := taskRunWizardMutedStyle().Render("[ ]")
		if m.overrideTargetSelected(target) {
			mark = taskRunWizardActiveStyle().Render("[x]")
		}
		kind := taskRunWizardKindType
		if target.Kind == taskRunWizardOverrideTargetTask {
			kind = taskRunWizardKindTask
		}
		kindLabel := taskRunWizardMutedStyle().Render(kind)
		lines = append(lines, taskRunWizardTruncate(cursor+mark+" "+kindLabel+" "+target.Label, width))
	}
	return lines
}

func (m *taskRunWizardModel) overrideEditorLines(width int) []string {
	focused := m.overrideFocus == taskRunWizardOverrideFocusEditor
	title := wizardPaneTitle("RUNTIME OVERRIDE", focused, "")
	target, ok := m.currentOverrideTarget()
	if !ok {
		return []string{title, taskRunWizardMutedStyle().Render("Select a target to edit.")}
	}
	if !m.overrideTargetSelected(target) {
		return []string{
			title,
			taskRunWizardMutedStyle().Render("Press Space to enable this target."),
			taskRunWizardTruncate(target.Label, width),
		}
	}
	editor := m.currentOverrideEditor()
	if editor == nil {
		return []string{title, taskRunWizardMutedStyle().Render("No editor is available for this target.")}
	}
	ideActive := m.overrideEditorCursor == taskRunWizardOverrideFieldIDE
	reasoningActive := m.overrideEditorCursor == taskRunWizardOverrideFieldReasoning
	return []string{
		title,
		taskRunWizardSubtitleStyle().Render(taskRunWizardTruncate(target.Label, width)),
		m.renderOverrideField(
			"IDE",
			wizardSelectValue(taskRunWizardChoiceLabel(m.overrideIDEOptions(), editor.IDE), ideActive && focused),
			ideActive,
		),
		m.renderOverrideField(
			"Model",
			m.overrideModelInput.View(),
			m.overrideEditorCursor == taskRunWizardOverrideFieldModel,
		),
		m.renderOverrideField(
			"Reasoning",
			wizardSelectValue(
				taskRunWizardChoiceLabel(m.overrideReasoningOptions(), editor.ReasoningEffort),
				reasoningActive && focused,
			),
			reasoningActive,
		),
		taskRunWizardMutedStyle().Render(taskRunWizardTruncate("Blank fields inherit runtime defaults.", width)),
	}
}

func (m *taskRunWizardModel) renderOverrideField(label string, value string, active bool) string {
	if m.overrideFocus != taskRunWizardOverrideFocusEditor {
		active = false
	}
	return wizardField(label, value, active, wizardOverrideLabelWidth)
}

func (m *taskRunWizardModel) renderReviewStep(width int) string {
	selected := selectedTaskRunWizardWorkflows(m.inputs)
	lines := []string{
		taskRunWizardSubtitleStyle().Render("Review before starting the daemon run."),
		"",
		taskRunWizardActiveStyle().Render("RUN ORDER"),
	}
	lines = append(lines, m.reviewOrderLines(selected, width)...)
	lines = append(lines,
		"",
		taskRunWizardActiveStyle().Render("RUNTIME"),
		wizardSummaryRow("Provider", taskRunWizardChoiceLabel(m.ideOptions, m.inputs.ide), width),
		wizardSummaryRow("Model", taskRunWizardBlank(m.inputs.model), width),
		wizardSummaryRow("Reasoning", taskRunWizardBlank(m.inputs.reasoningEffort), width),
		"",
		taskRunWizardActiveStyle().Render("EXECUTION"),
		wizardSummaryRow("Timeout", taskRunWizardBlank(m.inputs.timeout), width),
		wizardSummaryRow("Flags", m.reviewFlagsValue(), width),
	)
	if m.inputs.recoveryEnabled {
		lines = append(lines,
			wizardSummaryRow("Recovery IDE", taskRunWizardChoiceLabel(m.ideOptions, m.inputs.recoveryIDE), width),
			wizardSummaryRow("Recovery model", taskRunWizardBlank(m.inputs.recoveryModel), width),
			wizardSummaryRow("Recovery reasoning", taskRunWizardBlank(m.inputs.recoveryReasoning), width),
		)
	}
	if m.inputs.defineTaskRuntime {
		lines = append(lines, "", taskRunWizardActiveStyle().Render("OVERRIDES"))
		lines = append(lines, m.reviewOverrideLines(width)...)
	}
	lines = append(lines, "", taskRunWizardActiveStyle().Render("Press Enter to start the run."))
	return strings.Join(lines, "\n")
}

func (m *taskRunWizardModel) reviewOrderLines(selected []string, width int) []string {
	if len(selected) == 0 {
		return []string{"  " + taskRunWizardMutedStyle().Render(noneValue)}
	}
	const maxRows = 4
	lines := make([]string, 0, len(selected))
	for i, slug := range selected {
		if i >= maxRows {
			lines = append(lines, "  "+taskRunWizardMutedStyle().Render(fmt.Sprintf("+%d more", len(selected)-maxRows)))
			break
		}
		num := taskRunWizardMutedStyle().Render(fmt.Sprintf("%02d", i+1))
		lines = append(lines, "  "+num+" "+taskRunWizardTruncate(slug, max(8, width-6)))
	}
	return lines
}

func (m *taskRunWizardModel) reviewFlagsValue() string {
	flags := make([]string, 0, 6)
	if m.inputs.dryRun {
		flags = append(flags, "dry-run")
	}
	if m.inputs.autoCommit {
		flags = append(flags, "auto-commit")
	}
	if m.inputs.includeCompleted {
		flags = append(flags, "include-completed")
	}
	if m.inputs.recursive {
		flags = append(flags, "recursive")
	}
	if m.inputs.recoveryEnabled {
		flags = append(flags, "recovery")
	}
	if m.inputs.defineTaskRuntime {
		flags = append(flags, "per-task-runtime")
	}
	if len(flags) == 0 {
		return taskRunWizardMutedStyle().Render(noneValue)
	}
	return strings.Join(flags, ", ")
}

func (m *taskRunWizardModel) reviewOverrideLines(width int) []string {
	rules := m.inputs.taskRuntimeRules
	if len(rules) == 0 {
		return []string{"  " + taskRunWizardMutedStyle().Render(noneValue)}
	}
	const maxRows = 4
	lines := make([]string, 0, len(rules))
	for i := range rules {
		if i >= maxRows {
			lines = append(lines, "  "+taskRunWizardMutedStyle().Render(fmt.Sprintf("+%d more", len(rules)-maxRows)))
			break
		}
		lines = append(lines, "  "+taskRunWizardTruncate(formatTaskRunWizardRule(rules[i]), max(8, width-4)))
	}
	return lines
}

func formatTaskRunWizardRule(rule model.TaskRuntimeRule) string {
	selector := taskRunWizardKindTask
	target := ""
	switch {
	case rule.Type != nil:
		selector = taskRunWizardKindType
		target = strings.TrimSpace(*rule.Type)
	case rule.ID != nil:
		selector = taskRunWizardKindTask
		target = strings.TrimSpace(*rule.ID)
	}
	scope := ""
	if rule.Workflow != nil && strings.TrimSpace(*rule.Workflow) != "" {
		scope = strings.TrimSpace(*rule.Workflow) + "/"
	}
	over := make([]string, 0, 3)
	if rule.IDE != nil && strings.TrimSpace(*rule.IDE) != "" {
		over = append(over, "ide="+strings.TrimSpace(*rule.IDE))
	}
	if rule.Model != nil && strings.TrimSpace(*rule.Model) != "" {
		over = append(over, "model="+strings.TrimSpace(*rule.Model))
	}
	if rule.ReasoningEffort != nil && strings.TrimSpace(*rule.ReasoningEffort) != "" {
		over = append(over, "reason="+strings.TrimSpace(*rule.ReasoningEffort))
	}
	overStr := taskRunWizardMutedStyle().Render("inherit")
	if len(over) > 0 {
		overStr = strings.Join(over, " ")
	}
	return taskRunWizardMutedStyle().Render(selector+" ") + scope + target +
		taskRunWizardMutedStyle().Render(" → ") + overStr
}

func (m *taskRunWizardModel) renderField(label string, value string, active bool) string {
	return wizardField(label, value, active, wizardFieldLabelWidth)
}

func (m *taskRunWizardModel) renderFooter(width int) string {
	if m.showHelp {
		return taskRunWizardTruncate(wizardFooterHint([][2]string{{"?", "close"}, {"esc", "close"}}), width)
	}
	var pairs [][2]string
	switch {
	case m.step == taskRunWizardStepWorkflows && len(m.workflowOptions) > 0:
		pairs = [][2]string{
			{"space", "toggle"}, {"a", "all"}, {"/", "filter"},
			{"h/l", "focus"}, {"u/d", "order"}, {"enter", "next"}, {"?", "help"},
		}
	case m.step == taskRunWizardStepOverrides:
		pairs = [][2]string{
			{"[", "prev wf"}, {"]", "next wf"}, {"space", "select"},
			{"h/l", "focus"}, {"enter", "review"}, {"?", "help"},
		}
	case m.step == taskRunWizardStepReview:
		pairs = [][2]string{{"enter", "start"}, {"tab", "restart"}, {"esc", "back"}, {"?", "help"}}
	default:
		pairs = [][2]string{
			{"↑/↓", "move"}, {"←/→", "cycle"}, {"enter", "next"},
			{"esc", "back"}, {"?", "help"}, {"q", "quit"},
		}
	}
	hint := wizardFooterHint(pairs)
	if m.message != "" {
		hint = taskRunWizardErrorStyle().Render(m.message) + "  " + hint
	}
	return taskRunWizardTruncate(hint, width)
}

func (m *taskRunWizardModel) helpContent() string {
	rows := [][2]string{
		{"j/k · ↑/↓", "move cursor"},
		{"space", "toggle selection / cycle option"},
		{"←/→", "cycle option"},
		{"h/l", "move focus between panes"},
		{"u/d", "reorder workflow in run queue"},
		{"/", "filter workflows"},
		{"[ · ]", "switch workflow in overrides"},
		{"enter · tab", "advance step"},
		{"shift+tab · esc", "go back"},
		{"q · ctrl+c", "cancel wizard"},
	}
	muted := taskRunWizardMutedStyle()
	lines := []string{taskRunWizardTitleStyle().Render("KEYBOARD"), ""}
	for _, row := range rows {
		lines = append(lines, charmtheme.Keycap(row[0])+" "+muted.Render(row[1]))
	}
	return strings.Join(lines, "\n")
}

func taskRunWizardChoiceLabel(options []taskRunWizardChoice, value string) string {
	for _, option := range options {
		if option.Value == value {
			return option.Label
		}
	}
	return taskRunWizardBlank(value)
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
	applyInput(cmd, "recovery", inputs.recoveryEnabled, passThroughInput[bool], func(value bool) {
		state.recoveryEnabled = value
	})
	applyInput(cmd, "recovery-ide", inputs.recoveryIDE, passThroughInput[string], func(value string) {
		state.recoveryIDE = value
	})
	applyInput(cmd, "recovery-model", inputs.recoveryModel, passThroughInput[string], func(value string) {
		state.recoveryModel = value
	})
	applyInput(cmd, "recovery-reasoning", inputs.recoveryReasoning, passThroughInput[string], func(value string) {
		state.recoveryReasoningEffort = value
	})
	state.replaceConfiguredTaskRunRules = true
	state.executionTaskRuntimeRules = model.CloneTaskRuntimeRules(inputs.taskRuntimeRules)
	markInputFlagChanged(cmd, "task-runtime")
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
