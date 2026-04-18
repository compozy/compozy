package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/core/tasks"
)

var (
	validateTasksBinaryOnce sync.Once
	validateTasksBinaryPath string
	validateTasksBinaryErr  error
)

func TestValidateTasksCommandJSONMixedFixture(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Valid One", "type: backend", "complexity: low"},
		"# Task 1: Valid One",
	))
	writeRawTaskFileForCLI(t, tasksDir, "task_02.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Valid Two", "type: docs", "complexity: medium"},
		"# Task 2: Valid Two",
	))
	invalidTitlePath := filepath.Join(tasksDir, "task_03.md")
	writeRawTaskFileForCLI(t, tasksDir, "task_03.md", cliTaskMarkdown(
		[]string{"status: pending", "type: backend", "complexity: low"},
		"# Task 3: Missing Title",
	))
	invalidTypePath := filepath.Join(tasksDir, "task_04.md")
	writeRawTaskFileForCLI(t, tasksDir, "task_04.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Invalid Type", "type: nope", "complexity: low"},
		"# Task 4: Invalid Type",
	))

	stdout, stderr, exitCode := runValidateTasksCommand(
		t,
		workspaceRoot,
		"tasks",
		"validate",
		"--tasks-dir",
		tasksDir,
		"--format",
		"json",
	)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	var payload validateTasksOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal json output: %v\nstdout:\n%s", err, stdout)
	}
	if payload.FixPrompt == "" {
		t.Fatal("expected non-empty fix_prompt")
	}

	gotPaths := distinctPaths(payload.Issues)
	wantPaths := []string{invalidTitlePath, invalidTypePath}
	slices.Sort(gotPaths)
	slices.Sort(wantPaths)
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("unexpected invalid paths\nwant: %#v\ngot:  %#v", wantPaths, gotPaths)
	}
}

func TestValidateTasksCommandAllValid(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Valid One", "type: backend", "complexity: low"},
		"# Task 1: Valid One",
	))
	writeRawTaskFileForCLI(t, tasksDir, "task_02.md", cliTaskMarkdown(
		[]string{"status: blocked", "title: Valid Two", "type: docs", "complexity: medium"},
		"# Task 2: Valid Two",
	))

	stdout, stderr, exitCode := runValidateTasksCommand(t, workspaceRoot, "tasks", "validate", "--tasks-dir", tasksDir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "all tasks valid") {
		t.Fatalf("expected success output, got %q", stdout)
	}
}

func TestValidateTasksCommandMissingDir(t *testing.T) {
	workspaceRoot, _ := makeValidateTasksWorkspace(t, "demo")
	missingDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "missing")

	stdout, stderr, exitCode := runValidateTasksCommand(
		t,
		workspaceRoot,
		"tasks",
		"validate",
		"--tasks-dir",
		missingDir,
	)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout for missing-dir failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "read tasks directory") || !strings.Contains(stderr, missingDir) {
		t.Fatalf("expected clear missing-dir error, got %q", stderr)
	}
}

func TestWriteValidateTasksJSONAndHelpers(t *testing.T) {
	t.Parallel()

	registry, err := tasks.NewRegistry(nil)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	report := tasks.Report{
		TasksDir: "/tmp/tasks",
		Scanned:  1,
		Issues: []tasks.Issue{
			{
				Path:    "/tmp/tasks/task_01.md",
				Field:   "title",
				Message: "title is required",
			},
		},
	}

	var out bytes.Buffer
	if err := writeValidateTasksJSON(&out, report, registry); err != nil {
		t.Fatalf("write validate tasks json: %v", err)
	}

	var payload validateTasksOutput
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode validate tasks json: %v", err)
	}
	if payload.OK {
		t.Fatal("expected invalid payload")
	}
	if payload.FixPrompt == "" {
		t.Fatal("expected fix prompt in json payload")
	}
	if got := validateTasksMessage(tasks.Report{Scanned: 1}); got != "all tasks valid" {
		t.Fatalf("unexpected ok message: %q", got)
	}
	if got := validateTasksMessage(tasks.Report{}); got != "no tasks found" {
		t.Fatalf("unexpected no-tasks message: %q", got)
	}
}

func TestValidateTasksCommandTrimsFormatBeforeSelectingJSONWriter(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "type: backend", "complexity: low"},
		"# Task 1: Missing Title",
	))

	stdout, stderr, exitCode := runValidateTasksCommand(
		t,
		workspaceRoot,
		"tasks",
		"validate",
		"--tasks-dir",
		tasksDir,
		"--format",
		" json ",
	)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	var payload validateTasksOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf(
			"expected trimmed json format to produce json output: %v\nstdout:\n%s\nstderr:\n%s",
			err,
			stdout,
			stderr,
		)
	}
}

func TestValidateTasksCommandResolvesRelativeTasksDirFromWorkspaceRoot(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Valid One", "type: backend", "complexity: low"},
		"# Task 1: Valid One",
	))

	nested := filepath.Join(workspaceRoot, "pkg", "feature")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	stdout, stderr, exitCode := runValidateTasksCommand(
		t,
		nested,
		"tasks",
		"validate",
		"--tasks-dir",
		filepath.Join(".compozy", "tasks", "demo"),
	)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "all tasks valid") {
		t.Fatalf("expected success output, got %q", stdout)
	}
}

func TestResolveTaskWorkflowDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	got, err := resolveTaskWorkflowDir(root, "demo", "")
	if err != nil {
		t.Fatalf("resolve task workflow dir from name: %v", err)
	}
	want := filepath.Join(root, ".compozy", "tasks", "demo")
	if got != want {
		t.Fatalf("unexpected resolved dir\nwant: %q\ngot:  %q", want, got)
	}

	if _, err := resolveTaskWorkflowDir(root, "", ""); err == nil {
		t.Fatal("expected missing-input error")
	}
}

func runValidateTasksCommand(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	return runCLICommand(t, dir, args...)
}

func runCLICommand(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), validateTasksBinary(t), args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run validate-tasks command: %v", err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func validateTasksBinary(t *testing.T) string {
	t.Helper()

	validateTasksBinaryOnce.Do(func() {
		repoRoot, err := validateTasksRepoRoot()
		if err != nil {
			validateTasksBinaryErr = err
			return
		}

		buildDir, err := os.MkdirTemp("", "compozy-validate-tasks-*")
		if err != nil {
			validateTasksBinaryErr = err
			return
		}

		validateTasksBinaryPath = filepath.Join(buildDir, "compozy")
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", validateTasksBinaryPath, "./cmd/compozy")
		cmd.Dir = repoRoot
		cmd.Env = buildCLITestCommandEnv()
		output, err := cmd.CombinedOutput()
		if err != nil {
			validateTasksBinaryErr = fmt.Errorf("build compozy binary: %w\n%s", err, output)
		}
	})

	if validateTasksBinaryErr != nil {
		t.Fatal(validateTasksBinaryErr)
	}
	return validateTasksBinaryPath
}

func buildCLITestCommandEnv() []string {
	env := os.Environ()
	if strings.TrimSpace(originalCLIHome) == "" {
		return env
	}

	prefix := "HOME="
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		filtered = append(filtered, entry)
	}
	filtered = append(filtered, prefix+originalCLIHome)
	return filtered
}

func validateTasksRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(cwd, "..", "..")), nil
}

func makeValidateTasksWorkspace(t *testing.T, name string) (string, string) {
	t.Helper()

	root := t.TempDir()
	tasksDir := filepath.Join(root, ".compozy", "tasks", name)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	return root, tasksDir
}

func cliTaskMarkdown(frontMatter []string, h1 string) string {
	lines := []string{"---"}
	lines = append(lines, frontMatter...)
	lines = append(lines, "---", "", h1, "", "Body.")
	return strings.Join(lines, "\n") + "\n"
}

func writeRawTaskFileForCLI(t *testing.T, tasksDir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(tasksDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func distinctPaths(issues []tasks.Issue) []string {
	seen := make(map[string]struct{}, len(issues))
	paths := make([]string, 0, len(issues))
	for _, issue := range issues {
		if _, ok := seen[issue.Path]; ok {
			continue
		}
		seen[issue.Path] = struct{}{}
		paths = append(paths, issue.Path)
	}
	return paths
}
