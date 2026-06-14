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
	Key      string
	Workflow string
	Value    string
	Label    string
}

type taskRuntimeTaskOption struct {
	Key      string
	Workflow string
	ID       string
	Type     string
	Label    string
}

type taskRunRuntimeForm struct {
	selectedTypes []string
	selectedTasks []string
	typeOptions   []taskRuntimeTypeOption
	taskOptions   []taskRuntimeTaskOption
	typeEditors   map[string]*taskRuntimeEditor
	taskEditors   map[string]*taskRuntimeEditor
	baseRuntime   string
}

func collectTaskRunRuntimeForm(cmd *cobra.Command, state *commandState) error {
	return collectTaskRunRuntimeFormForSlugs(cmd, state, selectedTaskRunSlugs(state))
}

func collectTaskRunRuntimeFormForSlugs(cmd *cobra.Command, state *commandState, slugs []string) error {
	if state == nil || state.kind != commandKindTasksRun {
		return nil
	}

	form, err := newTaskRunRuntimeFormForSlugs(state, slugs)
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

func readTaskRuntimeFormEntries(tasksDir string, includeCompleted, recursive bool) ([]model.IssueEntry, error) {
	if recursive {
		return tasks.ReadTaskEntriesRecursive(tasksDir, includeCompleted)
	}
	return tasks.ReadTaskEntries(tasksDir, includeCompleted)
}

func newTaskRunRuntimeForm(state *commandState) (*taskRunRuntimeForm, error) {
	return newTaskRunRuntimeFormForSlugs(state, selectedTaskRunSlugs(state))
}

func newTaskRunRuntimeFormForSlugs(state *commandState, slugs []string) (*taskRunRuntimeForm, error) {
	if state == nil {
		return nil, nil
	}
	scopeWorkflow := len(slugs) > 1
	form := &taskRunRuntimeForm{
		typeEditors: make(map[string]*taskRuntimeEditor),
		taskEditors: make(map[string]*taskRuntimeEditor),
		baseRuntime: formatBaseTaskRuntime(state),
	}
	typeRuleByValue, taskRuleByID := indexTaskRuntimeRules(state.taskRuntimeRules())
	for _, slug := range slugs {
		workflow := strings.TrimSpace(slug)
		if workflow == "" {
			continue
		}
		tasksDir, err := resolveTaskWorkflowDir(state.workspaceRoot, workflow, state.tasksDir)
		if err != nil {
			return nil, err
		}
		entries, err := readTaskRuntimeFormEntries(tasksDir, state.includeCompleted, state.recursive)
		if err != nil {
			return nil, fmt.Errorf("read task entries for runtime overrides in %s: %w", workflow, err)
		}
		if err := form.populate(workflow, entries, typeRuleByValue, taskRuleByID, scopeWorkflow); err != nil {
			return nil, err
		}
	}
	if len(form.typeOptions) == 0 && len(form.taskOptions) == 0 {
		return nil, nil
	}
	form.ensureEditors()
	return form, nil
}

func (f *taskRunRuntimeForm) build() *huh.Form {
	groups := []*huh.Group{
		huh.NewGroup(f.selectorFields()...).Title("Per-Task Runtime"),
	}
	for _, option := range f.typeOptions {
		option := option
		editor := f.typeEditors[option.Key]
		groups = append(groups, huh.NewGroup(
			taskRuntimeIDEField(
				"type-"+option.Key+"-ide",
				"IDE",
				"Override the runtime for this task type",
				&editor.IDE,
			),
			taskRuntimeModelField("type-"+option.Key+"-model", &editor.Model),
			taskRuntimeReasoningField(
				"type-"+option.Key+"-reasoning-effort",
				"Reasoning Effort",
				"Override reasoning for this task type",
				&editor.ReasoningEffort,
			),
		).Title("Type: "+option.Label).Description("Applies to every task with this type.").WithHideFunc(func() bool {
			return !slices.Contains(f.selectedTypes, option.Key)
		}))
	}
	for _, option := range f.taskOptions {
		option := option
		editor := f.taskEditors[option.Key]
		groups = append(groups, huh.NewGroup(
			taskRuntimeIDEField(
				"task-"+option.Key+"-ide",
				"IDE",
				"Override the runtime for this task only",
				&editor.IDE,
			),
			taskRuntimeModelField("task-"+option.Key+"-model", &editor.Model),
			taskRuntimeReasoningField(
				"task-"+option.Key+"-reasoning-effort",
				"Reasoning Effort",
				"Override reasoning for this task only",
				&editor.ReasoningEffort,
			),
		).Title("Task: "+option.Label).Description("Task-specific overrides win over type rules.").WithHideFunc(func() bool {
			return !slices.Contains(f.selectedTasks, option.Key)
		}))
	}
	return huh.NewForm(groups...).WithTheme(darkHuhTheme())
}

func (f *taskRunRuntimeForm) selectorFields() []huh.Field {
	fields := []huh.Field{
		huh.NewNote().
			Title("Task Runtime Overrides").
			Description(taskRuntimeSelectionDescription(f.baseRuntime)),
	}
	if len(f.typeOptions) > 0 {
		options := make([]huh.Option[string], 0, len(f.typeOptions))
		for _, option := range f.typeOptions {
			options = append(options, huh.NewOption(option.Label, option.Key))
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
			options = append(options, huh.NewOption(option.Label, option.Key))
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
		workflow := taskRuntimeRuleWorkflow(rule)
		switch {
		case rule.Type != nil:
			typeRuleByValue[taskRuntimeSelectorKey(workflow, strings.TrimSpace(*rule.Type))] = rule
		case rule.ID != nil:
			taskRuleByID[taskRuntimeSelectorKey(workflow, strings.TrimSpace(*rule.ID))] = rule
		}
	}
	return typeRuleByValue, taskRuleByID
}

func (f *taskRunRuntimeForm) populate(
	workflow string,
	entries []model.IssueEntry,
	typeRuleByValue map[string]model.TaskRuntimeRule,
	taskRuleByID map[string]model.TaskRuntimeRule,
	scopeWorkflow bool,
) error {
	seenTypes := make(map[string]struct{})
	for _, entry := range entries {
		if err := f.addEntry(workflow, entry, seenTypes, typeRuleByValue, taskRuleByID, scopeWorkflow); err != nil {
			return err
		}
	}
	return nil
}

func (f *taskRunRuntimeForm) addEntry(
	workflow string,
	entry model.IssueEntry,
	seenTypes map[string]struct{},
	typeRuleByValue map[string]model.TaskRuntimeRule,
	taskRuleByID map[string]model.TaskRuntimeRule,
	scopeWorkflow bool,
) error {
	taskData, err := tasks.ParseTaskFile(entry.Content)
	if err != nil {
		return tasks.WrapParseError(entry.AbsPath, err)
	}

	taskType := strings.TrimSpace(taskData.TaskType)
	f.addTypeOption(workflow, taskType, seenTypes, typeRuleByValue, scopeWorkflow)

	id := strings.TrimSpace(entry.CodeFile)
	optionWorkflow := taskRuntimeOptionWorkflow(workflow, scopeWorkflow)
	key := taskRuntimeSelectorKey(optionWorkflow, id)
	f.taskOptions = append(f.taskOptions, taskRuntimeTaskOption{
		Key:      key,
		Workflow: optionWorkflow,
		ID:       id,
		Type:     taskType,
		Label:    formatTaskRuntimeTaskLabel(optionWorkflow, entry.CodeFile, taskData.Title, taskType),
	})
	if rule, ok := selectTaskRuntimeRule(taskRuleByID, optionWorkflow, id); ok {
		f.selectedTasks = append(f.selectedTasks, key)
		f.taskEditors[key] = taskRuntimeEditorFromRule(rule)
	}
	return nil
}

func (f *taskRunRuntimeForm) addTypeOption(
	workflow string,
	taskType string,
	seenTypes map[string]struct{},
	typeRuleByValue map[string]model.TaskRuntimeRule,
	scopeWorkflow bool,
) {
	if taskType == "" {
		return
	}
	optionWorkflow := taskRuntimeOptionWorkflow(workflow, scopeWorkflow)
	key := taskRuntimeSelectorKey(optionWorkflow, taskType)
	if _, ok := seenTypes[key]; !ok {
		f.typeOptions = append(f.typeOptions, taskRuntimeTypeOption{
			Key:      key,
			Workflow: optionWorkflow,
			Value:    taskType,
			Label:    formatTaskRuntimeTypeLabel(optionWorkflow, taskType),
		})
		seenTypes[key] = struct{}{}
	}
	if rule, ok := selectTaskRuntimeRule(typeRuleByValue, optionWorkflow, taskType); ok &&
		!slices.Contains(f.selectedTypes, key) {
		f.selectedTypes = append(f.selectedTypes, key)
		f.typeEditors[key] = taskRuntimeEditorFromRule(rule)
	}
}

func (f *taskRunRuntimeForm) ensureEditors() {
	for _, opt := range f.typeOptions {
		if _, ok := f.typeEditors[opt.Key]; !ok {
			f.typeEditors[opt.Key] = &taskRuntimeEditor{}
		}
	}
	for _, opt := range f.taskOptions {
		if _, ok := f.taskEditors[opt.Key]; !ok {
			f.taskEditors[opt.Key] = &taskRuntimeEditor{}
		}
	}
}

func (f *taskRunRuntimeForm) apply(state *commandState) {
	state.replaceConfiguredTaskRunRules = true
	state.executionTaskRuntimeRules = nil

	for _, option := range f.typeOptions {
		if !slices.Contains(f.selectedTypes, option.Key) {
			continue
		}
		rule := buildTaskRuntimeRuleForType(option.Workflow, option.Value, f.typeEditors[option.Key])
		if rule.HasOverride() {
			state.executionTaskRuntimeRules = append(state.executionTaskRuntimeRules, rule)
		}
	}
	for _, option := range f.taskOptions {
		if !slices.Contains(f.selectedTasks, option.Key) {
			continue
		}
		rule := buildTaskRuntimeRuleForTask(option.Workflow, option.ID, f.taskEditors[option.Key])
		if rule.HasOverride() {
			state.executionTaskRuntimeRules = append(state.executionTaskRuntimeRules, rule)
		}
	}
}

func buildTaskRuntimeRuleForType(workflow string, taskType string, editor *taskRuntimeEditor) model.TaskRuntimeRule {
	rule := model.TaskRuntimeRule{Type: stringPointer(strings.TrimSpace(taskType))}
	if trimmedWorkflow := strings.TrimSpace(workflow); trimmedWorkflow != "" {
		rule.Workflow = stringPointer(trimmedWorkflow)
	}
	applyTaskRuntimeEditor(&rule, editor)
	return rule
}

func buildTaskRuntimeRuleForTask(workflow string, taskID string, editor *taskRuntimeEditor) model.TaskRuntimeRule {
	rule := model.TaskRuntimeRule{ID: stringPointer(strings.TrimSpace(taskID))}
	if trimmedWorkflow := strings.TrimSpace(workflow); trimmedWorkflow != "" {
		rule.Workflow = stringPointer(trimmedWorkflow)
	}
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

func taskRuntimeRuleWorkflow(rule model.TaskRuntimeRule) string {
	if rule.Workflow == nil {
		return ""
	}
	return strings.TrimSpace(*rule.Workflow)
}

func taskRuntimeOptionWorkflow(workflow string, scopeWorkflow bool) string {
	if !scopeWorkflow {
		return ""
	}
	return strings.TrimSpace(workflow)
}

func taskRuntimeSelectorKey(workflow string, selector string) string {
	trimmedSelector := strings.TrimSpace(selector)
	trimmedWorkflow := strings.TrimSpace(workflow)
	if trimmedWorkflow == "" {
		return trimmedSelector
	}
	return trimmedWorkflow + "::" + trimmedSelector
}

func selectTaskRuntimeRule(
	rules map[string]model.TaskRuntimeRule,
	workflow string,
	selector string,
) (model.TaskRuntimeRule, bool) {
	if rule, ok := rules[taskRuntimeSelectorKey(workflow, selector)]; ok {
		return rule, true
	}
	rule, ok := rules[taskRuntimeSelectorKey("", selector)]
	return rule, ok
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

func formatTaskRuntimeTypeLabel(workflow, taskType string) string {
	if strings.TrimSpace(workflow) == "" {
		return taskType
	}
	return workflow + " / " + taskType
}

func formatTaskRuntimeTaskLabel(workflow, id, title, taskType string) string {
	labelTitle := strings.TrimSpace(title)
	if labelTitle == "" {
		labelTitle = id
	}
	if strings.TrimSpace(workflow) != "" {
		labelTitle = workflow + " / " + labelTitle
	}
	if strings.TrimSpace(taskType) == "" {
		return fmt.Sprintf("%s — %s", id, labelTitle)
	}
	return fmt.Sprintf("%s — %s [%s]", id, labelTitle, taskType)
}
