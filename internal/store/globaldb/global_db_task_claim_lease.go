package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (g *TaskRunRepo) FailRunLease(
	ctx context.Context,
	failure taskpkg.LeaseFailure,
) (taskpkg.Run, error) {
	if err := g.checkReady(ctx, "fail task run lease"); err != nil {
		return taskpkg.Run{}, err
	}
	normalized, err := failure.Normalize(g.now())
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := normalized.Actor.Validate(); err != nil {
		return taskpkg.Run{}, err
	}

	var updated taskpkg.Run
	if err := g.tasks.withTaskImmediateTransaction(ctx, "fail task run lease", func(exec taskSQLExecutor) error {
		current, loadErr := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
		if loadErr != nil {
			return loadErr
		}
		if current.IsNetworkWake() {
			return fmt.Errorf(
				"%w: network_wake runs must be failed through network settlement",
				taskpkg.ErrValidation,
			)
		}
		var err error
		updated, err = g.failRunLeaseWithExecutor(ctx, exec, normalized)
		return err
	}); err != nil {
		return taskpkg.Run{}, err
	}
	return updated, nil
}

func (g *TaskRunRepo) failRunLeaseWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	normalized taskpkg.LeaseFailure,
) (taskpkg.Run, error) {
	current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := requireCurrentRunLease(current, normalized.ClaimToken, normalized.Now); err != nil {
		return taskpkg.Run{}, err
	}
	if err := requireLeaseTerminalTransition(current, taskpkg.TaskRunStatusFailed); err != nil {
		return taskpkg.Run{}, err
	}
	if current.RunKind.Normalize() == taskpkg.RunKindCoordinator {
		normalized.Failure = taskpkg.CanonicalCoordinatorRunFailure(normalized.Failure)
	}
	affected, err := sqlcgen.New(exec).FailTaskRunLease(ctx, sqlcgen.FailTaskRunLeaseParams{
		Status:         taskpkg.TaskRunStatusFailed.String(),
		EndedAt:        nullableTaskTime(normalized.Now),
		TokensUsed:     normalized.TokensUsed,
		Error:          nullableTaskString(normalized.Failure.Error),
		ID:             current.ID,
		ClaimTokenHash: nullableTaskString(current.ClaimTokenHash),
	})
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: fail task run lease %q: %w", current.ID, err)
	}
	if affected == 0 {
		return taskpkg.Run{}, fmt.Errorf("store: task run lease %q: %w", current.ID, taskpkg.ErrTaskRunNotFound)
	}
	if current.IsTaskAnchored() {
		if err := settleCoordinatorFailureLoopWithExecutor(ctx, exec, current, normalized); err != nil {
			return taskpkg.Run{}, err
		}
		if err := recordLoopNodeTerminalWithExecutor(
			ctx,
			exec,
			current,
			"failure",
			loopFailureOutputRef(normalized.Failure),
			nil,
			normalized.Now,
		); err != nil {
			return taskpkg.Run{}, err
		}
		if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
			return taskpkg.Run{}, err
		}
		if err := appendFailedRunLeaseWatchEvent(ctx, exec, current, normalized); err != nil {
			return taskpkg.Run{}, err
		}
	}
	return g.tasks.getTaskRunWithExecutor(ctx, exec, current.ID)
}

func appendFailedRunLeaseWatchEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.Run,
	failure taskpkg.LeaseFailure,
) error {
	return appendTaskEventPayloadWithExecutor(
		ctx,
		exec,
		current.TaskID,
		current.ID,
		string(hookspkg.HookTaskRunFailed),
		failure.Actor,
		failure.Now,
		taskRunFailedWatchEventPayload{
			Status:         taskpkg.TaskRunStatusFailed,
			Error:          failure.Failure.Error,
			Metadata:       failure.Failure.Metadata,
			ClaimTokenHash: current.ClaimTokenHash,
		},
	)
}

// ListAutonomyLeaseHandles returns internal-only lease handles for one session.
// Public task-run read projections keep claim_token masked.
func (g *TaskRunRepo) ListAutonomyLeaseHandles(
	ctx context.Context,
	sessionID string,
) (handles []taskpkg.AutonomyLeaseHandle, err error) {
	if err := g.checkReady(ctx, "list autonomy lease handles"); err != nil {
		return nil, err
	}
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", taskpkg.ErrValidation)
	}

	rows, err := g.queries.ListAutonomyLeaseHandles(
		ctx,
		sql.NullString{String: trimmedSessionID, Valid: true},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"store: list autonomy lease handles for session %q: %w",
			trimmedSessionID,
			err,
		)
	}
	handles = make([]taskpkg.AutonomyLeaseHandle, 0, len(rows))
	for _, row := range rows {
		handle, mapErr := autonomyLeaseHandleFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		handles = append(handles, handle)
	}
	return handles, nil
}

func autonomyLeaseHandleFromGenerated(row sqlcgen.ListAutonomyLeaseHandlesRow) (taskpkg.AutonomyLeaseHandle, error) {
	workspaceID := strings.TrimSpace(row.WorkspaceID)
	handle := taskpkg.AutonomyLeaseHandle{
		RunID: row.ID, TaskID: taskNullStringValue(row.TaskID), RunKind: taskpkg.ParseRunKind(row.RunKind).Normalize(),
		WorkspaceID: workspaceID, TargetSessionID: row.TargetSessionID, OwnerKey: row.OwnerKey,
		Status: taskpkg.ParseRunStatus(row.Status).Normalize(), SessionID: row.SessionID,
		ClaimToken: row.ClaimToken, ClaimTokenHash: row.ClaimTokenHash,
	}
	if row.ClaimedByKind.Valid || row.ClaimedByRef.Valid {
		handle.ClaimedBy = &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(row.ClaimedByKind.String)),
			Ref:  strings.TrimSpace(row.ClaimedByRef.String),
		}
	}
	if err := setAutonomyLeaseHandleTimestamps(&handle, row.LeaseUntil, row.HeartbeatAt); err != nil {
		return taskpkg.AutonomyLeaseHandle{}, err
	}
	return handle, nil
}

func setAutonomyLeaseHandleTimestamps(
	handle *taskpkg.AutonomyLeaseHandle,
	leaseUntilRaw sql.NullString,
	heartbeatAtRaw sql.NullString,
) error {
	if leaseUntilRaw.Valid && strings.TrimSpace(leaseUntilRaw.String) != "" {
		leaseUntil, err := store.ParseTimestamp(leaseUntilRaw.String)
		if err != nil {
			return fmt.Errorf("store: parse autonomy lease_until for run %q: %w", handle.RunID, err)
		}
		handle.LeaseUntil = leaseUntil
	}
	if heartbeatAtRaw.Valid && strings.TrimSpace(heartbeatAtRaw.String) != "" {
		heartbeatAt, err := store.ParseTimestamp(heartbeatAtRaw.String)
		if err != nil {
			return fmt.Errorf(
				"store: parse autonomy heartbeat_at for run %q: %w",
				handle.RunID,
				err,
			)
		}
		handle.HeartbeatAt = heartbeatAt
	}
	return nil
}

// RecoverExpiredRunLeases requeues stale active leases without issuing new ownership.
func (g *TaskRunRepo) RecoverExpiredRunLeases(
	ctx context.Context,
	recovery taskpkg.ExpiredLeaseRecovery,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	if err := g.checkReady(ctx, "recover expired task run leases"); err != nil {
		return nil, err
	}
	normalized, err := recovery.Normalize(g.now())
	if err != nil {
		return nil, err
	}

	recovered := make([]taskpkg.ExpiredLeaseRecoveryResult, 0)
	if err := g.tasks.withTaskImmediateTransaction(
		ctx,
		"recover expired task run leases",
		func(exec taskSQLExecutor) error {
			runIDs, err := expiredLeaseRunIDs(ctx, exec, normalized)
			if err != nil {
				return err
			}
			for _, runID := range runIDs {
				current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
				if err != nil {
					return err
				}
				if current.LeaseUntil.IsZero() || current.LeaseUntil.After(normalized.Now) {
					continue
				}
				snapshot := taskRunLeaseSnapshot{
					status:         current.Status,
					sessionID:      current.SessionID,
					leaseUntil:     current.LeaseUntil,
					claimTokenHash: current.ClaimTokenHash,
				}
				exhausted, err := g.recoverExpiredLeaseWithExecutor(ctx, exec, current, snapshot, normalized)
				if err != nil {
					return err
				}
				if current.IsTaskAnchored() {
					if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
						return err
					}
				}
				updated, err := g.tasks.getTaskRunWithExecutor(ctx, exec, current.ID)
				if err != nil {
					return err
				}
				result := newExpiredLeaseRecoveryResult(&updated, snapshot, normalized.Reason, exhausted)
				if current.IsTaskAnchored() {
					taskRecord, err := g.tasks.getTaskWithExecutor(ctx, exec, current.TaskID)
					if err != nil {
						return err
					}
					if err := appendExpiredLeaseRecoveryEvents(
						ctx,
						exec,
						&result,
						taskRecord.Status,
						normalized.Actor,
						normalized.Now,
					); err != nil {
						return err
					}
				}
				recovered = append(recovered, result)
			}
			return nil
		},
	); err != nil {
		return nil, err
	}

	return recovered, nil
}

func newExpiredLeaseRecoveryResult(
	run *taskpkg.Run,
	snapshot taskRunLeaseSnapshot,
	reason string,
	exhausted bool,
) taskpkg.ExpiredLeaseRecoveryResult {
	return taskpkg.ExpiredLeaseRecoveryResult{
		Run:                    *run,
		PreviousRunStatus:      snapshot.status,
		PreviousSessionID:      snapshot.sessionID,
		PreviousLeaseUntil:     snapshot.leaseUntil,
		PreviousClaimTokenHash: snapshot.claimTokenHash,
		Reason:                 reason,
		Exhausted:              exhausted,
	}
}

func appendExpiredLeaseRecoveryEvents(
	ctx context.Context,
	exec taskSQLExecutor,
	result *taskpkg.ExpiredLeaseRecoveryResult,
	taskStatus taskpkg.Status,
	actor taskpkg.ActorContext,
	at time.Time,
) error {
	actor = expiredLeaseRecoveryActor(actor)
	if err := appendTaskEventPayloadWithExecutor(
		ctx,
		exec,
		result.Run.TaskID,
		result.Run.ID,
		eventspkg.TaskRunLeaseExpired,
		actor,
		at,
		taskpkg.ExpiredLeaseEventPayload{
			PreviousStatus:               result.PreviousRunStatus,
			Status:                       result.Run.Status,
			TaskStatus:                   taskStatus,
			Reason:                       result.Reason,
			SessionID:                    result.PreviousSessionID,
			LeaseUntil:                   result.PreviousLeaseUntil,
			PreviousTokenHash:            result.PreviousClaimTokenHash,
			ResolvedNetworkParticipation: participation.CloneSpec(result.Run.NetworkSpecSnapshot()),
		},
	); err != nil {
		return err
	}
	if !result.Exhausted {
		return nil
	}
	return appendTaskEventPayloadWithExecutor(
		ctx,
		exec,
		result.Run.TaskID,
		result.Run.ID,
		eventspkg.TaskRunNeedsAttention,
		actor,
		at,
		taskpkg.RunNeedsAttentionEventPayload{
			PreviousStatus:               result.PreviousRunStatus,
			Status:                       result.Run.Status,
			SessionID:                    result.PreviousSessionID,
			Diagnostic:                   result.Run.Error,
			QueuedAt:                     result.Run.QueuedAt,
			ResolvedNetworkParticipation: participation.CloneSpec(result.Run.NetworkSpecSnapshot()),
		},
	)
}
