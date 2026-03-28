package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	core "github.com/compozy/looper/internal/looper"
	"github.com/spf13/cobra"
)

func collectFormParams(cmd *cobra.Command) error {
	fmt.Println("\n🎯 Interactive Parameter Collection")
	inputs := newFormInputs()
	builder := newFormBuilder(cmd)
	inputs.register(builder)
	if err := builder.build().Run(); err != nil {
		return fmt.Errorf("form canceled or error: %w", err)
	}
	inputs.apply(cmd)
	fmt.Println("\n✅ Parameters collected successfully!")
	return nil
}

type formInputs struct {
	pr              string
	issuesDir       string
	concurrent      string
	batchSize       string
	ide             string
	model           string
	addDirs         string
	tailLines       string
	reasoningEffort string
	mode            string
	timeout         string
}

func newFormInputs() *formInputs {
	return &formInputs{}
}

func (fi *formInputs) register(builder *formBuilder) {
	builder.addModeField(&fi.mode)
	builder.addPRField(&fi.pr)
	builder.addOptionalPathField("issues-dir", &fi.issuesDir)
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
		&dryRun,
	)
	builder.addConfirmField(
		"grouped",
		"Generate Grouped Summaries?",
		"Create grouped issue summaries in issues/grouped/",
		&grouped,
	)
	builder.addConfirmField(
		"auto-commit",
		"Auto Commit?",
		"Include commit instructions at task/batch completion",
		&autoCommit,
	)
	builder.addIncludeCompletedField(&includeCompleted)
}

func (fi *formInputs) apply(cmd *cobra.Command) {
	applyStringInput(cmd, "mode", fi.mode, func(val string) { mode = val })
	applyStringInput(cmd, "pr", fi.pr, func(val string) { pr = val })
	applyStringInput(cmd, "issues-dir", fi.issuesDir, func(val string) { issuesDir = val })
	applyIntInput(cmd, "concurrent", fi.concurrent, func(val int) { concurrent = val })
	applyIntInput(cmd, "batch-size", fi.batchSize, func(val int) { batchSize = val })
	applyStringInput(cmd, "ide", fi.ide, func(val string) { ide = val })
	applyStringInput(cmd, "model", fi.model, func(val string) { model = val })
	applyStringSliceInput(cmd, "add-dir", fi.addDirs, func(val []string) { addDirs = val })
	applyIntInput(cmd, "tail-lines", fi.tailLines, func(val int) { tailLines = val })
	applyStringInput(cmd, "reasoning-effort", fi.reasoningEffort, func(val string) {
		reasoningEffort = val
	})
	applyStringInput(cmd, "timeout", fi.timeout, func(val string) { timeout = val })
}

type formBuilder struct {
	cmd    *cobra.Command
	fields []huh.Field
}

func newFormBuilder(cmd *cobra.Command) *formBuilder {
	return &formBuilder{cmd: cmd}
}

func (fb *formBuilder) build() *huh.Form {
	return huh.NewForm(huh.NewGroup(fb.fields...)).WithTheme(huh.ThemeCharm())
}

func (fb *formBuilder) addField(flag string, build func() huh.Field) {
	if fb.cmd.Flags().Changed(flag) {
		return
	}
	field := build()
	if field != nil {
		fb.fields = append(fb.fields, field)
	}
}

func (fb *formBuilder) addModeField(target *string) {
	fb.addField("mode", func() huh.Field {
		return huh.NewSelect[string]().
			Title("Execution Mode").
			Description("Choose what to process").
			Options(
				huh.NewOption("PR Review Issues (CodeRabbit)", string(core.ModePRReview)),
				huh.NewOption("PRD Task Files", string(core.ModePRDTasks)),
			).
			Value(target)
	})
}

func (fb *formBuilder) addPRField(target *string) {
	fb.addField("pr", func() huh.Field {
		title := "PR Number"
		placeholder := "259"
		description := "Required: Pull request number or identifier to process"
		errorMsg := "PR number is required"
		if mode == string(core.ModePRDTasks) {
			title = "Task Identifier"
			placeholder = "multi-repo"
			description = "Required: Task name/identifier (e.g., 'multi-repo' for tasks/prd-multi-repo)"
			errorMsg = "Task identifier is required"
		}
		return huh.NewInput().
			Title(title).
			Placeholder(placeholder).
			Description(description).
			Value(target).
			Validate(func(str string) error {
				if str == "" {
					return errors.New(errorMsg)
				}
				return nil
			})
	})
}

func (fb *formBuilder) addOptionalPathField(flag string, target *string) {
	fb.addField(flag, func() huh.Field {
		title := "Issues Directory (optional)"
		placeholder := "ai-docs/reviews-pr-<PR>/issues"
		description := "Leave empty to auto-generate from PR number"
		if mode == string(core.ModePRDTasks) {
			title = "Tasks Directory (optional)"
			placeholder = "tasks/prd-<name>"
			description = "Leave empty to auto-generate from task identifier"
		}
		return huh.NewInput().
			Title(title).
			Placeholder(placeholder).
			Description(description).
			Value(target)
	})
}

func (fb *formBuilder) addConcurrentField(target *string) {
	fb.addField("concurrent", func() huh.Field {
		return numericInput(
			"Concurrent Jobs",
			"1",
			"Number of batches to process in parallel (1-10)",
			target,
			1,
			10,
			true,
		)
	})
}

func (fb *formBuilder) addBatchSizeField(target *string) {
	fb.addField("batch-size", func() huh.Field {
		if mode == string(core.ModePRDTasks) {
			*target = "1"
			return nil
		}
		return numericInput(
			"Batch Size",
			"1",
			"Number of file groups per batch (1-50)",
			target,
			1,
			50,
			true,
		)
	})
}

func (fb *formBuilder) addIDEField(target *string) {
	fb.addField("ide", func() huh.Field {
		return huh.NewSelect[string]().
			Title("IDE Tool").
			Description("Choose which IDE tool to use").
			Options(
				huh.NewOption("Codex (recommended)", string(core.IDECodex)),
				huh.NewOption("Claude", string(core.IDEClaude)),
				huh.NewOption("Cursor", string(core.IDECursor)),
				huh.NewOption("Droid", string(core.IDEDroid)),
			).
			Value(target)
	})
}

func (fb *formBuilder) addModelField(target *string) {
	fb.addField("model", func() huh.Field {
		return huh.NewInput().
			Title("Model (optional)").
			Placeholder("auto").
			Description("Specific model to use (default: gpt-5.4 for codex/droid, opus for claude)").
			Value(target)
	})
}

func (fb *formBuilder) addAddDirsField(target *string) {
	fb.addField("add-dir", func() huh.Field {
		return huh.NewInput().
			Title("Additional Directories (optional)").
			Placeholder("../shared, ../docs").
			Description("Comma-separated directories to pass via --add-dir for Codex and Claude only").
			Value(target)
	})
}

func (fb *formBuilder) addTailLinesField(target *string) {
	fb.addField("tail-lines", func() huh.Field {
		return numericInput(
			"Log Tail Lines",
			"5",
			"Number of log lines to show in UI (1-100)",
			target,
			1,
			100,
			true,
		)
	})
}

func (fb *formBuilder) addReasoningEffortField(target *string) {
	fb.addField("reasoning-effort", func() huh.Field {
		return huh.NewSelect[string]().
			Title("Reasoning Effort").
			Description("Model reasoning effort level (applies to Codex, Claude, and Droid)").
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
			Title(title).
			Description(description).
			Value(target)
	})
}

func (fb *formBuilder) addIncludeCompletedField(target *bool) {
	fb.addField("include-completed", func() huh.Field {
		if mode != string(core.ModePRDTasks) {
			return nil
		}
		return huh.NewConfirm().
			Title("Include Completed Tasks?").
			Description("Process tasks marked as completed").
			Value(target)
	})
}

func numericInput(
	title string,
	placeholder string,
	description string,
	target *string,
	minVal int,
	maxVal int,
	allowEmpty bool,
) huh.Field {
	return huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Description(description).
		Value(target).
		Validate(func(str string) error {
			if str == "" {
				if allowEmpty {
					return nil
				}
				return errors.New("value is required")
			}
			val, err := strconv.Atoi(str)
			if err != nil {
				return errors.New("must be a number")
			}
			if val < minVal || val > maxVal {
				return fmt.Errorf("must be between %d and %d", minVal, maxVal)
			}
			return nil
		})
}

func applyStringInput(cmd *cobra.Command, flagName, value string, setter func(string)) {
	if cmd.Flags().Changed(flagName) || value == "" {
		return
	}
	setter(value)
}

func applyIntInput(cmd *cobra.Command, flagName, value string, setter func(int)) {
	if cmd.Flags().Changed(flagName) || value == "" {
		return
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		return
	}
	setter(val)
}

func applyStringSliceInput(cmd *cobra.Command, flagName, value string, setter func([]string)) {
	if cmd.Flags().Changed(flagName) || value == "" {
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
