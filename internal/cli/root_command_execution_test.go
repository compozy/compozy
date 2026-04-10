package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/setup"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/spf13/cobra"
)

var cliProcessIOMu sync.Mutex

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

func TestExecCommandExecuteDirectPromptIsEphemeralByDefault(t *testing.T) {
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

	if got := strings.TrimSpace(stdout); got != "Summarize the repository state" {
		t.Fatalf("unexpected exec stdout: %q", got)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run exec, got %q", stderr)
	}
	assertNoRunArtifactsForCLI(t, workspaceRoot)
}

func TestExecCommandExecutePromptFileJSONEmitsJSONLByDefault(t *testing.T) {
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

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) != 2 {
		t.Fatalf("expected two jsonl events, got %d\nstdout:\n%s", len(events), stdout)
	}
	if events[0]["type"] != "run.started" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1]["type"] != "run.succeeded" {
		t.Fatalf("unexpected second event: %#v", events[1])
	}
	if output, ok := events[1]["output"].(string); !ok || output != "Prompt from file\n" {
		t.Fatalf("unexpected final output payload: %#v", events[1])
	}
	assertNoRunArtifactsForCLI(t, workspaceRoot)
}

func TestExecCommandExecutePersistCreatesTurnArtifacts(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"--persist",
		"Persist this prompt",
	)
	if err != nil {
		t.Fatalf("execute persisted exec: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "Persist this prompt" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run persisted exec, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	for _, relPath := range []string{
		"run.json",
		"events.jsonl",
		filepath.Join("turns", "0001", "prompt.md"),
		filepath.Join("turns", "0001", "response.txt"),
		filepath.Join("turns", "0001", "result.json"),
	} {
		if _, statErr := os.Stat(filepath.Join(runDir, relPath)); statErr != nil {
			t.Fatalf("expected persisted exec artifact %s: %v", relPath, statErr)
		}
	}
}

func TestExecCommandExecuteRunIDUsesPersistedRuntimeDefaults(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	runID := "exec-resume"
	writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
		Version:         1,
		Mode:            model.ModeExec,
		RunID:           runID,
		Status:          "succeeded",
		WorkspaceRoot:   workspaceRoot,
		IDE:             model.IDECodex,
		Model:           "gpt-5-codex",
		ReasoningEffort: "high",
		AccessMode:      model.AccessModeDefault,
		AddDirs:         []string{filepath.Join(workspaceRoot, "docs")},
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		TurnCount:       1,
		ACPSessionID:    "sess-existing",
	})

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"--run-id",
		runID,
		"Resume this conversation",
	)
	if err != nil {
		t.Fatalf("execute resumed exec dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "Resume this conversation" {
		t.Fatalf("unexpected resumed dry-run stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for resumed dry-run exec, got %q", stderr)
	}
}

func TestExecCommandExecuteJSONMissingPromptEmitsFailureJSON(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(t, nil, "exec", "--format", "json")
	if err == nil {
		t.Fatalf("expected exec json missing-prompt failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected json exec failure to suppress stderr, got %q", stderr)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) != 1 {
		t.Fatalf("expected one json failure event, got %d\nstdout:\n%s", len(events), stdout)
	}
	if events[0]["type"] != "run.failed" {
		t.Fatalf("unexpected failure event: %#v", events[0])
	}
	errorMessage, _ := events[0]["error"].(string)
	if !strings.Contains(errorMessage, "requires exactly one prompt source") {
		t.Fatalf("unexpected json error message: %#v", events[0])
	}
}

func TestExecCommandExecuteRawJSONMissingPromptEmitsFailureJSON(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(t, nil, "exec", "--format", "raw-json")
	if err == nil {
		t.Fatalf("expected exec raw-json missing-prompt failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected raw-json exec failure to suppress stderr, got %q", stderr)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) != 1 {
		t.Fatalf("expected one raw-json failure event, got %d\nstdout:\n%s", len(events), stdout)
	}
	if events[0]["type"] != "run.failed" {
		t.Fatalf("unexpected failure event: %#v", events[0])
	}
	errorMessage, _ := events[0]["error"].(string)
	if !strings.Contains(errorMessage, "requires exactly one prompt source") {
		t.Fatalf("unexpected raw-json error message: %#v", events[0])
	}
}

func TestExecCommandExecuteJSONValidationFailureEmitsFailureJSON(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--format",
		"json",
		"--tui",
		"Prompt for validation failure",
	)
	if err == nil {
		t.Fatalf("expected exec json validation failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected json exec validation failure to suppress stderr, got %q", stderr)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) != 1 {
		t.Fatalf("expected one json validation failure event, got %d\nstdout:\n%s", len(events), stdout)
	}
	if events[0]["type"] != "run.failed" {
		t.Fatalf("unexpected validation failure event: %#v", events[0])
	}
	errorMessage, _ := events[0]["error"].(string)
	if !strings.Contains(errorMessage, "tui mode is not supported with json or raw-json output") {
		t.Fatalf("unexpected validation error message: %#v", events[0])
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

	if got := strings.TrimSpace(stdout); got != "Prompt from stdin" {
		t.Fatalf("unexpected stdin stdout: %q", got)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run stdin exec, got %q", stderr)
	}
	assertNoRunArtifactsForCLI(t, workspaceRoot)
}

func TestStartCommandExecuteDryRunPersistsKernelArtifacts(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	withWorkingDir(t, workspaceRoot)

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"start",
		"--name",
		"demo",
		"--tasks-dir",
		".compozy/tasks/demo",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("execute start dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	runMeta := readCLIArtifactJSON(t, filepath.Join(runDir, "run.json"))
	if got := runMeta["mode"]; got != string(model.ModePRDTasks) {
		t.Fatalf("unexpected run mode: %#v", runMeta)
	}

	result := readCLIArtifactJSON(t, filepath.Join(runDir, "result.json"))
	if got := result["status"]; got != "succeeded" {
		t.Fatalf("unexpected result payload: %#v", result)
	}

	promptPath := singleCLIJobArtifact(t, runDir, "*.prompt.md")
	outLogPath := singleCLIJobArtifact(t, runDir, "*.out.log")
	errLogPath := singleCLIJobArtifact(t, runDir, "*.err.log")
	for _, path := range []string{promptPath, outLogPath, errLogPath} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("expected job artifact %s: %v", path, statErr)
		}
	}

	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt artifact: %v", err)
	}
	if !strings.Contains(string(promptBytes), "`cy-execute-task`") {
		t.Fatalf("expected task prompt to reference cy-execute-task, got:\n%s", string(promptBytes))
	}

	eventKinds := cliRuntimeEventKinds(t, filepath.Join(runDir, "events.jsonl"))
	for _, want := range []eventspkg.EventKind{
		eventspkg.EventKindRunStarted,
		eventspkg.EventKindJobCompleted,
		eventspkg.EventKindRunCompleted,
	} {
		if !slices.Contains(eventKinds, want) {
			t.Fatalf("expected runtime events to include %s, got %v", want, eventKinds)
		}
	}
}

func TestFixReviewsCommandExecuteDryRunPersistsKernelArtifacts(t *testing.T) {
	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo", "reviews-001")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{{
		Title:       "Add nil check",
		File:        "internal/app/service.go",
		Line:        42,
		Author:      "coderabbitai[bot]",
		ProviderRef: "thread:PRT_1,comment:RC_1",
		Body:        "Please add a nil check before dereferencing the pointer.",
	}}); err != nil {
		t.Fatalf("write review round: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"fix-reviews",
		"--name",
		"demo",
		"--round",
		"1",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("execute fix-reviews dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run fix-reviews, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	runMeta := readCLIArtifactJSON(t, filepath.Join(runDir, "run.json"))
	if got := runMeta["mode"]; got != string(model.ModeCodeReview) {
		t.Fatalf("unexpected review run mode: %#v", runMeta)
	}

	result := readCLIArtifactJSON(t, filepath.Join(runDir, "result.json"))
	if got := result["status"]; got != "succeeded" {
		t.Fatalf("unexpected review result payload: %#v", result)
	}

	promptPath := singleCLIJobArtifact(t, runDir, "*.prompt.md")
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read review prompt artifact: %v", err)
	}
	for _, want := range []string{"`cy-fix-reviews`", "issue_001.md", "internal/app/service.go"} {
		if !strings.Contains(string(promptBytes), want) {
			t.Fatalf("expected review prompt to contain %q, got:\n%s", want, string(promptBytes))
		}
	}

	eventKinds := cliRuntimeEventKinds(t, filepath.Join(runDir, "events.jsonl"))
	for _, want := range []eventspkg.EventKind{
		eventspkg.EventKindRunStarted,
		eventspkg.EventKindJobCompleted,
		eventspkg.EventKindRunCompleted,
	} {
		if !slices.Contains(eventKinds, want) {
			t.Fatalf("expected runtime events to include %s, got %v", want, eventKinds)
		}
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

func assertNoRunArtifactsForCLI(t *testing.T, workspaceRoot string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(workspaceRoot, ".compozy", "runs")); !os.IsNotExist(err) {
		t.Fatalf("expected no persisted exec artifacts by default, got stat err=%v", err)
	}
}

func writePersistedExecRunForCLI(t *testing.T, workspaceRoot string, record coreRun.PersistedExecRun) {
	t.Helper()

	runArtifacts := model.NewRunArtifacts(workspaceRoot, record.RunID)
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir persisted exec run dir: %v", err)
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal persisted exec run: %v", err)
	}
	if err := os.WriteFile(runArtifacts.RunMetaPath, payload, 0o600); err != nil {
		t.Fatalf("write persisted exec run: %v", err)
	}
}

func decodeExecJSONLEvents(t *testing.T, stdout string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode exec jsonl line: %v\nline:\n%s", err, line)
		}
		events = append(events, payload)
	}
	return events
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

	return executeCommandCapturingProcessIO(t, NewRootCommand(), in, args...)
}

func executeCommandCapturingProcessIO(
	t *testing.T,
	cmd *cobra.Command,
	in io.Reader,
	args ...string,
) (string, string, error) {
	t.Helper()

	cliProcessIOMu.Lock()
	defer cliProcessIOMu.Unlock()

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

func allowBundledSkillsForExecutionTests() commandStateDefaults {
	defaults := defaultCommandStateDefaults()
	defaults.listBundledSkills = func() ([]setup.Skill, error) {
		return nil, nil
	}
	defaults.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{}, nil
	}
	return defaults
}

func readCLIArtifactJSON(t *testing.T, path string) map[string]any {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact %s: %v", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("decode artifact %s: %v", path, err)
	}
	return payload
}

func singleCLIJobArtifact(t *testing.T, runDir string, pattern string) string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(runDir, "jobs", pattern))
	if err != nil {
		t.Fatalf("glob job artifact %s: %v", pattern, err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one %s artifact, got %d (%v)", pattern, len(matches), matches)
	}
	return matches[0]
}

func cliRuntimeEventKinds(t *testing.T, eventsPath string) []eventspkg.EventKind {
	t.Helper()

	content, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events artifact %s: %v", eventsPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	kinds := make([]eventspkg.EventKind, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event eventspkg.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode runtime event line: %v\nline:\n%s", err, line)
		}
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func containsAll(s string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(s, fragment) {
			return false
		}
	}
	return true
}
