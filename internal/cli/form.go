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

func collectFormParams(cmd *cobra.Command, state *commandState) error {
	fmt.Println("\n🎯 Interactive Parameter Collection")
	inputs := newFormInputs()
	builder := newFormBuilder(cmd, state)
	inputs.register(builder)
	if err := builder.build().Run(); err != nil {
		return fmt.Errorf("form canceled or error: %w", err)
	}
	inputs.apply(cmd, state)
	fmt.Println("\n✅ Parameters collected successfully!")
	return nil
}

type formInputs struct {
	identifier       string
	inputDir         string
	concurrent       string
	batchSize        string
	ide              string
	model            string
	addDirs          string
	tailLines        string
	reasoningEffort  string
	timeout          string
	includeCompleted bool
	grouped          bool
	dryRun           bool
	autoCommit       bool
}

func newFormInputs() *formInputs {
	return &formInputs{}
}

func (fi *formInputs) register(builder *formBuilder) {
	builder.addIdentifierField(&fi.identifier)
	builder.addInputDirField(&fi.inputDir)
	builder.addConcurrentField(&fi.concurrent)
	if !builder.state.isTaskWorkflow() {
		builder.addBatchSizeField(&fi.batchSize)
	}
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
	if !builder.state.isTaskWorkflow() {
		builder.addConfirmField(
			"grouped",
			"Generate Grouped Summaries?",
			"Create grouped issue summaries in issues/grouped/",
			&fi.grouped,
		)
	}
	builder.addConfirmField(
		"auto-commit",
		"Auto Commit?",
		"Include commit instructions at task/batch completion",
		&fi.autoCommit,
	)
	if builder.state.isTaskWorkflow() {
		builder.addIncludeCompletedField(&fi.includeCompleted)
	}
}

func (fi *formInputs) apply(cmd *cobra.Command, state *commandState) {
	applyStringInput(cmd, state.identifierFlagName(), fi.identifier, state.setIdentifierValue)
	applyStringInput(cmd, state.inputDirFlagName(), fi.inputDir, state.setInputDirValue)
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
}

type formBuilder struct {
	cmd    *cobra.Command
	state  *commandState
	fields []huh.Field
}

func newFormBuilder(cmd *cobra.Command, state *commandState) *formBuilder {
	return &formBuilder{cmd: cmd, state: state}
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

func (fb *formBuilder) addIdentifierField(target *string) {
	fb.addField(fb.state.identifierFlagName(), func() huh.Field {
		return huh.NewInput().
			Title(fb.state.identifierTitle()).
			Placeholder(fb.state.identifierPlaceholder()).
			Description(fb.state.identifierDescription()).
			Value(target).
			Validate(func(str string) error {
				if str == "" {
					return errors.New(fb.state.identifierRequiredMessage())
				}
				return nil
			})
	})
}

func (fb *formBuilder) addInputDirField(target *string) {
	fb.addField(fb.state.inputDirFlagName(), func() huh.Field {
		return huh.NewInput().
			Title(fb.state.inputDirTitle()).
			Placeholder(fb.state.inputDirPlaceholder()).
			Description(fb.state.inputDirDescription()).
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

func applyBoolInput(cmd *cobra.Command, flagName string, value bool, setter func(bool)) {
	if cmd.Flags().Changed(flagName) {
		return
	}
	setter(value)
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
