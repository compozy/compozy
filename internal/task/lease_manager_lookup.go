package task

import (
	"context"
	"errors"
	"fmt"

	"strings"
)

// FailRunLease marks one active task-run lease failed after token verification.
func (m *Service) FailRunLease(
	ctx context.Context,
	failure LeaseFailure,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := failure.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	normalized.Actor = actor
	run, err := m.store.FailRunLease(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if run.IsNetworkWake() {
		m.dispatchTaskRunFailed(ctx, run, Task{}, actor)
		return &run, nil
	}
	reconciledTask, err := m.reconcileTaskCascade(ctx, run.TaskID, actor)
	if err != nil {
		return nil, err
	}
	m.dispatchTerminalWake(ctx, reconciledTask, run, actor)
	m.dispatchTaskRunFailed(ctx, run, reconciledTask, actor)
	return &run, nil
}

type autoEnqueueTriggeredPayload struct {
	Status      Status    `json:"status"`
	RunStatus   RunStatus `json:"run_status"`
	TriggerKind string    `json:"trigger_kind"`
	TriggerRef  string    `json:"trigger_ref"`
}

func (m *Service) activeSessionRunLeases(ctx context.Context, sessionID string) ([]Run, error) {
	statuses := []RunStatus{TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning}
	runs := make([]Run, 0, len(statuses))
	for _, status := range statuses {
		matches, err := m.store.ListTaskRuns(ctx, RunQuery{
			SessionID: sessionID,
			Status:    status,
		})
		if err != nil {
			return nil, err
		}
		for _, run := range matches {
			if strings.TrimSpace(run.SessionID) != strings.TrimSpace(sessionID) ||
				strings.TrimSpace(run.ClaimTokenHash) == "" {
				continue
			}
			runs = append(runs, run)
		}
	}
	return runs, nil
}

// LookupActiveRunForSession resolves the internal claim token for a session-owned
// run while preserving the existing token-fenced lease writers as the sole
// mutation authority.
func (m *Service) LookupActiveRunForSession(
	ctx context.Context,
	sessionID string,
	runID string,
) (AutonomyLeaseHandle, error) {
	normalizedSessionID, normalizedRunID, err := normalizeAutonomyLookupInput(sessionID, runID)
	if err != nil {
		return AutonomyLeaseHandle{}, err
	}
	store, ok := m.store.(AutonomyLeaseStore)
	if !ok {
		return AutonomyLeaseHandle{}, errors.New("task: autonomy lease lookup store is unavailable")
	}
	handles, err := store.ListAutonomyLeaseHandles(ctx, normalizedSessionID)
	if err != nil {
		return AutonomyLeaseHandle{}, err
	}
	return resolveAutonomyLeaseHandle(normalizedSessionID, normalizedRunID, handles, m.now().UTC())
}

func normalizeAutonomyLookupInput(sessionID string, runID string) (string, string, error) {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID == "" {
		return "", "", autonomyError(
			AutonomySessionRequired,
			ErrPermissionDenied,
			"agent session identity is required",
		)
	}
	normalizedRunID := strings.TrimSpace(runID)
	if normalizedRunID == "" {
		return "", "", fmt.Errorf("%w: run_id is required", ErrValidation)
	}
	return normalizedSessionID, normalizedRunID, nil
}
