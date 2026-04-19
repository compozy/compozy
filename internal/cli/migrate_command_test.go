package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCommandPrintsUnmappedTypeSummaryAndValidateFailsUntilFixed(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"domain: backend",
			"type: Feature Implementation",
			"scope: full",
			"complexity: low",
		},
		"# Task 1: Needs Classification",
	))

	stdout, stderr, exitCode := runCLICommand(t, workspaceRoot, "migrate", "--tasks-dir", tasksDir)
	if exitCode != 0 {
		t.Fatalf("expected migrate exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "V1->V2 migrated: 1") {
		t.Fatalf("expected migrate summary to include v1->v2 counter, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Unmapped type files: 1") {
		t.Fatalf("expected migrate summary to include unmapped count, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Fix prompt:") {
		t.Fatalf("expected migrate output to include fix prompt, got:\n%s", stdout)
	}

	stdout, stderr, exitCode = runCLICommand(t, workspaceRoot, "tasks", "validate", "--tasks-dir", tasksDir)
	if exitCode != 1 {
		t.Fatalf("expected tasks validate exit code 1, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "Fix prompt:") {
		t.Fatalf("expected tasks validate output to include fix prompt, got:\n%s", stdout)
	}

	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Needs Classification",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Needs Classification",
	))

	stdout, stderr, exitCode = runCLICommand(t, workspaceRoot, "tasks", "validate", "--tasks-dir", tasksDir)
	if exitCode != 0 {
		t.Fatalf(
			"expected tasks validate exit code 0 after fix, got %d\nstdout:\n%s\nstderr:\n%s",
			exitCode,
			stdout,
			stderr,
		)
	}
	if !strings.Contains(stdout, "all tasks valid") {
		t.Fatalf("expected tasks validate success output, got:\n%s", stdout)
	}
}

func TestValidateTasksCommandPassesCommittedACPFixtures(t *testing.T) {
	repoRoot, err := validateTasksRepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	tasksDir, err := committedACPFixtureDir(repoRoot)
	if err != nil {
		t.Fatalf("resolve committed acp fixture dir: %v", err)
	}
	stdout, stderr, exitCode := runCLICommand(t, repoRoot, "tasks", "validate", "--tasks-dir", tasksDir)
	if exitCode != 0 {
		t.Fatalf("expected acp fixture validation to pass, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "all tasks valid") {
		t.Fatalf("expected success output, got:\n%s", stdout)
	}
}

func committedACPFixtureDir(repoRoot string) (string, error) {
	activePath := filepath.Join(repoRoot, ".compozy", "tasks", "acp-integration")
	if info, err := os.Stat(activePath); err == nil && info.IsDir() {
		return activePath, nil
	}

	matches, err := filepath.Glob(filepath.Join(repoRoot, ".compozy", "tasks", "_archived", "*-acp-integration"))
	if err != nil {
		return "", err
	}
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil && info.IsDir() {
			return match, nil
		}
	}
	return "", os.ErrNotExist
}
