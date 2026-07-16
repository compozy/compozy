package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/recovery"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestIsTerminalRunStatusParked(t *testing.T) {
	t.Parallel()

	t.Run("Should treat parked as terminal", func(t *testing.T) {
		t.Parallel()
		if !isTerminalRunStatus(runStatusParked) {
			t.Fatalf("isTerminalRunStatus(%q) = false, want true", runStatusParked)
		}
	})

	t.Run("Should treat parked as terminal regardless of case or padding", func(t *testing.T) {
		t.Parallel()
		for _, status := range []string{"parked", "PARKED", "  parked  "} {
			if !isTerminalRunStatus(status) {
				t.Fatalf("isTerminalRunStatus(%q) = false, want true", status)
			}
		}
	})

	t.Run("Should keep non-terminal statuses non-terminal", func(t *testing.T) {
		t.Parallel()
		for _, status := range []string{runStatusStarting, runStatusRunning} {
			if isTerminalRunStatus(status) {
				t.Fatalf("isTerminalRunStatus(%q) = true, want false", status)
			}
		}
	})
}

func TestRecoveryStatusForRunStatusParked(t *testing.T) {
	t.Parallel()

	t.Run("Should map daemon parked to recovery parked", func(t *testing.T) {
		t.Parallel()
		got := recoveryStatusForRunStatus(runStatusParked)
		if got != recovery.StatusParked {
			t.Fatalf("recoveryStatusForRunStatus(%q) = %q, want %q", runStatusParked, got, recovery.StatusParked)
		}
	})

	t.Run("Should keep the parked vocabulary string identical across both models", func(t *testing.T) {
		t.Parallel()
		if runStatusParked != string(recovery.StatusParked) {
			t.Fatalf("daemon parked %q != recovery parked %q", runStatusParked, recovery.StatusParked)
		}
	})

	t.Run("Should not collapse parked into failed or canceled", func(t *testing.T) {
		t.Parallel()
		got := recoveryStatusForRunStatus(runStatusParked)
		if got == recovery.StatusFailed || got == recovery.StatusCanceled {
			t.Fatalf("recoveryStatusForRunStatus(%q) = %q, want distinct from failed/canceled", runStatusParked, got)
		}
	})
}

// TestWaitForTaskMultiChildObservesParkedAsTerminal exercises the parent await
// loop against a real store double: a child seeded in the parked status must be
// returned as terminal instead of being polled indefinitely.
func TestWaitForTaskMultiChildObservesParkedAsTerminal(t *testing.T) {
	paths := mustHomePaths(t)
	t.Setenv("HOME", filepath.Dir(paths.HomeDir))
	db := openDaemonGlobalDB(t, paths)
	workspace := registerDaemonWorkspace(t, db)

	const childRunID = "parked-child-run"
	if _, err := db.PutRun(context.Background(), globaldb.Run{
		RunID:            childRunID,
		WorkspaceID:      workspace.ID,
		ParentRunID:      "parent-run",
		Mode:             runModeTask,
		Status:           runStatusParked,
		PresentationMode: "stream",
		StartedAt:        time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutRun(%q) error = %v", childRunID, err)
	}

	manager := &RunManager{globalDB: db}

	// A generous deadline: if the loop treats parked as non-terminal it will poll
	// until this context expires, which the assertions below catch as a failure.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	row, err := manager.waitForTaskMultiChild(ctx, childRunID, model.StallPolicy{})
	if err != nil {
		t.Fatalf("waitForTaskMultiChild(%q) error = %v", childRunID, err)
	}
	if row.Status != runStatusParked {
		t.Fatalf("waitForTaskMultiChild status = %q, want %q", row.Status, runStatusParked)
	}
	if elapsed := time.Since(start); elapsed >= taskMultiChildPollInterval {
		t.Fatalf("waitForTaskMultiChild took %s, want an immediate terminal return", elapsed)
	}
}
