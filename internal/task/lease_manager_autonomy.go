package task

import (
	"strings"
	"time"
)

func resolveAutonomyLeaseHandle(
	sessionID string,
	runID string,
	handles []AutonomyLeaseHandle,
	now time.Time,
) (AutonomyLeaseHandle, error) {
	target, hasTarget, activeHandles := autonomyLeaseCandidates(handles, sessionID, runID, now)
	if len(activeHandles) > 1 {
		return AutonomyLeaseHandle{}, autonomyError(
			AutonomyLeaseAlreadyHeld,
			ErrActiveRunLease,
			"session %q owns multiple active task-run leases",
			sessionID,
		)
	}
	if !hasTarget {
		return missingAutonomyLeaseError(sessionID, runID, activeHandles)
	}
	if !isActiveAutonomyLeaseHandle(target, sessionID, now) {
		return AutonomyLeaseHandle{}, autonomyError(
			AutonomyLeaseExpired,
			ErrLeaseExpired,
			"run %q is not an active lease for session %q",
			runID,
			sessionID,
		)
	}
	if len(activeHandles) == 1 && activeHandles[0].RunID != runID {
		return AutonomyLeaseHandle{}, autonomyError(
			AutonomyForeignRun,
			ErrPermissionDenied,
			"run %q is not owned by session %q",
			runID,
			sessionID,
		)
	}
	return target, nil
}

func autonomyLeaseCandidates(
	handles []AutonomyLeaseHandle,
	sessionID string,
	runID string,
	now time.Time,
) (AutonomyLeaseHandle, bool, []AutonomyLeaseHandle) {
	var target AutonomyLeaseHandle
	hasTarget := false
	activeHandles := make([]AutonomyLeaseHandle, 0, len(handles))
	for idx := range handles {
		handle := normalizeAutonomyLeaseHandle(handles[idx])
		if handle.RunID == runID {
			target = handle
			hasTarget = true
		}
		if isActiveAutonomyLeaseHandle(handle, sessionID, now) {
			activeHandles = append(activeHandles, handle)
		}
	}
	return target, hasTarget, activeHandles
}

func normalizeAutonomyLeaseHandle(handle AutonomyLeaseHandle) AutonomyLeaseHandle {
	handle.SessionID = strings.TrimSpace(handle.SessionID)
	handle.RunID = strings.TrimSpace(handle.RunID)
	handle.TaskID = strings.TrimSpace(handle.TaskID)
	handle.RunKind = handle.RunKind.Normalize()
	handle.WorkspaceID = strings.TrimSpace(handle.WorkspaceID)
	handle.TargetSessionID = strings.TrimSpace(handle.TargetSessionID)
	handle.OwnerKey = strings.TrimSpace(handle.OwnerKey)
	handle.ClaimToken = strings.TrimSpace(handle.ClaimToken)
	handle.ClaimTokenHash = strings.TrimSpace(handle.ClaimTokenHash)
	return handle
}

func isActiveAutonomyLeaseHandle(handle AutonomyLeaseHandle, sessionID string, now time.Time) bool {
	return handle.SessionID == sessionID &&
		isAutonomyLeaseStatusActive(handle.Status) &&
		!handle.LeaseUntil.IsZero() &&
		handle.LeaseUntil.After(now) &&
		handle.ClaimToken != "" &&
		handle.ClaimTokenHash != ""
}

func missingAutonomyLeaseError(
	sessionID string,
	runID string,
	activeHandles []AutonomyLeaseHandle,
) (AutonomyLeaseHandle, error) {
	if len(activeHandles) == 1 {
		return AutonomyLeaseHandle{}, autonomyError(
			AutonomyForeignRun,
			ErrPermissionDenied,
			"run %q is not owned by session %q",
			runID,
			sessionID,
		)
	}
	return AutonomyLeaseHandle{}, autonomyError(
		AutonomyNoActiveLease,
		ErrInvalidClaimToken,
		"session %q has no active task-run lease",
		sessionID,
	)
}

func requeueSessionRunLease(run Run) Run {
	run.Status = TaskRunStatusQueued
	run.ClaimedBy = nil
	run.ClaimedAt = time.Time{}
	run.SessionID = ""
	run.ClaimTokenHash = ""
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.StartedAt = time.Time{}
	run.EndedAt = time.Time{}
	run.Error = ""
	run.Result = nil
	return run
}
