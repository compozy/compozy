package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const updateNonTerminalTaskRunFixtureSQL = `
UPDATE task_runs
SET task_id = :task_id,
    workspace_id = :workspace_id,
    run_kind = :run_kind,
    loop_run_id = :loop_run_id,
    status = :status,
    attempt = :attempt,
    recovery_count = :recovery_count,
    previous_run_id = :previous_run_id,
    failure_kind = :failure_kind,
    claimed_by_kind = :claimed_by_kind,
    claimed_by_ref = :claimed_by_ref,
    session_id = :session_id,
    origin_kind = :origin_kind,
    origin_ref = :origin_ref,
    idempotency_key = :idempotency_key,
    network_spec_json = :network_spec_json,
    network_mode = :network_mode,
    network_channel = :network_channel,
    network_source = :network_source,
    designation_group_id = :designation_group_id,
    claim_token = CASE
        WHEN :status IN ('claimed', 'starting', 'running')
         AND :claim_token_hash IS NOT NULL
         AND claim_token_hash = :claim_token_hash
        THEN claim_token
        ELSE NULL
    END,
    claim_token_hash = :claim_token_hash,
    lease_until = :lease_until,
    heartbeat_at = :heartbeat_at,
    queued_at = :queued_at,
    claimed_at = :claimed_at,
    started_at = :started_at,
    ended_at = :ended_at,
    tokens_used = :tokens_used,
    error = :error,
    metadata_json = :metadata_json,
    result_json = :result_json,
    review_required = :review_required,
    review_request_round = :review_request_round,
    review_policy_snapshot = :review_policy_snapshot,
    review_request_id = :review_request_id,
    parent_run_id = :parent_run_id,
    review_id = :review_id,
    review_round = :review_round,
    continuation_reason = :continuation_reason,
    missing_work_json = :missing_work_json,
    next_round_guidance = :next_round_guidance,
    network_wake_id = :network_wake_id,
    network_target_session_id = :network_target_session_id,
    network_owner_key = :network_owner_key
WHERE id = :id
  AND status IN ('queued', 'claimed', 'starting', 'running')`

// CreateTaskRun is a storage-fixture escape available only while compiling the
// GlobalDB package's own tests. Production creation is owned by the queue-first
// execution transaction or a purpose-specific importer.
func (g *TaskRepo) CreateTaskRun(ctx context.Context, run taskpkg.Run) error {
	if err := g.checkReady(ctx, "create task run fixture"); err != nil {
		return err
	}

	normalized, err := g.normalizeTaskRunForCreate(run)
	if err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "create task run fixture", func(exec taskSQLExecutor) error {
		if normalized.IsTaskAnchored() {
			taskRecord, loadErr := g.getTaskWithExecutor(ctx, exec, normalized.TaskID)
			if loadErr != nil {
				return loadErr
			}
			normalized, loadErr = bindTaskRunWorkspace(normalized, taskRecord)
			if loadErr != nil {
				return loadErr
			}
		}
		if err := insertTaskRunWithExecutor(ctx, exec, normalized); err != nil {
			return err
		}
		return replaceTaskRunCapabilitiesWithExecutor(ctx, exec, normalized)
	})
}

// UpdateNonTerminalTaskRun is a package-test fixture for constructing legacy
// lifecycle snapshots. Production code has no generic full-row writer.
func (g *TaskRepo) UpdateNonTerminalTaskRun(ctx context.Context, run taskpkg.Run) error {
	if err := g.checkReady(ctx, "update nonterminal task run fixture"); err != nil {
		return err
	}
	normalized, err := g.normalizeTaskRunForUpdate(run)
	if err != nil {
		return err
	}
	if !isFixtureNonTerminalTaskRunStatus(normalized.Status) {
		return fmt.Errorf(
			"%w: task run %q status %q requires an authoritative mutation",
			taskpkg.ErrInvalidStatusTransition,
			normalized.ID,
			normalized.Status.Normalize(),
		)
	}
	return g.withTaskImmediateTransaction(ctx, "update nonterminal task run fixture", func(exec taskSQLExecutor) error {
		return g.updateNonTerminalTaskRunFixtureWithExecutor(ctx, exec, normalized)
	})
}

func (g *TaskRepo) updateNonTerminalTaskRunFixtureWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	normalized taskpkg.Run,
) error {
	current, err := g.getTaskRunWithExecutor(ctx, exec, normalized.ID)
	if err != nil {
		return err
	}
	if !isFixtureNonTerminalTaskRunStatus(current.Status) {
		return fmt.Errorf(
			"%w: task run %q is %q and cannot be rewritten by the fixture",
			taskpkg.ErrInvalidStatusTransition,
			current.ID,
			current.Status.Normalize(),
		)
	}
	if err := taskpkg.ValidateImmutableRunFields(current, normalized); err != nil {
		return err
	}
	currentSessionID := strings.TrimSpace(current.SessionID)
	nextSessionID := strings.TrimSpace(normalized.SessionID)
	if currentSessionID != "" &&
		nextSessionID != currentSessionID &&
		(nextSessionID != "" || normalized.Status.Normalize() != taskpkg.TaskRunStatusQueued) &&
		!allowsManagedTaskRunFixtureSessionTransfer(current, normalized) {
		return taskpkg.ErrSessionAlreadyBound
	}
	if normalized.QueuedAt.IsZero() {
		normalized.QueuedAt = current.QueuedAt
	}
	params, err := taskRunParams(normalized)
	if err != nil {
		return err
	}
	result, err := exec.ExecContext(ctx, updateNonTerminalTaskRunFixtureSQL, taskRunFixtureUpdateArgs(&params)...)
	if err != nil {
		return fmt.Errorf("store: update task run fixture %q: %w", normalized.ID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: read updated task run fixture rows for %q: %w", normalized.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: task run %q", taskpkg.ErrTaskRunNotFound, normalized.ID)
	}
	if normalized.IsTaskAnchored() {
		if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, current, normalized); err != nil {
			return err
		}
	}
	return replaceTaskRunCapabilitiesWithExecutor(ctx, exec, normalized)
}

func (g *TaskRepo) withTaskMutationTransactionForTest(
	ctx context.Context,
	action string,
	fn func(*taskMutationTxStore) error,
) error {
	if err := g.checkReady(ctx, action); err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, action, func(exec taskSQLExecutor) error {
		return fn(&taskMutationTxStore{
			taskExecutionTxStore: &taskExecutionTxStore{
				deleteTaskTxStore: &deleteTaskTxStore{tasks: g, exec: exec},
			},
		})
	})
}

func taskRunFixtureUpdateArgs(params *sqlcgen.InsertTaskRunParams) []any {
	return []any{
		sql.Named("task_id", params.TaskID),
		sql.Named("workspace_id", params.WorkspaceID),
		sql.Named("run_kind", params.RunKind),
		sql.Named("loop_run_id", params.LoopRunID),
		sql.Named("status", params.Status),
		sql.Named("attempt", params.Attempt),
		sql.Named("recovery_count", params.RecoveryCount),
		sql.Named("previous_run_id", params.PreviousRunID),
		sql.Named("failure_kind", params.FailureKind),
		sql.Named("claimed_by_kind", params.ClaimedByKind),
		sql.Named("claimed_by_ref", params.ClaimedByRef),
		sql.Named("session_id", params.SessionID),
		sql.Named("origin_kind", params.OriginKind),
		sql.Named("origin_ref", params.OriginRef),
		sql.Named("idempotency_key", params.IdempotencyKey),
		sql.Named("network_spec_json", params.NetworkSpecJson),
		sql.Named("network_mode", params.NetworkMode),
		sql.Named("network_channel", params.NetworkChannel),
		sql.Named("network_source", params.NetworkSource),
		sql.Named("designation_group_id", params.DesignationGroupID),
		sql.Named("claim_token_hash", params.ClaimTokenHash),
		sql.Named("lease_until", params.LeaseUntil),
		sql.Named("heartbeat_at", params.HeartbeatAt),
		sql.Named("queued_at", params.QueuedAt),
		sql.Named("claimed_at", params.ClaimedAt),
		sql.Named("started_at", params.StartedAt),
		sql.Named("ended_at", params.EndedAt),
		sql.Named("tokens_used", params.TokensUsed),
		sql.Named("error", params.Error),
		sql.Named("metadata_json", params.MetadataJson),
		sql.Named("result_json", params.ResultJson),
		sql.Named("review_required", params.ReviewRequired),
		sql.Named("review_request_round", params.ReviewRequestRound),
		sql.Named("review_policy_snapshot", params.ReviewPolicySnapshot),
		sql.Named("review_request_id", params.ReviewRequestID),
		sql.Named("parent_run_id", params.ParentRunID),
		sql.Named("review_id", params.ReviewID),
		sql.Named("review_round", params.ReviewRound),
		sql.Named("continuation_reason", params.ContinuationReason),
		sql.Named("missing_work_json", params.MissingWorkJson),
		sql.Named("next_round_guidance", params.NextRoundGuidance),
		sql.Named("network_wake_id", params.NetworkWakeID),
		sql.Named("network_target_session_id", params.NetworkTargetSessionID),
		sql.Named("network_owner_key", params.NetworkOwnerKey),
		sql.Named("id", params.ID),
	}
}

func isFixtureNonTerminalTaskRunStatus(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusQueued,
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning:
		return true
	default:
		return false
	}
}

func allowsManagedTaskRunFixtureSessionTransfer(current taskpkg.Run, next taskpkg.Run) bool {
	currentStatus := current.Status.Normalize()
	nextStatus := next.Status.Normalize()
	currentSessionID := strings.TrimSpace(current.SessionID)
	if strings.TrimSpace(next.SessionID) == "" {
		return false
	}
	if current.ClaimedBy == nil ||
		current.ClaimedBy.Kind.Normalize() != taskpkg.ActorKindAgentSession ||
		strings.TrimSpace(current.ClaimedBy.Ref) != currentSessionID {
		return false
	}
	if currentStatus == taskpkg.TaskRunStatusClaimed && nextStatus == taskpkg.TaskRunStatusStarting {
		return true
	}
	return currentStatus == taskpkg.TaskRunStatusStarting &&
		nextStatus == taskpkg.TaskRunStatusStarting &&
		current.StartedAt.IsZero() &&
		next.StartedAt.IsZero()
}

// ClaimNextRun is a package-test fixture for exercising elementary lease SQL.
func (g *TaskRunRepo) ClaimNextRun(
	ctx context.Context,
	criteria taskpkg.ClaimCriteria,
) (taskpkg.ClaimResult, error) {
	if err := g.checkReady(ctx, "claim next task run fixture"); err != nil {
		return taskpkg.ClaimResult{}, err
	}
	normalized, err := criteria.Normalize(g.now())
	if err != nil {
		return taskpkg.ClaimResult{}, err
	}
	var result taskpkg.ClaimResult
	err = g.tasks.withTaskImmediateTransaction(ctx, "claim next task run fixture", func(exec taskSQLExecutor) error {
		var claimErr error
		result, claimErr = g.claimNextRunWithExecutor(ctx, exec, normalized)
		return claimErr
	})
	return result, err
}

// HeartbeatRunLease is a package-test fixture for elementary lease SQL.
func (g *TaskRunRepo) HeartbeatRunLease(
	ctx context.Context,
	heartbeat taskpkg.LeaseHeartbeat,
) (taskpkg.Run, error) {
	normalized, err := heartbeat.Normalize(g.now())
	if err != nil {
		return taskpkg.Run{}, err
	}
	var updated taskpkg.Run
	err = g.tasks.withTaskImmediateTransaction(
		ctx,
		"heartbeat task run lease fixture",
		func(exec taskSQLExecutor) error {
			var mutationErr error
			updated, mutationErr = g.heartbeatRunLeaseWithExecutor(ctx, exec, normalized)
			return mutationErr
		},
	)
	return updated, err
}

// ReleaseRunLease is a package-test fixture for elementary lease SQL.
func (g *TaskRunRepo) ReleaseRunLease(
	ctx context.Context,
	release taskpkg.LeaseRelease,
) (taskpkg.Run, error) {
	normalized, err := release.Normalize(g.now())
	if err != nil {
		return taskpkg.Run{}, err
	}
	var updated taskpkg.Run
	err = g.tasks.withTaskImmediateTransaction(ctx, "release task run lease fixture", func(exec taskSQLExecutor) error {
		var mutationErr error
		updated, mutationErr = g.releaseRunLeaseWithExecutor(ctx, exec, normalized)
		return mutationErr
	})
	return updated, err
}

// CompleteRunLease is a package-test compatibility fixture. Production callers
// use CompleteRunLeaseSettlement through the task service.
func (g *TaskRunRepo) CompleteRunLease(
	ctx context.Context,
	completion taskpkg.LeaseCompletion,
) (taskpkg.Run, error) {
	settlement, err := g.CompleteRunLeaseSettlement(ctx, completion)
	if err != nil {
		return taskpkg.Run{}, err
	}
	return settlement.Run, nil
}

// FailRunLease is a package-test fixture for elementary lease SQL.
func (g *TaskRunRepo) FailRunLease(
	ctx context.Context,
	failure taskpkg.LeaseFailure,
) (taskpkg.Run, error) {
	normalized, err := failure.Normalize(g.now())
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := normalized.Actor.Validate(); err != nil {
		return taskpkg.Run{}, err
	}
	var mutation taskpkg.FailedRunLeaseMutation
	err = g.tasks.withTaskImmediateTransaction(ctx, "fail task run lease fixture", func(exec taskSQLExecutor) error {
		var mutationErr error
		mutation, mutationErr = g.failRunLeaseMutationWithExecutor(ctx, exec, normalized)
		if mutationErr != nil {
			return mutationErr
		}
		return appendTaskEventPayloadWithExecutor(
			ctx,
			exec,
			mutation.Run.TaskID,
			mutation.Run.ID,
			string(hookspkg.HookTaskRunFailed),
			normalized.Actor,
			normalized.Now,
			taskpkg.NewRunLeaseFailedEventPayload(&mutation),
		)
	})
	return mutation.Run, err
}

// ReserveQueuedRun is a package-test fixture for elementary reservation SQL.
func (g *TaskRepo) ReserveQueuedRun(
	ctx context.Context,
	reservation taskpkg.QueueRunReservation,
) (taskpkg.Task, taskpkg.Run, bool, error) {
	input, err := g.normalizeQueuedRunReservationInput(reservation)
	if err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}
	var taskRecord taskpkg.Task
	var run taskpkg.Run
	var existing bool
	err = g.withTaskImmediateTransaction(ctx, "reserve queued task run fixture", func(exec taskSQLExecutor) error {
		var reserveErr error
		taskRecord, run, existing, reserveErr = g.reserveQueuedRunWithExecutor(ctx, exec, input)
		return reserveErr
	})
	return taskRecord, run, existing, err
}
