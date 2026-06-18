package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestReadExecRunOutcome(t *testing.T) {
	t.Parallel()

	t.Run("Should read failed exec turn outcome and preserve execution error", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		artifacts := model.NewRunArtifacts(workspaceRoot, "exec-failed")
		writeExecOutcomeFixture(t, workspaceRoot, artifacts, StatusFailed, 1, "turn failed")
		originalErr := errors.New("exec failed")

		outcome, err := ReadExecRunOutcome(
			context.Background(),
			&model.RuntimeConfig{WorkspaceRoot: workspaceRoot},
			artifacts,
			originalErr,
		)
		if !errors.Is(err, originalErr) {
			t.Fatalf("ReadExecRunOutcome() error = %v, want original error", err)
		}
		if outcome.Status != StatusFailed || len(outcome.Jobs) != 1 {
			t.Fatalf("unexpected outcome: %#v", outcome)
		}
		job := outcome.Jobs[0]
		if job.SafeName != execRecoveryJobID || job.Status != StatusFailed || job.ExitCode != 1 ||
			job.Error != "turn failed" {
			t.Fatalf("unexpected job outcome: %#v", job)
		}
	})

	t.Run("Should mark canceled when context or execution error is canceled", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		artifacts := model.NewRunArtifacts(workspaceRoot, "exec-canceled")
		writeExecOutcomeFixture(t, workspaceRoot, artifacts, StatusFailed, 1, "still running")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		outcome, err := ReadExecRunOutcome(
			ctx,
			&model.RuntimeConfig{WorkspaceRoot: workspaceRoot},
			artifacts,
			context.Canceled,
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadExecRunOutcome() error = %v, want context.Canceled", err)
		}
		if outcome.Status != StatusCanceled || outcome.Jobs[0].ExitCode != -1 {
			t.Fatalf("unexpected canceled outcome: %#v", outcome)
		}
	})

	t.Run("Should return execution error when persisted record is missing", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		artifacts := model.NewRunArtifacts(workspaceRoot, "exec-missing")
		originalErr := errors.New("setup failed")

		_, err := ReadExecRunOutcome(
			context.Background(),
			&model.RuntimeConfig{WorkspaceRoot: workspaceRoot},
			artifacts,
			originalErr,
		)
		if !errors.Is(err, originalErr) {
			t.Fatalf("ReadExecRunOutcome() error = %v, want original setup error", err)
		}
	})

	t.Run("Should map unknown persisted status without execution error", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		artifacts := model.NewRunArtifacts(workspaceRoot, "exec-unknown")
		writeExecOutcomeFixture(t, workspaceRoot, artifacts, RunStatus("pending"), 0, "")

		outcome, err := ReadExecRunOutcome(
			context.Background(),
			&model.RuntimeConfig{WorkspaceRoot: workspaceRoot},
			artifacts,
			nil,
		)
		if err != nil {
			t.Fatalf("ReadExecRunOutcome() error = %v", err)
		}
		if outcome.Status != StatusUnknown || outcome.Jobs[0].Status != StatusUnknown {
			t.Fatalf("unexpected unknown outcome: %#v", outcome)
		}
	})

	t.Run("Should map succeeded exec without turn result", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		artifacts := model.NewRunArtifacts(workspaceRoot, "exec-succeeded")
		writeExecOutcomeFixture(t, workspaceRoot, artifacts, StatusSucceeded, 0, "")

		outcome, err := ReadExecRunOutcome(
			context.Background(),
			&model.RuntimeConfig{WorkspaceRoot: workspaceRoot},
			artifacts,
			nil,
		)
		if err != nil {
			t.Fatalf("ReadExecRunOutcome() error = %v", err)
		}
		if outcome.Status != StatusSucceeded || outcome.Jobs[0].ExitCode != 0 {
			t.Fatalf("unexpected succeeded outcome: %#v", outcome)
		}
	})
}

func writeExecOutcomeFixture(
	t *testing.T,
	workspaceRoot string,
	artifacts model.RunArtifacts,
	status RunStatus,
	turnCount int,
	errText string,
) {
	t.Helper()
	record := struct {
		Version       int       `json:"version"`
		Mode          string    `json:"mode"`
		RunID         string    `json:"run_id"`
		Status        string    `json:"status"`
		WorkspaceRoot string    `json:"workspace_root"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
		TurnCount     int       `json:"turn_count"`
		LastError     string    `json:"last_error,omitempty"`
		EventsPath    string    `json:"events_path,omitempty"`
		TurnsDir      string    `json:"turns_dir,omitempty"`
	}{
		Version:       1,
		Mode:          model.ModeExec,
		RunID:         artifacts.RunID,
		Status:        string(status),
		WorkspaceRoot: workspaceRoot,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		TurnCount:     turnCount,
		LastError:     errText,
		EventsPath:    artifacts.EventsPath,
		TurnsDir:      artifacts.TurnsDir,
	}
	writeRecoveryJSON(t, artifacts.RunMetaPath, record)
	if turnCount <= 0 {
		return
	}
	turnPath := filepath.Join(artifacts.TurnsDir, "0001", "result.json")
	turn := persistedExecTurnOutcome{
		Status:     status,
		ResultPath: turnPath,
		Error:      errText,
	}
	writeRecoveryJSON(t, turnPath, turn)
}

func writeRecoveryJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
