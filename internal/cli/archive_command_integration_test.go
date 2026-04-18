package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestArchiveCommandArchivesSyncedWorkflowIntoNewPathFormat(t *testing.T) {
	homeDir := newShortCLITestHomeDir(t)
	t.Setenv("HOME", homeDir)

	workspaceRoot := t.TempDir()
	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "task_001.md"), []byte(strings.Join([]string{
		"---",
		"status: completed",
		"title: Demo",
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# Demo",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	stdout, stderr, exitCode := runCLICommand(t, workspaceRoot, "sync", "--name", "demo")
	if exitCode != 0 {
		t.Fatalf("execute sync: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	output, stderr, exitCode := runCLICommand(t, workspaceRoot, "archive", "--name", "demo")
	if exitCode != 0 {
		t.Fatalf("execute archive: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, output, stderr)
	}
	if !strings.Contains(output, "Archived: 1") {
		t.Fatalf("archive output missing archived count:\n%s", output)
	}

	matches, err := filepath.Glob(filepath.Join(workspaceRoot, ".compozy", "tasks", "_archived", "*-demo"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("archived matches = %#v, want one archived workflow", matches)
	}
	if matched, err := regexp.MatchString(
		`^\d{13}-[a-z0-9]{8}-demo$`,
		filepath.Base(matches[0]),
	); err != nil ||
		!matched {
		t.Fatalf("unexpected archived workflow path: %s", matches[0])
	}
}
