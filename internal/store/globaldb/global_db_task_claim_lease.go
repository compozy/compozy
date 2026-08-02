package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

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
