package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

func TestWriteExecJSONFailureAndReportedErrorHelpers(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteExecJSONFailure(&buf, "exec-123", errors.New("boom")); err != nil {
		t.Fatalf("WriteExecJSONFailure: %v", err)
	}

	var payload execSetupErrorPayload
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &payload); err != nil {
		t.Fatalf("decode exec failure payload: %v", err)
	}
	if payload.Type != "run.failed" || payload.RunID != "exec-123" || payload.Error != "boom" {
		t.Fatalf("unexpected exec failure payload: %#v", payload)
	}

	reported := &execReportedError{err: errors.New("reported")}
	if !IsExecErrorReported(reported) {
		t.Fatal("expected reported exec error to be detected")
	}
	if got := reported.Error(); got != "reported" {
		t.Fatalf("unexpected reported error text: %q", got)
	}
	if reported.Unwrap() == nil {
		t.Fatal("expected unwrap to expose the wrapped error")
	}
	if IsExecErrorReported(errors.New("plain")) {
		t.Fatal("expected plain error not to be reported")
	}
}

func TestExecRunStateCompleteDryRunWritesArtifacts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	runArtifacts := model.NewRunArtifacts(tmpDir, "exec-dry-run")
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	state := &execRunState{
		record:       PersistedExecRun{UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
		runArtifacts: runArtifacts,
		turn:         1,
		turnPaths: execTurnPaths{
			promptPath:   filepath.Join(tmpDir, "prompt.md"),
			responsePath: filepath.Join(tmpDir, "response.txt"),
			resultPath:   filepath.Join(tmpDir, "result.json"),
		},
	}

	if err := state.completeDryRun("summarize the repository"); err != nil {
		t.Fatalf("completeDryRun: %v", err)
	}

	promptBytes, err := os.ReadFile(state.turnPaths.promptPath)
	if err != nil {
		t.Fatalf("read prompt artifact: %v", err)
	}
	if got := string(promptBytes); got != "summarize the repository" {
		t.Fatalf("unexpected prompt artifact: %q", got)
	}

	responseBytes, err := os.ReadFile(state.turnPaths.responsePath)
	if err != nil {
		t.Fatalf("read response artifact: %v", err)
	}
	if got := string(responseBytes); got != "summarize the repository" {
		t.Fatalf("unexpected response artifact: %q", got)
	}

	var turn persistedExecTurn
	resultBytes, err := os.ReadFile(state.turnPaths.resultPath)
	if err != nil {
		t.Fatalf("read result artifact: %v", err)
	}
	if err := json.Unmarshal(resultBytes, &turn); err != nil {
		t.Fatalf("decode turn result: %v", err)
	}
	if !turn.DryRun || turn.Status != runStatusSucceeded {
		t.Fatalf("unexpected dry-run turn result: %#v", turn)
	}
}

func TestFinalizeExecResultWrapsCompletionErrorsAsReported(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	state := &execRunState{
		turnPaths: execTurnPaths{
			responsePath: filepath.Join(tmpDir, "missing", "response.txt"),
		},
	}

	err := finalizeExecResult(state, execExecutionResult{
		status: runStatusFailed,
		err:    errors.New("boom"),
	})
	if err == nil {
		t.Fatal("expected finalizeExecResult to fail")
	}
	if !IsExecErrorReported(err) {
		t.Fatalf("expected finalizeExecResult to return reported error, got %T", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped error to retain original cause, got %v", err)
	}
}

func TestExecRetryHelpersCoverRetryableAndBoundedTimeouts(t *testing.T) {
	t.Parallel()

	if !isExecRetryableError(&activityTimeoutError{timeout: time.Second}) {
		t.Fatal("expected activity timeout to be retryable")
	}
	if !isExecRetryableError(&agent.SessionSetupError{
		Stage: agent.SessionSetupStageNewSession,
		Err:   errors.New("retry"),
	}) {
		t.Fatal("expected session setup errors to be retryable")
	}
	if isExecRetryableError(errors.New("plain")) {
		t.Fatal("expected plain errors not to be retryable")
	}

	if got := nextRetryTimeout(5*time.Second, 2); got != 10*time.Second {
		t.Fatalf("unexpected retry timeout growth: %v", got)
	}
	if got := nextRetryTimeout(40*time.Minute, 2); got != 30*time.Minute {
		t.Fatalf("expected retry timeout cap, got %v", got)
	}
	if !equalStringSlices([]string{"a", "b"}, []string{"a", "b"}) {
		t.Fatal("expected equalStringSlices to match identical slices")
	}
	if equalStringSlices([]string{"a"}, []string{"b"}) {
		t.Fatal("expected equalStringSlices to reject mismatched slices")
	}
}

func TestLoadPersistedExecRunDefaultsPathsAndResumeValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	runArtifacts := model.NewRunArtifacts(tmpDir, "exec-123")
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	record := PersistedExecRun{
		Version:         execRunSchemaVersion,
		Mode:            model.ModeExec,
		RunID:           "exec-123",
		Status:          "running",
		WorkspaceRoot:   tmpDir,
		IDE:             model.IDECodex,
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		AccessMode:      "workspace-write",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		ACPSessionID:    "sess-123",
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal run record: %v", err)
	}
	if err := os.WriteFile(runArtifacts.RunMetaPath, payload, 0o600); err != nil {
		t.Fatalf("write run record: %v", err)
	}

	loaded, err := LoadPersistedExecRun(tmpDir, "exec-123")
	if err != nil {
		t.Fatalf("LoadPersistedExecRun: %v", err)
	}
	if loaded.EventsPath != runArtifacts.EventsPath {
		t.Fatalf("expected default events path %q, got %q", runArtifacts.EventsPath, loaded.EventsPath)
	}
	if loaded.TurnsDir != runArtifacts.TurnsDir {
		t.Fatalf("expected default turns dir %q, got %q", runArtifacts.TurnsDir, loaded.TurnsDir)
	}

	err = validateExecResumeCompatibility(&model.RuntimeConfig{
		RunID:           "exec-123",
		WorkspaceRoot:   tmpDir,
		IDE:             model.IDECodex,
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		AccessMode:      "workspace-write",
	}, loaded)
	if err != nil {
		t.Fatalf("validateExecResumeCompatibility: %v", err)
	}

	err = validateExecResumeCompatibility(&model.RuntimeConfig{
		RunID:         "exec-123",
		WorkspaceRoot: filepath.Join(tmpDir, "other"),
		IDE:           model.IDECodex,
		Model:         "gpt-5.4",
	}, loaded)
	if err == nil || !strings.Contains(err.Error(), "belongs to workspace") {
		t.Fatalf("expected workspace mismatch error, got %v", err)
	}
}

func TestRuntimeEventHelperUtilities(t *testing.T) {
	t.Parallel()

	if got := providerStatusCode(nil); got != 200 {
		t.Fatalf("expected synthetic success status 200, got %d", got)
	}
	if got := providerStatusCode(errors.New("plain")); got != 0 {
		t.Fatalf("expected status 0 for plain error, got %d", got)
	}
	if got := issueIDFromPath("/tmp/reviews/issue_001.md"); got != "issue_001.md" {
		t.Fatalf("unexpected issue id from path: %q", got)
	}
}
