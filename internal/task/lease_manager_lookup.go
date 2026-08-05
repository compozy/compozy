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
	settlement, err := m.failRunLeaseSettlement(ctx, normalized, actor)
	if err != nil {
		return nil, err
	}
	defer m.restoreTaskRunNetworkBestEffort(ctx, settlement.Run.SessionID, settlement.Run.ID)
	m.dispatchTerminalWake(ctx, settlement.Task, settlement.Run, actor)
	m.dispatchTaskRunFailed(ctx, settlement.Run, settlement.Task, actor)
	return &settlement.Run, nil
}

type autoEnqueueTriggeredPayload struct {
	Status      Status    `json:"status"`
	RunStatus   RunStatus `json:"run_status"`
	TriggerKind string    `json:"trigger_kind"`
	TriggerRef  string    `json:"trigger_ref"`
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
