package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/spf13/cobra"
)

func collectFormParams(cmd *cobra.Command, state *commandState) error {
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), renderFormIntro())
	inputs := newFormInputs()
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
	tailLines        string
	reasoningEffort  string
	timeout          string
	includeCompleted bool
	includeResolved  bool
	grouped          bool
	dryRun           bool
	autoCommit       bool
}

func newFormInputs() *formInputs {
	return &formInputs{}
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
	builder.addTailLinesField(&fi.tailLines)
	builder.addReasoningEffortField(&fi.reasoningEffort)
	builder.addTimeoutField(&fi.timeout)
	builder.addConfirmField(
		"dry-run",
		"Dry Run?",
		"Only generate prompts without running IDE tool",
		&fi.dryRun,
	)
	builder.addConfirmField(
		"grouped",
		"Generate Grouped Summaries?",
		"Create grouped issue summaries in reviews-NNN/grouped/",
		&fi.grouped,
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
	applyStringInput(cmd, "name", fi.name, func(val string) { state.name = val })
	applyStringInput(cmd, "pr", fi.pr, func(val string) { state.pr = val })
	applyStringInput(cmd, "provider", fi.provider, func(val string) { state.provider = val })
	applyIntInput(cmd, "round", fi.round, func(val int) { state.round = val })
	applyStringInput(cmd, "reviews-dir", fi.reviewsDir, func(val string) { state.reviewsDir = val })
	applyStringInput(cmd, "tasks-dir", fi.tasksDir, func(val string) { state.tasksDir = val })
	applyIntInput(cmd, "concurrent", fi.concurrent, func(val int) { state.concurrent = val })
	applyIntInput(cmd, "batch-size", fi.batchSize, func(val int) { state.batchSize = val })
	applyStringInput(cmd, "ide", fi.ide, func(val string) { state.ide = val })
	applyStringInput(cmd, "model", fi.model, func(val string) { state.model = val })
	applyStringSliceInput(cmd, "add-dir", fi.addDirs, func(val []string) { state.addDirs = val })
	applyIntInput(cmd, "tail-lines", fi.tailLines, func(val int) { state.tailLines = val })
	applyStringInput(cmd, "reasoning-effort", fi.reasoningEffort, func(val string) {
		state.reasoningEffort = val
	})
	applyStringInput(cmd, "timeout", fi.timeout, func(val string) { state.timeout = val })
	applyBoolInput(cmd, "dry-run", fi.dryRun, func(val bool) { state.dryRun = val })
	applyBoolInput(cmd, "grouped", fi.grouped, func(val bool) { state.grouped = val })
	applyBoolInput(cmd, "auto-commit", fi.autoCommit, func(val bool) { state.autoCommit = val })
	applyBoolInput(cmd, "include-completed", fi.includeCompleted, func(val bool) {
		state.includeCompleted = val
	})
	applyBoolInput(cmd, "include-resolved", fi.includeResolved, func(val bool) {
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
	return &formBuilder{cmd: cmd, state: state, tasksBaseDir: model.TasksBaseDir()}
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
		if fb.state.kind == commandKindStart || fb.state.kind == commandKindFixReviews {
			var dirs []string
			if fb.state.kind == commandKindStart {
				dirs = listStartTaskSubdirs(fb.tasksBaseDir)
			} else {
				dirs = listTaskSubdirs(fb.tasksBaseDir)
			}
			if len(dirs) > 0 {
				fb.nameFromDirList = true
				title := "Task Name"
				description := "Select the task directory to run"
				if fb.state.kind == commandKindFixReviews {
					title = "Workflow Name"
					description = "Select the workflow directory for review fixes"
				}
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
		}

		title := "Workflow Name"
		description := "Required: workflow name (for example: my-feature)"
		if fb.state.kind == commandKindStart {
			title = "Task Name"
			description = "Required: task workflow name (for example: multi-repo)"
		}
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
		return huh.NewSelect[string]().
			Key("provider").
			Title("Review Provider").
			Description("Choose which review provider to fetch from").
			Options(
				huh.NewOption("CodeRabbit", "coderabbit"),
			).
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
		return huh.NewSelect[string]().
			Key("ide").
			Title("IDE Tool").
			Description("Choose which ACP runtime to use (installed directly or available via a supported launcher).").
			Options(
				huh.NewOption("Codex (recommended)", string(core.IDECodex)),
				huh.NewOption("Claude", string(core.IDEClaude)),
				huh.NewOption("Cursor", string(core.IDECursor)),
				huh.NewOption("Droid", string(core.IDEDroid)),
				huh.NewOption("OpenCode", string(core.IDEOpenCode)),
				huh.NewOption("Pi", string(core.IDEPi)),
				huh.NewOption("Gemini", string(core.IDEGemini)),
			).
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
			Description("Comma-separated directories to pass via --add-dir for Codex and Claude only").
			Value(target)
	})
}

func (fb *formBuilder) addTailLinesField(target *string) {
	fb.addField("tail-lines", func() huh.Field {
		return numericInput(
			"tail-lines",
			"UI Log Retention",
			"0",
			"Maximum log lines to retain in UI per job (0 = full history)",
			target,
			0,
			0,
		)
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
				huh.NewOption("Medium (recommended)", "medium"),
				huh.NewOption("High", "high"),
				huh.NewOption("Extra High", "xhigh"),
			).
			Value(target)
	})
}

func (fb *formBuilder) addTimeoutField(target *string) {
	fb.addField("timeout", func() huh.Field {
		return huh.NewInput().
			Key("timeout").
			Title("Activity Timeout").
			Placeholder("10m").
			Description("Cancel job if no output received within this period (e.g., 5m, 30s)").
			Value(target).
			Validate(func(str string) error {
				if str == "" {
					return nil
				}
				_, err := time.ParseDuration(str)
				if err != nil {
					return errors.New("invalid duration format (e.g., 5m, 30s, 1h)")
				}
				return nil
			})
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

func applyStringInput(cmd *cobra.Command, flagName, value string, setter func(string)) {
	if cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) || value == "" {
		return
	}
	setter(value)
}

func applyIntInput(cmd *cobra.Command, flagName, value string, setter func(int)) {
	if cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) || value == "" {
		return
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		return
	}
	setter(val)
}

func applyBoolInput(cmd *cobra.Command, flagName string, value bool, setter func(bool)) {
	if cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) {
		return
	}
	setter(value)
}

func applyStringSliceInput(cmd *cobra.Command, flagName, value string, setter func([]string)) {
	if cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) || value == "" {
		return
	}
	values := parseAddDirInput(value)
	if len(values) == 0 {
		return
	}
	setter(values)
}

func parseAddDirInput(value string) []string {
	return core.NormalizeAddDirs(strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	}))
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

func listStartTaskSubdirs(baseDir string) []string {
	dirs := listTaskSubdirs(baseDir)
	if len(dirs) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		meta, err := tasks.RefreshTaskMeta(filepath.Join(baseDir, dir))
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
