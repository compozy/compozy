package cli

import (
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

	stdout, stderr, exitCode = runCLICommand(t, workspaceRoot, "validate-tasks", "--tasks-dir", tasksDir)
	if exitCode != 1 {
		t.Fatalf("expected validate-tasks exit code 1, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "Fix prompt:") {
		t.Fatalf("expected validate-tasks output to include fix prompt, got:\n%s", stdout)
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

	stdout, stderr, exitCode = runCLICommand(t, workspaceRoot, "validate-tasks", "--tasks-dir", tasksDir)
	if exitCode != 0 {
		t.Fatalf(
			"expected validate-tasks exit code 0 after fix, got %d\nstdout:\n%s\nstderr:\n%s",
			exitCode,
			stdout,
			stderr,
		)
	}
	if !strings.Contains(stdout, "all tasks valid") {
		t.Fatalf("expected validate-tasks success output, got:\n%s", stdout)
	}
}

func TestValidateTasksCommandPassesCommittedACPFixtures(t *testing.T) {
	repoRoot, err := validateTasksRepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	tasksDir := filepath.Join(repoRoot, ".compozy", "tasks", "acp-integration")
	stdout, stderr, exitCode := runCLICommand(t, repoRoot, "validate-tasks", "--tasks-dir", tasksDir)
	if exitCode != 0 {
		t.Fatalf("expected acp fixture validation to pass, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "all tasks valid") {
		t.Fatalf("expected success output, got:\n%s", stdout)
	}
}
