package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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
		"type value is unmapped; must be one of:",
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

func TestExecCommandExecuteDirectPromptWritesRunArtifacts(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"Summarize the repository state",
	)
	if err != nil {
		t.Fatalf("execute exec: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	for _, relPath := range []string{
		"run.json",
		filepath.Join("jobs", "exec.prompt.md"),
		filepath.Join("jobs", "exec.out.log"),
		filepath.Join("jobs", "exec.err.log"),
	} {
		if _, statErr := os.Stat(filepath.Join(runDir, relPath)); statErr != nil {
			t.Fatalf("expected exec artifact %s: %v", relPath, statErr)
		}
	}
	if !containsAll(stdout, "Execution Summary:", "- Total Groups: 1", "- Failed: 0") {
		t.Fatalf("unexpected exec stdout:\n%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run exec, got %q", stderr)
	}
}

func TestExecCommandExecutePromptFileJSONEmitsStructuredOutput(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	promptPath := filepath.Join(workspaceRoot, "prompt.md")
	if err := os.WriteFile(promptPath, []byte("Prompt from file\n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"--prompt-file",
		promptPath,
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("execute exec json: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected json exec to suppress stderr, got %q", stderr)
	}

	var payload struct {
		Status     string `json:"status"`
		ResultPath string `json:"result_path"`
		Jobs       []struct {
			Status     string `json:"status"`
			PromptPath string `json:"prompt_path"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode exec stdout json: %v\nstdout:\n%s", err, stdout)
	}
	if payload.Status != "succeeded" {
		t.Fatalf("unexpected run status: %q", payload.Status)
	}
	if len(payload.Jobs) != 1 || payload.Jobs[0].Status != "succeeded" {
		t.Fatalf("unexpected job payload: %#v", payload.Jobs)
	}
	if _, statErr := os.Stat(payload.ResultPath); statErr != nil {
		t.Fatalf("expected result.json to exist: %v", statErr)
	}
	if _, statErr := os.Stat(payload.Jobs[0].PromptPath); statErr != nil {
		t.Fatalf("expected prompt artifact to exist: %v", statErr)
	}

	resultBytes, err := os.ReadFile(payload.ResultPath)
	if err != nil {
		t.Fatalf("read result.json: %v", err)
	}
	if strings.TrimSpace(stdout) != strings.TrimSpace(string(resultBytes)) {
		t.Fatalf("expected stdout JSON to match result.json\nstdout:\n%s\nresult:\n%s", stdout, string(resultBytes))
	}
}

func TestExecCommandExecuteStdinWorksEndToEnd(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		strings.NewReader("Prompt from stdin\n"),
		"exec",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("execute exec stdin: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	promptBytes, err := os.ReadFile(filepath.Join(runDir, "jobs", "exec.prompt.md"))
	if err != nil {
		t.Fatalf("read prompt artifact: %v", err)
	}
	if string(promptBytes) != "Prompt from stdin\n" {
		t.Fatalf("unexpected stdin prompt artifact: %q", string(promptBytes))
	}
	if !strings.Contains(stdout, "Execution Summary:") {
		t.Fatalf("expected text exec summary, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run stdin exec, got %q", stderr)
	}
}

func latestRunDirForCLI(t *testing.T, workspaceRoot string) string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(workspaceRoot, ".compozy", "runs"))
	if err != nil {
		t.Fatalf("read runs dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one run dir, got %d", len(entries))
	}
	return filepath.Join(workspaceRoot, ".compozy", "runs", entries[0].Name())
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()

	cliWorkingDirMu.Lock()

	originalWD, err := os.Getwd()
	if err != nil {
		cliWorkingDirMu.Unlock()
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		defer cliWorkingDirMu.Unlock()
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
	if err := os.Chdir(dir); err != nil {
		cliWorkingDirMu.Unlock()
		t.Fatalf("chdir: %v", err)
	}
}

func executeRootCommandCapturingProcessIO(t *testing.T, in io.Reader, args ...string) (string, string, error) {
	t.Helper()

	cmd := NewRootCommand()
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	os.Stdout = stdoutWrite
	os.Stderr = stderrWrite
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	cmd.SetOut(stdoutWrite)
	cmd.SetErr(stderrWrite)
	if in != nil {
		cmd.SetIn(in)
	}
	cmd.SetArgs(args)

	runErr := cmd.Execute()

	if err := stdoutWrite.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWrite.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	stdoutBytes, err := io.ReadAll(stdoutRead)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrRead)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := stdoutRead.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	if err := stderrRead.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}

	return string(stdoutBytes), string(stderrBytes), runErr
}

func containsAll(s string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(s, fragment) {
			return false
		}
	}
	return true
}
