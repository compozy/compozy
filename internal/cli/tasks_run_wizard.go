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
	switch key {
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
			return m, m.nextStep()
		}
		m.execCursor++
		m.syncTextFocus()
	case " ", taskRunWizardKeySpace:
		m.toggleExecutionBool()
	default:
		return m.updateExecutionText(msg)
	}
	return m, nil
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
		taskRunWizardFieldRetryBackoff:
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
	crumbs := []string{"Workflows", "Runtime", "Execution", "Overrides", "Review"}
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
	case taskRunWizardStepOverrides:
		body = m.renderOverridesStep(width, height)
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
	if width >= 96 {
		return m.renderWorkflowDualPane(filtered, width, height)
	}
	return m.renderWorkflowCompact(filtered, width, height)
}

func (m *taskRunWizardModel) renderWorkflowDualPane(filtered []string, width int, height int) string {
	leftWidth := max(28, (width-8)*3/5)
	rightWidth := max(24, width-8-leftWidth)
	rows := max(4, height-6)
	leftLines := m.workflowListLines(filtered, leftWidth, rows)
	rightLines := m.workflowOrderLines(rightWidth, rows)
	return strings.Join([]string{
		taskRunWizardSubtitleStyle().Render("Select workflows and shape the run queue before execution."),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			taskRunWizardColumn(leftLines, leftWidth),
			"  ",
			taskRunWizardColumn(rightLines, rightWidth),
		),
	}, "\n")
}

func (m *taskRunWizardModel) renderWorkflowCompact(filtered []string, width int, height int) string {
	rows := max(3, (height-8)/2)
	lines := []string{
		taskRunWizardSubtitleStyle().Render("Select workflows, then use the Run Order section to reorder."),
	}
	if m.searchActive || m.searchQuery != "" {
		lines = append(lines, taskRunWizardMutedStyle().Render("Filter: ")+m.searchQuery)
	}
	lines = append(lines, m.workflowListLines(filtered, width-4, rows)...)
	lines = append(lines, "")
	lines = append(lines, m.workflowOrderLines(width-4, rows)...)
	return strings.Join(lines, "\n")
}

func (m *taskRunWizardModel) workflowListLines(filtered []string, width int, visibleRows int) []string {
	title := "Workflows"
	if m.workflowFocus == taskRunWizardWorkflowFocusList {
		title = taskRunWizardActiveStyle().Render(title)
	} else {
		title = taskRunWizardMutedStyle().Render(title)
	}
	lines := []string{title}
	if m.searchActive || m.searchQuery != "" {
		lines = append(lines, taskRunWizardMutedStyle().Render("Filter: ")+m.searchQuery)
	}
	start := 0
	if m.workflowCursor >= visibleRows {
		start = m.workflowCursor - visibleRows + 1
	}
	end := min(start+visibleRows, len(filtered))
	for idx := start; idx < end; idx++ {
		option := filtered[idx]
		cursor := "  "
		if idx == m.workflowCursor && m.workflowFocus == taskRunWizardWorkflowFocusList {
			cursor = taskRunWizardActiveStyle().Render("▸ ")
		}
		mark := "[ ]"
		if slices.Contains(m.inputs.selectedWorkflows, option) {
			mark = taskRunWizardActiveStyle().Render("[x]")
		}
		lines = append(lines, taskRunWizardTruncate(cursor+mark+" "+option, width))
	}
	if len(filtered) == 0 {
		lines = append(lines, taskRunWizardMutedStyle().Render("No workflows match the filter."))
	}
	return lines
}

func (m *taskRunWizardModel) workflowOrderLines(width int, visibleRows int) []string {
	title := "Run Order"
	if m.workflowFocus == taskRunWizardWorkflowFocusOrder {
		title = taskRunWizardActiveStyle().Render(title)
	} else {
		title = taskRunWizardMutedStyle().Render(title)
	}
	lines := []string{title}
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
		if idx == m.orderCursor && m.workflowFocus == taskRunWizardWorkflowFocusOrder {
			cursor = taskRunWizardActiveStyle().Render("▸ ")
		}
		row := fmt.Sprintf("%s%02d %s", cursor, idx+1, m.inputs.selectedWorkflows[idx])
		lines = append(lines, taskRunWizardTruncate(row, width))
	}
	return lines
}

func taskRunWizardColumn(lines []string, width int) string {
	for i := range lines {
		lines[i] = taskRunWizardTruncate(lines[i], width)
	}
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
	rows := max(4, height-7)
	lines := []string{
		taskRunWizardSubtitleStyle().Render("Select task types or individual tasks, scoped by workflow."),
		taskRunWizardMutedStyle().Render("Workflow: ") + m.renderOverrideWorkflowTabs(width-16),
		"",
	}
	if width >= 104 {
		leftWidth := max(36, (width-8)/2)
		rightWidth := max(32, width-8-leftWidth)
		return strings.Join(append(lines,
			lipgloss.JoinHorizontal(lipgloss.Top,
				taskRunWizardColumn(m.overrideTargetLines(targets, leftWidth, rows), leftWidth),
				"  ",
				taskRunWizardColumn(m.overrideEditorLines(rightWidth), rightWidth),
			),
		), "\n")
	}
	lines = append(lines, m.overrideTargetLines(targets, width-4, rows)...)
	lines = append(lines, "")
	lines = append(lines, m.overrideEditorLines(width-4)...)
	return strings.Join(lines, "\n")
}

func (m *taskRunWizardModel) renderOverrideWorkflowTabs(width int) string {
	slugs := selectedTaskRunWizardWorkflows(m.inputs)
	parts := make([]string, 0, len(slugs))
	for i, slug := range slugs {
		label := slug
		if i == m.overrideWorkflowCursor {
			label = taskRunWizardActiveStyle().Render(label)
		} else {
			label = taskRunWizardMutedStyle().Render(label)
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return noneValue
	}
	return taskRunWizardTruncate(strings.Join(parts, taskRunWizardMutedStyle().Render(" / ")), width)
}

func (m *taskRunWizardModel) overrideTargetLines(
	targets []taskRunWizardOverrideTarget,
	width int,
	visibleRows int,
) []string {
	title := "Targets"
	if m.overrideFocus == taskRunWizardOverrideFocusTargets {
		title = taskRunWizardActiveStyle().Render(title)
	} else {
		title = taskRunWizardMutedStyle().Render(title)
	}
	lines := []string{title}
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
		if idx == m.overrideTargetCursor && m.overrideFocus == taskRunWizardOverrideFocusTargets {
			cursor = taskRunWizardActiveStyle().Render("▸ ")
		}
		mark := "[ ]"
		if m.overrideTargetSelected(target) {
			mark = taskRunWizardActiveStyle().Render("[x]")
		}
		kind := "type"
		if target.Kind == taskRunWizardOverrideTargetTask {
			kind = "task"
		}
		lines = append(lines, taskRunWizardTruncate(cursor+mark+" "+kind+"  "+target.Label, width))
	}
	return lines
}

func (m *taskRunWizardModel) overrideEditorLines(width int) []string {
	title := "Runtime Override"
	if m.overrideFocus == taskRunWizardOverrideFocusEditor {
		title = taskRunWizardActiveStyle().Render(title)
	} else {
		title = taskRunWizardMutedStyle().Render(title)
	}
	target, ok := m.currentOverrideTarget()
	if !ok {
		return []string{title, taskRunWizardMutedStyle().Render("Select a target to edit.")}
	}
	if !m.overrideTargetSelected(target) {
		return []string{
			title,
			taskRunWizardMutedStyle().Render("Select this target with Space before editing."),
			taskRunWizardTruncate(target.Label, width),
		}
	}
	editor := m.currentOverrideEditor()
	if editor == nil {
		return []string{title, taskRunWizardMutedStyle().Render("No editor is available for this target.")}
	}
	return []string{
		title,
		taskRunWizardTruncate(target.Label, width),
		m.renderOverrideField(
			"IDE",
			taskRunWizardChoiceLabel(m.overrideIDEOptions(), editor.IDE),
			m.overrideEditorCursor == taskRunWizardOverrideFieldIDE,
		),
		m.renderOverrideField(
			"Model",
			m.overrideModelInput.View(),
			m.overrideEditorCursor == taskRunWizardOverrideFieldModel,
		),
		m.renderOverrideField(
			"Reasoning",
			taskRunWizardChoiceLabel(m.overrideReasoningOptions(), editor.ReasoningEffort),
			m.overrideEditorCursor == taskRunWizardOverrideFieldReasoning,
		),
		taskRunWizardMutedStyle().Render(taskRunWizardTruncate("Blank fields inherit the runtime defaults.", width)),
	}
}

func (m *taskRunWizardModel) renderOverrideField(label string, value string, active bool) string {
	if m.overrideFocus != taskRunWizardOverrideFocusEditor {
		active = false
	}
	return m.renderField(label, value, active)
}

func (m *taskRunWizardModel) renderReviewStep(width int) string {
	selected := selectedTaskRunWizardWorkflows(m.inputs)
	lines := []string{
		taskRunWizardSubtitleStyle().Render("Review selections before starting the daemon run."),
		m.renderSummary("Run order", formatTaskRunWizardOrder(selected), width),
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
		m.renderSummary("Overrides", formatTaskRunWizardRuntimeRules(m.inputs.taskRuntimeRules), width),
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
		msg = "[Space]toggle [a]all [/]filter [h/l]focus [u/d]reorder [Enter]next"
	}
	if m.step == taskRunWizardStepOverrides {
		msg = "[[]prev workflow []]next workflow [Space]select [h/l]focus [Enter]review"
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
		"  h/l: move focus between panes",
		"  u/d: move workflow up or down in Run Order",
		"  [ and ]: switch workflow in runtime overrides",
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

func formatTaskRunWizardOrder(slugs []string) string {
	if len(slugs) == 0 {
		return noneValue
	}
	parts := make([]string, 0, len(slugs))
	for i, slug := range slugs {
		parts = append(parts, fmt.Sprintf("%02d %s", i+1, slug))
	}
	return strings.Join(parts, "  ")
}

func formatTaskRunWizardRuntimeRules(rules []model.TaskRuntimeRule) string {
	if len(rules) == 0 {
		return noneValue
	}
	return fmt.Sprintf("%d selected", len(rules))
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
