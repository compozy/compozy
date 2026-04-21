package cli

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/spf13/cobra"
)

type taskRuntimeEditor struct {
	IDE             string
	Model           string
	ReasoningEffort string
}

type taskRuntimeTypeOption struct {
	Value string
	Label string
}

type taskRuntimeTaskOption struct {
	ID    string
	Type  string
	Label string
}

type startTaskRuntimeForm struct {
	selectedTypes []string
	selectedTasks []string
	typeOptions   []taskRuntimeTypeOption
	taskOptions   []taskRuntimeTaskOption
	typeEditors   map[string]*taskRuntimeEditor
	taskEditors   map[string]*taskRuntimeEditor
	baseRuntime   string
}

func collectStartTaskRuntimeForm(cmd *cobra.Command, state *commandState) error {
	if state == nil || state.kind != commandKindStart {
		return nil
	}

	form, err := newStartTaskRuntimeForm(state)
	if err != nil || form == nil {
		return err
	}
	if err := form.build().Run(); err != nil {
		return fmt.Errorf("task runtime form canceled or error: %w", err)
	}
	form.apply(state)
	markInputFlagChanged(cmd, "task-runtime")
	return nil
}

func newStartTaskRuntimeForm(state *commandState) (*startTaskRuntimeForm, error) {
	tasksDir, err := resolveTaskWorkflowDir(state.workspaceRoot, state.name, state.tasksDir)
	if err != nil {
		return nil, err
	}
	entries, err := tasks.ReadTaskEntries(tasksDir, state.includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("read task entries for runtime overrides: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	form := &startTaskRuntimeForm{
		typeEditors: make(map[string]*taskRuntimeEditor),
		taskEditors: make(map[string]*taskRuntimeEditor),
		baseRuntime: formatBaseTaskRuntime(state),
	}
	typeRuleByValue, taskRuleByID := indexTaskRuntimeRules(state.taskRuntimeRules())
	if err := form.populate(entries, typeRuleByValue, taskRuleByID); err != nil {
		return nil, err
	}
	form.ensureEditors()
	return form, nil
}

func (f *startTaskRuntimeForm) build() *huh.Form {
	groups := []*huh.Group{
		huh.NewGroup(f.selectorFields()...).Title("Per-Task Runtime"),
	}
	for _, option := range f.typeOptions {
		option := option
		editor := f.typeEditors[option.Value]
		groups = append(groups, huh.NewGroup(
			taskRuntimeIDEField(
				"type-"+option.Value+"-ide",
				"IDE",
				"Override the runtime for this task type",
				&editor.IDE,
			),
			taskRuntimeModelField("type-"+option.Value+"-model", &editor.Model),
			taskRuntimeReasoningField(
				"type-"+option.Value+"-reasoning-effort",
				"Reasoning Effort",
				"Override reasoning for this task type",
				&editor.ReasoningEffort,
			),
		).Title("Type: "+option.Label).Description("Applies to every task with this type.").WithHideFunc(func() bool {
			return !slices.Contains(f.selectedTypes, option.Value)
		}))
	}
	for _, option := range f.taskOptions {
		option := option
		editor := f.taskEditors[option.ID]
		groups = append(groups, huh.NewGroup(
			taskRuntimeIDEField(
				"task-"+option.ID+"-ide",
				"IDE",
				"Override the runtime for this task only",
				&editor.IDE,
			),
			taskRuntimeModelField("task-"+option.ID+"-model", &editor.Model),
			taskRuntimeReasoningField(
				"task-"+option.ID+"-reasoning-effort",
				"Reasoning Effort",
				"Override reasoning for this task only",
				&editor.ReasoningEffort,
			),
		).Title("Task: "+option.Label).Description("Task-specific overrides win over type rules.").WithHideFunc(func() bool {
			return !slices.Contains(f.selectedTasks, option.ID)
		}))
	}
	return huh.NewForm(groups...).WithTheme(darkHuhTheme())
}

func (f *startTaskRuntimeForm) selectorFields() []huh.Field {
	fields := []huh.Field{
		huh.NewNote().
			Title("Task Runtime Overrides").
			Description(taskRuntimeSelectionDescription(f.baseRuntime)),
	}
	if len(f.typeOptions) > 0 {
		options := make([]huh.Option[string], 0, len(f.typeOptions))
		for _, option := range f.typeOptions {
			options = append(options, huh.NewOption(option.Label, option.Value))
		}
		fields = append(fields, huh.NewMultiSelect[string]().
			Key("task-runtime-types").
			Title("Task Types").
			Description("Select task types to override in bulk.").
			Options(options...).
			Value(&f.selectedTypes))
	}
	if len(f.taskOptions) > 0 {
		options := make([]huh.Option[string], 0, len(f.taskOptions))
		for _, option := range f.taskOptions {
			options = append(options, huh.NewOption(option.Label, option.ID))
		}
		fields = append(fields, huh.NewMultiSelect[string]().
			Key("task-runtime-tasks").
			Title("Specific Tasks").
			Description("Select individual tasks for exceptions or one-off runtime choices.").
			Filterable(true).
			Options(options...).
			Value(&f.selectedTasks))
	}
	return fields
}

func indexTaskRuntimeRules(
	rules []model.TaskRuntimeRule,
) (map[string]model.TaskRuntimeRule, map[string]model.TaskRuntimeRule) {
	typeRuleByValue := make(map[string]model.TaskRuntimeRule)
	taskRuleByID := make(map[string]model.TaskRuntimeRule)
	for _, rule := range rules {
		switch {
		case rule.Type != nil:
			typeRuleByValue[strings.TrimSpace(*rule.Type)] = rule
		case rule.ID != nil:
			taskRuleByID[strings.TrimSpace(*rule.ID)] = rule
		}
	}
	return typeRuleByValue, taskRuleByID
}

func (f *startTaskRuntimeForm) populate(
	entries []model.IssueEntry,
	typeRuleByValue map[string]model.TaskRuntimeRule,
	taskRuleByID map[string]model.TaskRuntimeRule,
) error {
	seenTypes := make(map[string]struct{})
	for _, entry := range entries {
		if err := f.addEntry(entry, seenTypes, typeRuleByValue, taskRuleByID); err != nil {
			return err
		}
	}
	return nil
}

func (f *startTaskRuntimeForm) addEntry(
	entry model.IssueEntry,
	seenTypes map[string]struct{},
	typeRuleByValue map[string]model.TaskRuntimeRule,
	taskRuleByID map[string]model.TaskRuntimeRule,
) error {
	taskData, err := tasks.ParseTaskFile(entry.Content)
	if err != nil {
		return tasks.WrapParseError(entry.AbsPath, err)
	}

	taskType := strings.TrimSpace(taskData.TaskType)
	f.addTypeOption(taskType, seenTypes, typeRuleByValue)

	id := strings.TrimSpace(entry.CodeFile)
	f.taskOptions = append(f.taskOptions, taskRuntimeTaskOption{
		ID:    id,
		Type:  taskType,
		Label: formatTaskRuntimeTaskLabel(entry.CodeFile, taskData.Title, taskType),
	})
	if rule, ok := taskRuleByID[id]; ok {
		f.selectedTasks = append(f.selectedTasks, id)
		f.taskEditors[id] = taskRuntimeEditorFromRule(rule)
	}
	return nil
}

func (f *startTaskRuntimeForm) addTypeOption(
	taskType string,
	seenTypes map[string]struct{},
	typeRuleByValue map[string]model.TaskRuntimeRule,
) {
	if taskType == "" {
		return
	}
	if _, ok := seenTypes[taskType]; !ok {
		f.typeOptions = append(f.typeOptions, taskRuntimeTypeOption{
			Value: taskType,
			Label: taskType,
		})
		seenTypes[taskType] = struct{}{}
	}
	if rule, ok := typeRuleByValue[taskType]; ok && !slices.Contains(f.selectedTypes, taskType) {
		f.selectedTypes = append(f.selectedTypes, taskType)
		f.typeEditors[taskType] = taskRuntimeEditorFromRule(rule)
	}
}

func (f *startTaskRuntimeForm) ensureEditors() {
	for _, opt := range f.typeOptions {
		if _, ok := f.typeEditors[opt.Value]; !ok {
			f.typeEditors[opt.Value] = &taskRuntimeEditor{}
		}
	}
	for _, opt := range f.taskOptions {
		if _, ok := f.taskEditors[opt.ID]; !ok {
			f.taskEditors[opt.ID] = &taskRuntimeEditor{}
		}
	}
}

func (f *startTaskRuntimeForm) apply(state *commandState) {
	state.replaceConfiguredTaskRunRules = true
	state.executionTaskRuntimeRules = nil

	for _, selectedType := range f.selectedTypes {
		rule := buildTaskRuntimeRuleForType(selectedType, f.typeEditors[selectedType])
		if rule.HasOverride() {
			state.executionTaskRuntimeRules = append(state.executionTaskRuntimeRules, rule)
		}
	}
	for _, selectedTask := range f.selectedTasks {
		rule := buildTaskRuntimeRuleForTask(selectedTask, f.taskEditors[selectedTask])
		if rule.HasOverride() {
			state.executionTaskRuntimeRules = append(state.executionTaskRuntimeRules, rule)
		}
	}
}

func buildTaskRuntimeRuleForType(taskType string, editor *taskRuntimeEditor) model.TaskRuntimeRule {
	rule := model.TaskRuntimeRule{Type: stringPointer(strings.TrimSpace(taskType))}
	applyTaskRuntimeEditor(&rule, editor)
	return rule
}

func buildTaskRuntimeRuleForTask(taskID string, editor *taskRuntimeEditor) model.TaskRuntimeRule {
	rule := model.TaskRuntimeRule{ID: stringPointer(strings.TrimSpace(taskID))}
	applyTaskRuntimeEditor(&rule, editor)
	return rule
}

func applyTaskRuntimeEditor(rule *model.TaskRuntimeRule, editor *taskRuntimeEditor) {
	if rule == nil || editor == nil {
		return
	}
	if ide := strings.TrimSpace(editor.IDE); ide != "" {
		rule.IDE = stringPointer(ide)
	}
	if modelName := strings.TrimSpace(editor.Model); modelName != "" {
		rule.Model = stringPointer(modelName)
	}
	if reasoning := strings.TrimSpace(editor.ReasoningEffort); reasoning != "" {
		rule.ReasoningEffort = stringPointer(reasoning)
	}
}

func taskRuntimeEditorFromRule(rule model.TaskRuntimeRule) *taskRuntimeEditor {
	editor := &taskRuntimeEditor{}
	if rule.IDE != nil {
		editor.IDE = strings.TrimSpace(*rule.IDE)
	}
	if rule.Model != nil {
		editor.Model = strings.TrimSpace(*rule.Model)
	}
	if rule.ReasoningEffort != nil {
		editor.ReasoningEffort = strings.TrimSpace(*rule.ReasoningEffort)
	}
	return editor
}

func taskRuntimeIDEField(key, title, description string, target *string) huh.Field {
	return huh.NewSelect[string]().
		Key(key).
		Title(title).
		Description(description).
		Options(taskRuntimeIDEOptions()...).
		Value(target)
}

func taskRuntimeModelField(key string, target *string) huh.Field {
	return huh.NewInput().
		Key(key).
		Title("Model").
		Placeholder("inherit default").
		Description("Leave empty to inherit from the current default or type rule.").
		Value(target)
}

func taskRuntimeReasoningField(key, title, description string, target *string) huh.Field {
	return huh.NewSelect[string]().
		Key(key).
		Title(title).
		Description(description).
		Options(taskRuntimeReasoningOptions()...).
		Value(target)
}

func taskRuntimeIDEOptions() []huh.Option[string] {
	options := []huh.Option[string]{huh.NewOption("Inherit default", "")}
	options = append(options, ideCatalogOptions()...)
	return options
}

func taskRuntimeReasoningOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Inherit default", ""),
		huh.NewOption("Low", "low"),
		huh.NewOption("Medium", "medium"),
		huh.NewOption("High", "high"),
		huh.NewOption("Extra High", "xhigh"),
	}
}

func taskRuntimeSelectionDescription(baseRuntime string) string {
	return "Base runtime: " + baseRuntime +
		"\nType rules apply before task-specific rules. Leave fields as inherit/blank to keep the base runtime."
}

func formatBaseTaskRuntime(state *commandState) string {
	if state == nil {
		return ""
	}
	label := strings.TrimSpace(state.ide)
	modelName := strings.TrimSpace(state.model)
	reasoning := strings.TrimSpace(state.reasoningEffort)
	parts := make([]string, 0, 3)
	if label != "" {
		parts = append(parts, label)
	}
	if modelName != "" {
		parts = append(parts, "model "+modelName)
	}
	if reasoning != "" {
		parts = append(parts, "reasoning "+reasoning)
	}
	return strings.Join(parts, " · ")
}

func formatTaskRuntimeTaskLabel(id, title, taskType string) string {
	labelTitle := strings.TrimSpace(title)
	if labelTitle == "" {
		labelTitle = id
	}
	if strings.TrimSpace(taskType) == "" {
		return fmt.Sprintf("%s — %s", id, labelTitle)
	}
	return fmt.Sprintf("%s — %s [%s]", id, labelTitle, taskType)
}
