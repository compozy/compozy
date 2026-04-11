package cli

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/providerdefaults"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/spf13/cobra"
)

const workflowNameTitle = "Workflow Name"

func collectFormParams(cmd *cobra.Command, state *commandState) error {
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), renderFormIntro())
	inputs := newFormInputsFromState(state)
	builder := newFormBuilder(cmd, state)
	inputs.register(builder)
	if err := builder.build().Run(); err != nil {
		return fmt.Errorf("form canceled or error: %w", err)
	}
	inputs.apply(cmd, state)
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), renderFormSuccess())
	return nil
}

type formInputs struct {
	name             string
	pr               string
	provider         string
	round            string
	reviewsDir       string
	tasksDir         string
	concurrent       string
	batchSize        string
	ide              string
	model            string
	addDirs          string
	reasoningEffort  string
	includeCompleted bool
	includeResolved  bool
	nitpicks         bool
	dryRun           bool
	autoCommit       bool
}

func newFormInputs() *formInputs {
	return &formInputs{}
}

func newFormInputsFromState(state *commandState) *formInputs {
	inputs := newFormInputs()
	if state == nil {
		return inputs
	}

	inputs.name = state.name
	inputs.pr = state.pr
	inputs.provider = state.provider
	if state.round > 0 {
		inputs.round = strconv.Itoa(state.round)
	}
	inputs.nitpicks = state.nitpicks
	inputs.reviewsDir = state.reviewsDir
	inputs.tasksDir = state.tasksDir
	if state.concurrent > 0 {
		inputs.concurrent = strconv.Itoa(state.concurrent)
	}
	if state.batchSize > 0 {
		inputs.batchSize = strconv.Itoa(state.batchSize)
	}
	inputs.ide = state.ide
	inputs.model = state.model
	if len(state.addDirs) > 0 {
		inputs.addDirs = formatAddDirInput(state.addDirs)
	}
	inputs.reasoningEffort = state.reasoningEffort
	inputs.includeCompleted = state.includeCompleted
	inputs.includeResolved = state.includeResolved
	inputs.dryRun = state.dryRun
	inputs.autoCommit = state.autoCommit

	return inputs
}

func (fi *formInputs) register(builder *formBuilder) {
	builder.addNameField(&fi.name)
	builder.addPRField(&fi.pr)
	builder.addProviderField(&fi.provider)
	builder.addRoundField(&fi.round)
	builder.addReviewsDirField(&fi.reviewsDir)
	builder.addTasksDirField(&fi.tasksDir)
	builder.addConcurrentField(&fi.concurrent)
	builder.addBatchSizeField(&fi.batchSize)
	builder.addIDEField(&fi.ide)
	builder.addModelField(&fi.model)
	builder.addAddDirsField(&fi.addDirs)
	builder.addReasoningEffortField(&fi.reasoningEffort)
	builder.addConfirmField(
		"nitpicks",
		"Include Nitpicks?",
		"Import CodeRabbit review-body comments (nitpick, minor, and major)",
		&fi.nitpicks,
	)
	builder.addConfirmField(
		"dry-run",
		"Dry Run?",
		"Only generate prompts without running IDE tool",
		&fi.dryRun,
	)
	builder.addConfirmField(
		"auto-commit",
		"Auto Commit?",
		"Include commit instructions at task/batch completion",
		&fi.autoCommit,
	)
	builder.addConfirmField(
		"include-completed",
		"Include Completed Tasks?",
		"Process tasks marked as completed",
		&fi.includeCompleted,
	)
	builder.addConfirmField(
		"include-resolved",
		"Include Resolved Review Issues?",
		"Process issues already marked as resolved",
		&fi.includeResolved,
	)
}

func (fi *formInputs) apply(cmd *cobra.Command, state *commandState) {
	applyInput(cmd, "name", fi.name, passThroughInput[string], func(val string) { state.name = val })
	applyInput(cmd, "pr", fi.pr, passThroughInput[string], func(val string) { state.pr = val })
	applyInput(cmd, "provider", fi.provider, passThroughInput[string], func(val string) { state.provider = val })
	applyInput(cmd, "round", fi.round, parseIntInput, func(val int) { state.round = val })
	applyInput(cmd, "nitpicks", fi.nitpicks, passThroughInput[bool], func(val bool) { state.nitpicks = val })
	applyInput(cmd, "reviews-dir", fi.reviewsDir, passThroughInput[string], func(val string) { state.reviewsDir = val })
	applyInput(cmd, "tasks-dir", fi.tasksDir, passThroughInput[string], func(val string) { state.tasksDir = val })
	applyInput(cmd, "concurrent", fi.concurrent, parseIntInput, func(val int) { state.concurrent = val })
	applyInput(cmd, "batch-size", fi.batchSize, parseIntInput, func(val int) { state.batchSize = val })
	applyInput(cmd, "ide", fi.ide, passThroughInput[string], func(val string) { state.ide = val })
	applyInput(cmd, "model", fi.model, passThroughInput[string], func(val string) { state.model = val })
	applyInput(cmd, "add-dir", fi.addDirs, parseStringSliceInput, func(val []string) { state.addDirs = val })
	applyInput(cmd, "reasoning-effort", fi.reasoningEffort, passThroughInput[string], func(val string) {
		state.reasoningEffort = val
	})
	applyInput(cmd, "dry-run", fi.dryRun, passThroughInput[bool], func(val bool) { state.dryRun = val })
	applyInput(cmd, "auto-commit", fi.autoCommit, passThroughInput[bool], func(val bool) { state.autoCommit = val })
	applyInput(cmd, "include-completed", fi.includeCompleted, passThroughInput[bool], func(val bool) {
		state.includeCompleted = val
	})
	applyInput(cmd, "include-resolved", fi.includeResolved, passThroughInput[bool], func(val bool) {
		state.includeResolved = val
	})
}

type formBuilder struct {
	cmd             *cobra.Command
	state           *commandState
	fields          []huh.Field
	nameFromDirList bool
	tasksBaseDir    string
}

func newFormBuilder(cmd *cobra.Command, state *commandState) *formBuilder {
	return &formBuilder{
		cmd:          cmd,
		state:        state,
		tasksBaseDir: model.TasksBaseDirForWorkspace(state.workspaceRoot),
	}
}

func (fb *formBuilder) build() *huh.Form {
	return huh.NewForm(huh.NewGroup(fb.fields...)).WithTheme(darkHuhTheme())
}

func (fb *formBuilder) hasFlag(flag string) bool {
	return fb.cmd.Flags().Lookup(flag) != nil
}

func (fb *formBuilder) addField(flag string, build func() huh.Field) {
	if !fb.hasFlag(flag) || fb.cmd.Flags().Changed(flag) || fb.hideField(flag) {
		return
	}
	field := build()
	if field != nil {
		fb.fields = append(fb.fields, field)
	}
}

func (fb *formBuilder) hideField(flag string) bool {
	if flag == "tasks-dir" && fb.nameFromDirList {
		return true
	}

	switch fb.state.kind {
	case commandKindStart:
		switch flag {
		case "concurrent", "dry-run", "include-completed":
			return true
		}
	case commandKindFixReviews:
		switch flag {
		case "dry-run", "include-resolved":
			return true
		}
	}

	return false
}

func (fb *formBuilder) addNameField(target *string) {
	fb.addField("name", func() huh.Field {
		title, description, dirs := fb.nameFieldOptions()
		if len(dirs) > 0 {
			fb.nameFromDirList = true
			options := make([]huh.Option[string], 0, len(dirs))
			for _, d := range dirs {
				options = append(options, huh.NewOption(d, d))
			}
			return huh.NewSelect[string]().
				Key("name").
				Title(title).
				Description(description).
				Options(options...).
				Value(target)
		}

		title, description = fb.nameInputLabels()
		return huh.NewInput().
			Key("name").
			Title(title).
			Placeholder("my-feature").
			Description(description).
			Value(target).
			Validate(func(str string) error {
				if strings.TrimSpace(str) == "" && !fb.hasFlag("reviews-dir") {
					return errors.New("name is required")
				}
				return nil
			})
	})
}

func (fb *formBuilder) nameFieldOptions() (string, string, []string) {
	switch fb.state.kind {
	case commandKindStart:
		return "Task Name", "Select the task directory to run", listStartTaskSubdirs(fb.tasksBaseDir)
	case commandKindFixReviews:
		return workflowNameTitle, "Select the workflow directory for review fixes", listTaskSubdirs(fb.tasksBaseDir)
	case commandKindFetchReviews:
		return workflowNameTitle, "Select the workflow directory to fetch reviews into", listTaskSubdirs(
			fb.tasksBaseDir,
		)
	default:
		return "", "", nil
	}
}

func (fb *formBuilder) nameInputLabels() (string, string) {
	if fb.state.kind == commandKindStart {
		return "Task Name", "Required: task workflow name (for example: multi-repo)"
	}
	return workflowNameTitle, "Required: workflow name (for example: my-feature)"
}

func (fb *formBuilder) addPRField(target *string) {
	fb.addField("pr", func() huh.Field {
		return huh.NewInput().
			Key("pr").
			Title("Pull Request").
			Placeholder("259").
			Description("Required: pull request number to fetch reviews from").
			Value(target).
			Validate(func(str string) error {
				if strings.TrimSpace(str) == "" {
					return errors.New("pull request number is required")
				}
				return nil
			})
	})
}

func (fb *formBuilder) addProviderField(target *string) {
	fb.addField("provider", func() huh.Field {
		options := providerCatalogOptions()
		if len(options) == 0 {
			options = []huh.Option[string]{huh.NewOption("CodeRabbit", "coderabbit")}
		}
		return huh.NewSelect[string]().
			Key("provider").
			Title("Review Provider").
			Description("Choose which review provider to fetch from").
			Options(options...).
			Value(target)
	})
}

func (fb *formBuilder) addRoundField(target *string) {
	fb.addField("round", func() huh.Field {
		description := "Leave empty to auto-detect the appropriate round"
		if fb.state.kind == commandKindFetchReviews {
			description = "Leave empty to create the next available review round"
		}
		return numericInput(
			"round",
			"Review Round",
			"auto",
			description,
			target,
			1,
			999,
		)
	})
}

func (fb *formBuilder) addReviewsDirField(target *string) {
	fb.addField("reviews-dir", func() huh.Field {
		return huh.NewInput().
			Key("reviews-dir").
			Title("Reviews Directory (optional)").
			Placeholder(".compozy/tasks/<name>/reviews-NNN").
			Description("Leave empty to resolve from PRD name and round").
			Value(target)
	})
}

func (fb *formBuilder) addTasksDirField(target *string) {
	fb.addField("tasks-dir", func() huh.Field {
		return huh.NewInput().
			Key("tasks-dir").
			Title("Tasks Directory (optional)").
			Placeholder(".compozy/tasks/<name>").
			Description("Leave empty to auto-generate from task name").
			Value(target)
	})
}

func (fb *formBuilder) addConcurrentField(target *string) {
	fb.addField("concurrent", func() huh.Field {
		return numericInput(
			"concurrent",
			"Concurrent Jobs",
			"1",
			"Number of batches to process in parallel (1-10)",
			target,
			1,
			10,
		)
	})
}

func (fb *formBuilder) addBatchSizeField(target *string) {
	fb.addField("batch-size", func() huh.Field {
		return numericInput(
			"batch-size",
			"Batch Size",
			"1",
			"Number of file groups per batch (1-50)",
			target,
			1,
			50,
		)
	})
}

func (fb *formBuilder) addIDEField(target *string) {
	fb.addField("ide", func() huh.Field {
		options := ideCatalogOptions()
		return huh.NewSelect[string]().
			Key("ide").
			Title("IDE Tool").
			Description("Choose which ACP runtime to use (installed directly or available via a supported launcher).").
			Options(options...).
			Value(target)
	})
}

func (fb *formBuilder) addModelField(target *string) {
	fb.addField("model", func() huh.Field {
		return huh.NewInput().
			Key("model").
			Title("Model (optional)").
			Placeholder("auto").
			Description("Model override (defaults: codex/droid=gpt-5.4, " +
				"claude=opus, opencode/pi=anthropic/claude-opus-4-6, gemini=gemini-2.5-pro)").
			Value(target)
	})
}

func (fb *formBuilder) addAddDirsField(target *string) {
	fb.addField("add-dir", func() huh.Field {
		return huh.NewInput().
			Key("add-dir").
			Title("Additional Directories (optional)").
			Placeholder("../shared, ../docs").
			Description(
				"Comma-separated directories to pass via --add-dir for Claude and Codex only; quote paths that contain commas",
			).
			Value(target)
	})
}

func (fb *formBuilder) addReasoningEffortField(target *string) {
	fb.addField("reasoning-effort", func() huh.Field {
		return huh.NewSelect[string]().
			Key("reasoning-effort").
			Title("Reasoning Effort").
			Description("Model reasoning effort level (applies to Codex, Claude, Droid, OpenCode, and Pi)").
			Options(
				huh.NewOption("Low", "low"),
				huh.NewOption("Medium", "medium"),
				huh.NewOption("High", "high"),
				huh.NewOption("Extra High", "xhigh"),
			).
			Value(target)
	})
}

func (fb *formBuilder) addConfirmField(flag, title, description string, target *bool) {
	fb.addField(flag, func() huh.Field {
		return huh.NewConfirm().
			Key(flag).
			Title(title).
			Description(description).
			Value(target)
	})
}

func numericInput(
	key string,
	title string,
	placeholder string,
	description string,
	target *string,
	minVal int,
	maxVal int,
) huh.Field {
	return huh.NewInput().
		Key(key).
		Title(title).
		Placeholder(placeholder).
		Description(description).
		Value(target).
		Validate(func(str string) error {
			if str == "" {
				return nil
			}
			val, err := strconv.Atoi(str)
			if err != nil {
				return errors.New("must be a number")
			}
			if val < minVal {
				return fmt.Errorf("must be %d or greater", minVal)
			}
			if maxVal > 0 && val > maxVal {
				return fmt.Errorf("must be between %d and %d", minVal, maxVal)
			}
			return nil
		})
}

func applyInput[T any, V any](
	cmd *cobra.Command,
	flagName string,
	value V,
	parse func(V) (T, bool),
	setter func(T),
) {
	if cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) {
		return
	}
	resolved, ok := parse(value)
	if !ok {
		return
	}
	setter(resolved)
}

func passThroughInput[T any](value T) (T, bool) {
	return value, true
}

func parseIntInput(value string) (int, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, true
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return val, true
}

func parseStringSliceInput(value string) ([]string, bool) {
	return parseAddDirInput(value), true
}

func parseAddDirInput(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	reader := csv.NewReader(strings.NewReader(value))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err == nil {
		var values []string
		for _, record := range records {
			values = append(values, record...)
		}
		return core.NormalizeAddDirs(values)
	}

	return core.NormalizeAddDirs(strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	}))
}

func formatAddDirInput(values []string) string {
	if len(values) == 0 {
		return ""
	}

	formatted := make([]string, 0, len(values))
	for _, value := range values {
		formatted = append(formatted, formatCSVField(value))
	}
	return strings.Join(formatted, ", ")
}

func formatCSVField(value string) string {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	if err := writer.Write([]string{value}); err != nil {
		return value
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return value
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

func listTaskSubdirs(baseDir string) []string {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && model.IsActiveWorkflowDirName(e.Name()) {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

func ideCatalogOptions() []huh.Option[string] {
	entries := agent.DriverCatalog()
	options := make([]huh.Option[string], 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		label := strings.TrimSpace(entry.DisplayName)
		if label == "" {
			label = entry.IDE
		}
		options = append(options, huh.NewOption(label, entry.IDE))
	}
	return options
}

func providerCatalogOptions() []huh.Option[string] {
	entries := provider.Catalog(providerdefaults.DefaultRegistry())
	options := make([]huh.Option[string], 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		label := strings.TrimSpace(entry.DisplayName)
		if label == "" {
			label = entry.Name
		}
		options = append(options, huh.NewOption(label, entry.Name))
	}
	return options
}

func listStartTaskSubdirs(baseDir string) []string {
	dirs := listTaskSubdirs(baseDir)
	if len(dirs) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		meta, err := tasks.ReadTaskMeta(filepath.Join(baseDir, dir))
		if err != nil {
			filtered = append(filtered, dir)
			continue
		}
		if meta.Total > 0 && meta.Pending == 0 {
			continue
		}
		filtered = append(filtered, dir)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
