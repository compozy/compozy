package preflight

import (
	"bytes"
	"context"
	"fmt"
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
	decision, err := CheckConfig(context.Background(), Config{
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
	if got := decision; got != Skipped {
		t.Fatalf("expected skipped decision, got %q", got)
	}
	if got := logs.String(); !strings.Contains(got, "preflight=skipped") {
		t.Fatalf("expected skipped log entry, got %q", got)
	}
}

func TestPreflightCheckCleanReportReturnsOK(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	decision, err := CheckConfig(context.Background(), Config{
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
	if got := decision; got != OK {
		t.Fatalf("expected ok decision, got %q", got)
	}
	if got := logs.String(); !strings.Contains(got, "preflight=ok") {
		t.Fatalf("expected ok log entry, got %q", got)
	}
}

func TestPreflightCheckNonInteractiveWithoutForceWritesFixPromptAndAborts(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	decision, err := CheckConfig(context.Background(), Config{
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
	if got := decision; got != Aborted {
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
	decision, err := CheckConfig(context.Background(), Config{
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
	if got := decision; got != Forced {
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
	decision, err := CheckConfig(context.Background(), Config{
		TasksDir:      "/tmp/tasks",
		Registry:      testValidationRegistry(t),
		IsInteractive: func() bool { return true },
		Logger:        testPreflightLogger(&logs),
		ValidationFn: func(context.Context, string, *tasks.TypeRegistry) (tasks.Report, error) {
			return testValidationReport(), nil
		},
		ValidationForm: func(tasks.Report, *tasks.TypeRegistry, io.Writer) (Decision, error) {
			return Continued, nil
		},
	})
	if err != nil {
		t.Fatalf("preflight interactive: %v", err)
	}
	if got := decision; got != Continued {
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

	decision, err := Check(
		context.Background(),
		tasksDir,
		testValidationRegistry(t),
		func() bool { return false },
		false,
	)
	if err != nil {
		t.Fatalf("preflight wrapper: %v", err)
	}
	if got := decision; got != OK {
		t.Fatalf("expected ok decision, got %q", got)
	}
}

func TestPreflightCheckScopesValidationToRecursiveMode(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writePreflightTask(t, tasksDir, "task_01.md", strings.Join([]string{
		"---",
		"status: pending",
		"title: Root",
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# Root",
		"",
		"Body.",
		"",
	}, "\n"))
	nestedDir := filepath.Join(tasksDir, "features", "auth")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writePreflightTask(t, nestedDir, "task_01.md", strings.Join([]string{
		"---",
		"status: pending",
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# Nested Draft",
		"",
		"Body.",
		"",
	}, "\n"))

	flatDecision, err := CheckConfig(context.Background(), Config{
		TasksDir: tasksDir,
		Registry: testValidationRegistry(t),
	})
	if err != nil {
		t.Fatalf("flat preflight: %v", err)
	}
	if flatDecision != OK {
		t.Fatalf("expected flat preflight to ignore nested draft, got %q", flatDecision)
	}

	var stderr bytes.Buffer
	recursiveDecision, err := CheckConfig(context.Background(), Config{
		TasksDir:  tasksDir,
		Registry:  testValidationRegistry(t),
		Recursive: true,
		Stderr:    &stderr,
	})
	if err != nil {
		t.Fatalf("recursive preflight: %v", err)
	}
	if recursiveDecision != Aborted {
		t.Fatalf("expected recursive preflight to abort, got %q", recursiveDecision)
	}
	if got := stderr.String(); !strings.Contains(got, "title is required") {
		t.Fatalf("expected recursive validation failure in stderr, got %q", got)
	}
}

func TestRunValidationFormUsesInjectedInputAndOutput(t *testing.T) {
	t.Parallel()

	decision, err := runValidationFormWithIO(
		testValidationReport(),
		testValidationRegistry(t),
		&bytes.Buffer{},
		strings.NewReader("c"),
		&bytes.Buffer{},
		nil,
	)
	if err != nil {
		t.Fatalf("run validation form: %v", err)
	}
	if got := decision; got != Continued {
		t.Fatalf("expected continued decision, got %q", got)
	}
}

func TestRunValidationFormCopyPromptCopiesToClipboardAfterExit(t *testing.T) {
	t.Parallel()

	var copied string
	var stderr bytes.Buffer
	decision, err := runValidationFormWithIO(
		testValidationReport(),
		testValidationRegistry(t),
		&stderr,
		strings.NewReader("p"),
		&bytes.Buffer{},
		func(text string) error {
			copied = text
			return nil
		},
	)
	if err != nil {
		t.Fatalf("run validation form copy prompt: %v", err)
	}
	if got := decision; got != Aborted {
		t.Fatalf("expected aborted decision after copy prompt, got %q", got)
	}
	if !strings.Contains(copied, "Fix the Compozy task metadata files below.") {
		t.Fatalf("expected fix prompt copied to clipboard, got %q", copied)
	}
	if got := stderr.String(); !strings.Contains(got, "Fix prompt copied to clipboard.") {
		t.Fatalf("expected clipboard confirmation on stderr, got %q", got)
	}
	if strings.Contains(stderr.String(), "Fix the Compozy task metadata files below.") {
		t.Fatalf("expected clipboard success path not to dump prompt text to stderr, got %q", stderr.String())
	}
}

func TestRunValidationFormCopyPromptFallsBackToStderrWhenClipboardFails(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	decision, err := runValidationFormWithIO(
		testValidationReport(),
		testValidationRegistry(t),
		&stderr,
		strings.NewReader("p"),
		&bytes.Buffer{},
		func(string) error {
			return fmt.Errorf("clipboard unavailable")
		},
	)
	if err != nil {
		t.Fatalf("run validation form copy prompt fallback: %v", err)
	}
	if got := decision; got != Aborted {
		t.Fatalf("expected aborted decision after copy prompt fallback, got %q", got)
	}
	for _, want := range []string{
		"Unable to copy fix prompt to clipboard: clipboard unavailable",
		"Fix prompt:",
		"Fix the Compozy task metadata files below.",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected stderr fallback to contain %q, got %q", want, stderr.String())
		}
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
