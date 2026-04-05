package cli

import (
	"os"
	"strings"
	"testing"
)

func TestMigrateCommandExecuteDirectReportsUnmappedTypeFollowUp(t *testing.T) {
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

	withWorkingDir(t, workspaceRoot)

	output, err := executeRootCommand("migrate", "--tasks-dir", tasksDir)
	if err != nil {
		t.Fatalf("execute migrate: %v\noutput:\n%s", err, output)
	}
	if !containsAll(output,
		"V1->V2 migrated: 1",
		"Unmapped type files: 1",
		"Fix prompt:",
		"type \"\" must be one of:",
	) {
		t.Fatalf("unexpected migrate output:\n%s", output)
	}
}

func TestValidateTasksCommandExecuteDirectCoversFailureAndSuccess(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Missing Title",
	))

	withWorkingDir(t, workspaceRoot)

	output, err := executeRootCommand("validate-tasks", "--tasks-dir", tasksDir)
	if err == nil {
		t.Fatalf("expected validation failure\noutput:\n%s", output)
	}
	if !containsAll(output, "task validation failed", "Fix prompt:", "title is required") {
		t.Fatalf("unexpected invalid validation output:\n%s", output)
	}

	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Missing Title",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Missing Title",
	))

	output, err = executeRootCommand("validate-tasks", "--tasks-dir", tasksDir)
	if err != nil {
		t.Fatalf("expected validation success: %v\noutput:\n%s", err, output)
	}
	if output != "all tasks valid (1 scanned)\n" {
		t.Fatalf("unexpected validation success output: %q", output)
	}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
}

func containsAll(s string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(s, fragment) {
			return false
		}
	}
	return true
}
