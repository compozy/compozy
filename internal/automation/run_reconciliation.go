package automation

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// ErrRunActive refuses settlement while an execution may still be live.
var ErrRunActive = errors.New(
	"automation: run may still be executing; stop its session or delegated execution before canceling",
)

func (m *Manager) reconcileRuns(ctx context.Context) error {
	for _, status := range []RunStatus{RunScheduled, RunRunning} {
		runs, err := m.store.ListRuns(ctx, RunQuery{
			Status: status, AllWorkspaces: true, ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil {
			return err
		}
		for _, run := range runs {
			live, err := m.runSessionLive(ctx, run)
			if err != nil {
				return err
			}
			if live {
				continue
			}
			if _, err := m.store.SettleRun(ctx, run, RunFailed, "orphaned_after_restart", m.now()); err != nil {
				return fmt.Errorf("reconcile run %q: %w", run.ID, err)
			}
			m.logger.Info("automation.run.reconciled", "run_id", run.ID, "session_id", run.SessionID,
				"reason", "orphaned_after_restart")
		}
	}
	return nil
}

// CancelRun settles an orphan without interrupting a live execution.
func (m *Manager) CancelRun(ctx context.Context, id string) (Run, error) {
	if ctx == nil {
		return Run{}, errors.New("automation: cancel run context is required")
	}
	run, err := m.GetRun(ctx, id)
	if err != nil {
		return Run{}, err
	}
	if run.Status == RunDelegated {
		return Run{}, ErrRunActive
	}
	if run.Status != RunScheduled && run.Status != RunRunning {
		return run, nil
	}
	// An admission without a session can still be creating its execution.
	if run.SessionID == "" {
		return Run{}, ErrRunActive
	}
	live, err := m.runSessionLive(ctx, run)
	if err != nil {
		return Run{}, err
	}
	if live {
		return Run{}, ErrRunActive
	}
	return m.store.SettleRun(ctx, run, RunCancelled, "operator_canceled", m.now())
}

func (m *Manager) runSessionLive(ctx context.Context, run Run) (bool, error) {
	if run.TaskID != "" || run.TaskRunID != "" || run.LoopRunID != "" {
		return true, nil
	}
	if run.SessionID == "" {
		return false, nil
	}
	info, err := m.sessions.Status(ctx, run.SessionID)
	if errors.Is(err, session.ErrSessionNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read session %q for run %q: %w", run.SessionID, run.ID, err)
	}
	if info == nil {
		return false, errors.New("automation: session lookup returned no status")
	}
	return info.State != session.StateStopped, nil
}
