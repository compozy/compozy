package run

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/tasks"
)

func TestPreflightCheckSkipValidationReturnsSkippedWithoutCallingValidator(t *testing.T) {
	t.Parallel()

	called := false
	var logs bytes.Buffer
	decision, err := PreflightCheckConfig(context.Background(), PreflightConfig{
		SkipValidation: true,
		ValidationFn: func(context.Context, string, *tasks.TypeRegistry) (tasks.Report, error) {
			called = true
			return tasks.Report{}, nil
		},
		Logger: testPreflightLogger(&logs),
	})
	if err != nil {
		t.Fatalf("preflight skip validation: %v", err)
	}
	if called {
		t.Fatal("expected skip validation to bypass the validator")
	}
	if got := decision; got != PreflightSkipped {
		t.Fatalf("expected skipped decision, got %q", got)
	}
	if got := logs.String(); !strings.Contains(got, "preflight=skipped") {
		t.Fatalf("expected skipped log entry, got %q", got)
	}
}

func TestPreflightCheckCleanReportReturnsOK(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	decision, err := PreflightCheckConfig(context.Background(), PreflightConfig{
		TasksDir: "/tmp/tasks",
		Registry: testValidationRegistry(t),
		ValidationFn: func(context.Context, string, *tasks.TypeRegistry) (tasks.Report, error) {
			return tasks.Report{TasksDir: "/tmp/tasks", Scanned: 2}, nil
		},
		Logger: testPreflightLogger(&logs),
	})
	if err != nil {
		t.Fatalf("preflight ok: %v", err)
	}
	if got := decision; got != PreflightOK {
		t.Fatalf("expected ok decision, got %q", got)
	}
	if got := logs.String(); !strings.Contains(got, "preflight=ok") {
		t.Fatalf("expected ok log entry, got %q", got)
	}
}

func TestPreflightCheckNonInteractiveWithoutForceWritesFixPromptAndAborts(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	decision, err := PreflightCheckConfig(context.Background(), PreflightConfig{
		TasksDir:      "/tmp/tasks",
		Registry:      testValidationRegistry(t),
		IsInteractive: func() bool { return false },
		Stderr:        &stderr,
		ValidationFn: func(context.Context, string, *tasks.TypeRegistry) (tasks.Report, error) {
			return testValidationReport(), nil
		},
	})
	if err != nil {
		t.Fatalf("preflight non-interactive abort: %v", err)
	}
	if got := decision; got != PreflightAborted {
		t.Fatalf("expected aborted decision, got %q", got)
	}
	got := stderr.String()
	for _, want := range []string{"task validation failed", "Fix prompt:", "title is required"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, got)
		}
	}
}

func TestPreflightCheckNonInteractiveWithForceReturnsForced(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	var stderr bytes.Buffer
	decision, err := PreflightCheckConfig(context.Background(), PreflightConfig{
		TasksDir:      "/tmp/tasks",
		Registry:      testValidationRegistry(t),
		IsInteractive: func() bool { return false },
		Force:         true,
		Stderr:        &stderr,
		Logger:        testPreflightLogger(&logs),
		ValidationFn: func(context.Context, string, *tasks.TypeRegistry) (tasks.Report, error) {
			return testValidationReport(), nil
		},
	})
	if err != nil {
		t.Fatalf("preflight forced: %v", err)
	}
	if got := decision; got != PreflightForced {
		t.Fatalf("expected forced decision, got %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected forced path not to print fix prompt, got %q", got)
	}
	if got := logs.String(); !strings.Contains(got, "preflight=forced") {
		t.Fatalf("expected forced log entry, got %q", got)
	}
}

func TestPreflightCheckInteractiveUsesValidationFormDecision(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	decision, err := PreflightCheckConfig(context.Background(), PreflightConfig{
		TasksDir:      "/tmp/tasks",
		Registry:      testValidationRegistry(t),
		IsInteractive: func() bool { return true },
		Logger:        testPreflightLogger(&logs),
		ValidationFn: func(context.Context, string, *tasks.TypeRegistry) (tasks.Report, error) {
			return testValidationReport(), nil
		},
		ValidationForm: func(tasks.Report, *tasks.TypeRegistry, io.Writer) (PreflightDecision, error) {
			return PreflightContinued, nil
		},
	})
	if err != nil {
		t.Fatalf("preflight interactive: %v", err)
	}
	if got := decision; got != PreflightContinued {
		t.Fatalf("expected continued decision, got %q", got)
	}
	if got := logs.String(); !strings.Contains(got, "preflight=continued") {
		t.Fatalf("expected continued log entry, got %q", got)
	}
}

func TestPreflightCheckWrapperUsesActualValidator(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writePreflightTask(t, tasksDir, "task_01.md", strings.Join([]string{
		"---",
		"status: pending",
		"title: Valid Title",
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# Task 1: Valid Title",
		"",
		"Body.",
		"",
	}, "\n"))

	decision, err := PreflightCheck(
		context.Background(),
		tasksDir,
		testValidationRegistry(t),
		func() bool { return false },
		false,
	)
	if err != nil {
		t.Fatalf("preflight wrapper: %v", err)
	}
	if got := decision; got != PreflightOK {
		t.Fatalf("expected ok decision, got %q", got)
	}
}

func TestRunValidationFormUsesInjectedInputAndOutput(t *testing.T) {
	t.Parallel()

	originalInput := validationFormInput
	originalOutput := validationFormOutput
	t.Cleanup(func() {
		validationFormInput = originalInput
		validationFormOutput = originalOutput
	})

	validationFormInput = strings.NewReader("c")
	validationFormOutput = &bytes.Buffer{}

	decision, err := runValidationForm(testValidationReport(), testValidationRegistry(t), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run validation form: %v", err)
	}
	if got := decision; got != PreflightContinued {
		t.Fatalf("expected continued decision, got %q", got)
	}
}

func TestWritePreflightFailureRendersSummaryAndFixPrompt(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	if err := writePreflightFailure(&stderr, testValidationReport(), testValidationRegistry(t)); err != nil {
		t.Fatalf("write preflight failure: %v", err)
	}

	got := stderr.String()
	for _, want := range []string{"task validation failed", "Fix prompt:", "/tmp/tasks/task_01.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, got)
		}
	}
}

func TestResolvePreflightStderrAndIsInteractiveHelpers(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	if got := resolvePreflightStderr(buf); got != buf {
		t.Fatalf("expected explicit stderr writer to be preserved")
	}
	if got := resolvePreflightStderr(nil); got != os.Stderr {
		t.Fatalf("expected nil stderr to fall back to os.Stderr")
	}
	if isInteractive(nil) {
		t.Fatal("expected nil interactive callback to be false")
	}
	if !isInteractive(func() bool { return true }) {
		t.Fatal("expected interactive callback to be honored")
	}
	if err := writePreflightFailure(nil, testValidationReport(), testValidationRegistry(t)); err != nil {
		t.Fatalf("expected nil stderr writer to be ignored, got %v", err)
	}
}

func testPreflightLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

func writePreflightTask(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write preflight task %s: %v", name, err)
	}
}
