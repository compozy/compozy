package run

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/core/tasks"

	tea "charm.land/bubbletea/v2"
)

var (
	validationFormInput  io.Reader = os.Stdin
	validationFormOutput io.Writer = os.Stdout
)

type PreflightDecision string

const (
	PreflightOK        PreflightDecision = "ok"
	PreflightContinued PreflightDecision = "continued"
	PreflightAborted   PreflightDecision = "aborted"
	PreflightSkipped   PreflightDecision = "skipped"
	PreflightForced    PreflightDecision = "forced"
)

type PreflightConfig struct {
	TasksDir       string
	Registry       *tasks.TypeRegistry
	IsInteractive  func() bool
	Force          bool
	SkipValidation bool
	Stderr         io.Writer
	Logger         *slog.Logger
	ValidationFn   func(context.Context, string, *tasks.TypeRegistry) (tasks.Report, error)
	ValidationForm func(tasks.Report, *tasks.TypeRegistry, io.Writer) (PreflightDecision, error)
}

func PreflightCheck(
	ctx context.Context,
	tasksDir string,
	registry *tasks.TypeRegistry,
	isInteractive func() bool,
	force bool,
) (PreflightDecision, error) {
	return PreflightCheckConfig(ctx, PreflightConfig{
		TasksDir:      tasksDir,
		Registry:      registry,
		IsInteractive: isInteractive,
		Force:         force,
	})
}

func PreflightCheckConfig(ctx context.Context, cfg PreflightConfig) (PreflightDecision, error) {
	if cfg.SkipValidation {
		logPreflightDecision(cfg.Logger, PreflightSkipped, "", tasks.Report{})
		return PreflightSkipped, nil
	}

	validate := cfg.ValidationFn
	if validate == nil {
		validate = tasks.Validate
	}
	form := cfg.ValidationForm
	if form == nil {
		form = runValidationForm
	}

	report, err := validate(ctx, cfg.TasksDir, cfg.Registry)
	if err != nil {
		return "", fmt.Errorf("run task metadata validation: %w", err)
	}
	if report.OK() {
		logPreflightDecision(cfg.Logger, PreflightOK, report.TasksDir, report)
		return PreflightOK, nil
	}
	if cfg.Force {
		logPreflightDecision(cfg.Logger, PreflightForced, report.TasksDir, report)
		return PreflightForced, nil
	}
	if isInteractive(cfg.IsInteractive) {
		decision, err := form(report, cfg.Registry, resolvePreflightStderr(cfg.Stderr))
		if err != nil {
			return "", err
		}
		logPreflightDecision(cfg.Logger, decision, report.TasksDir, report)
		return decision, nil
	}
	if err := writePreflightFailure(resolvePreflightStderr(cfg.Stderr), report, cfg.Registry); err != nil {
		return "", err
	}
	logPreflightDecision(cfg.Logger, PreflightAborted, report.TasksDir, report)
	return PreflightAborted, nil
}

func runValidationForm(report tasks.Report, registry *tasks.TypeRegistry, stderr io.Writer) (PreflightDecision, error) {
	model := newValidationFormModel(report, registry, stderr)
	program := tea.NewProgram(
		model,
		tea.WithInput(validationFormInput),
		tea.WithOutput(validationFormOutput),
		tea.WithoutSignalHandler(),
	)
	result, err := program.Run()
	if err != nil {
		return "", fmt.Errorf("run validation preflight form: %w", err)
	}

	typed, ok := result.(*validationFormModel)
	if !ok {
		return "", fmt.Errorf("unexpected validation form result type %T", result)
	}
	if typed.decision == "" {
		return PreflightAborted, nil
	}
	return typed.decision, nil
}

func writePreflightFailure(stderr io.Writer, report tasks.Report, registry *tasks.TypeRegistry) error {
	if stderr == nil {
		return nil
	}

	if _, err := fmt.Fprintf(
		stderr,
		"task validation failed: %d issue(s) across %d file(s)\n",
		len(report.Issues),
		distinctValidationIssuePaths(report.Issues),
	); err != nil {
		return fmt.Errorf("write preflight summary: %w", err)
	}

	currentPath := ""
	for _, issue := range report.Issues {
		if issue.Path != currentPath {
			currentPath = issue.Path
			if _, err := fmt.Fprintf(stderr, "\n%s\n", currentPath); err != nil {
				return fmt.Errorf("write preflight issue path: %w", err)
			}
		}
		if _, err := fmt.Fprintf(stderr, "- %s: %s\n", issue.Field, issue.Message); err != nil {
			return fmt.Errorf("write preflight issue: %w", err)
		}
	}

	prompt := tasks.FixPrompt(report, registry)
	if strings.TrimSpace(prompt) == "" {
		return nil
	}
	if _, err := fmt.Fprintf(stderr, "\nFix prompt:\n%s\n", prompt); err != nil {
		return fmt.Errorf("write preflight fix prompt: %w", err)
	}
	return nil
}

func logPreflightDecision(logger *slog.Logger, decision PreflightDecision, tasksDir string, report tasks.Report) {
	if logger == nil {
		return
	}
	logger.Info(
		"task metadata preflight",
		"preflight",
		string(decision),
		"tasks_dir",
		tasksDir,
		"issues",
		len(report.Issues),
		"scanned",
		report.Scanned,
	)
}

func resolvePreflightStderr(stderr io.Writer) io.Writer {
	if stderr != nil {
		return stderr
	}
	return os.Stderr
}

func isInteractive(fn func() bool) bool {
	if fn == nil {
		return false
	}
	return fn()
}
