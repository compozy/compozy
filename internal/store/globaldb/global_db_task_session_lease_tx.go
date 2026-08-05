package globaldb

import (
	"context"
	"fmt"
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *taskLeaseSettlementTxStore) ListActiveSessionRunLeases(
	ctx context.Context,
	sessionID string,
) ([]taskpkg.Run, error) {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if err := taskpkg.ValidateReferenceSize(normalizedSessionID, "session_lease_release.session_id"); err != nil {
		return nil, err
	}

	statuses := []taskpkg.RunStatus{
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning,
	}
	runs := make([]taskpkg.Run, 0, len(statuses))
	for _, status := range statuses {
		matches, err := s.tasks.listTaskRunsWithExecutor(ctx, s.exec, taskpkg.RunQuery{
			SessionID: normalizedSessionID,
			Status:    status,
		})
		if err != nil {
			return nil, err
		}
		for _, run := range matches {
			if strings.TrimSpace(run.SessionID) != normalizedSessionID || strings.TrimSpace(run.ClaimTokenHash) == "" {
				continue
			}
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (s *taskLeaseSettlementTxStore) ReleaseSessionRunLease(
	ctx context.Context,
	previous taskpkg.Run,
	release taskpkg.SessionLeaseRelease,
	actor taskpkg.ActorContext,
) (taskpkg.Run, error) {
	if err := actor.Validate(); err != nil {
		return taskpkg.Run{}, err
	}
	if err := release.Validate("session_lease_release"); err != nil {
		return taskpkg.Run{}, err
	}

	snapshot := taskRunLeaseSnapshot{
		status:         previous.Status,
		sessionID:      previous.SessionID,
		leaseUntil:     previous.LeaseUntil,
		claimTokenHash: previous.ClaimTokenHash,
	}
	if err := requeueExpiredLease(ctx, s.exec, previous, snapshot, 0); err != nil {
		return taskpkg.Run{}, fmt.Errorf("release session task-run lease %q: %w", previous.ID, err)
	}
	if previous.IsTaskAnchored() {
		if err := clearTaskCurrentRunProjection(ctx, s.exec, previous.TaskID, previous.ID); err != nil {
			return taskpkg.Run{}, err
		}
	}

	updated, err := s.tasks.getTaskRunWithExecutor(ctx, s.exec, previous.ID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if !updated.IsNetworkWake() {
		return updated, nil
	}

	wakeID, targetSessionID, ownerKey := updated.NetworkWakeCorrelation()
	if err := appendNetworkWakeEventWithExecutor(ctx, s.exec, networkWakeEvent{
		workspaceID: updated.WorkspaceID, wakeID: wakeID, taskRunID: updated.ID,
		ownerKey: ownerKey, targetSessionID: targetSessionID,
		eventType: networkWakeEventReleased, state: updated.Status.String(),
		reason: release.Reason, actor: actor.Actor, at: release.Now,
	}); err != nil {
		return taskpkg.Run{}, err
	}
	return updated, nil
}
