package run

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/compozy/compozy/internal/core/tasks"

	tea "charm.land/bubbletea/v2"
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
	FormInput      io.Reader
	FormOutput     io.Writer
	ClipboardWrite func(string) error
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
		var decision PreflightDecision
		if form == nil {
			decision, err = runValidationFormWithIO(
				report,
				cfg.Registry,
				resolvePreflightStderr(cfg.Stderr),
				cfg.FormInput,
				cfg.FormOutput,
				cfg.ClipboardWrite,
			)
		} else {
			decision, err = form(report, cfg.Registry, resolvePreflightStderr(cfg.Stderr))
		}
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

func runValidationFormWithIO(
	report tasks.Report,
	registry *tasks.TypeRegistry,
	stderr io.Writer,
	input io.Reader,
	output io.Writer,
	clipboardWrite func(string) error,
) (PreflightDecision, error) {
	model := newValidationFormModel(report, registry, stderr, clipboardWrite)
	program := tea.NewProgram(
		model,
		tea.WithInput(resolveValidationFormInput(input)),
		tea.WithOutput(resolveValidationFormOutput(output)),
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
	if typed.shouldCopyFixPrompt {
		if err := typed.copyFixPrompt(); err != nil {
			return "", err
		}
	}
	if typed.decision == "" {
		return PreflightAborted, nil
	}
	return typed.decision, nil
}

func resolveValidationFormInput(input io.Reader) io.Reader {
	if input != nil {
		return input
	}
	return os.Stdin
}

func resolveValidationFormOutput(output io.Writer) io.Writer {
	if output != nil {
		return output
	}
	return os.Stdout
}

func (m *validationFormModel) copyFixPrompt() error {
	if strings.TrimSpace(m.fixPrompt) == "" {
		return nil
	}

	clipboardWriter := m.clipboardWrite
	if clipboardWriter == nil {
		clipboardWriter = clipboard.WriteAll
	}
	err := clipboardWriter(m.fixPrompt)
	if err == nil {
		if m.stderr == nil {
			return nil
		}
		if _, writeErr := fmt.Fprintln(m.stderr, "Fix prompt copied to clipboard."); writeErr != nil {
			return fmt.Errorf("write clipboard confirmation: %w", writeErr)
		}
		return nil
	} else if m.stderr != nil {
		if _, writeErr := fmt.Fprintf(
			m.stderr,
			"Unable to copy fix prompt to clipboard: %v\n\nFix prompt:\n%s\n",
			err,
			m.fixPrompt,
		); writeErr != nil {
			return fmt.Errorf("write validation fix prompt fallback: %w", writeErr)
		}
		return nil
	}

	return fmt.Errorf("copy validation fix prompt to clipboard: %w", err)
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
