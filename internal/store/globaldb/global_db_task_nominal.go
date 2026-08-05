package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *TaskRunRepo) admitRunDirectExecutionWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation taskpkg.RunDirectExecutionAdmissionMutation,
) (taskpkg.NominalRunMutationResult, error) {
	previous, err := g.requireFencedTaskRun(ctx, exec, mutation.Fence(), true)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	if previous.Status.Normalize() != taskpkg.TaskRunStatusQueued ||
		previous.RunKind.Normalize() != taskpkg.RunKindWorker ||
		strings.TrimSpace(previous.LoopRunID) != "" {
		return taskpkg.NominalRunMutationResult{}, invalidNominalRunTransition(previous, "direct execution admission")
	}
	actor := mutation.Actor()
	if err := actor.Validate(); err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	claimedAt := mutation.ClaimedAt().UTC()
	if claimedAt.IsZero() {
		return taskpkg.NominalRunMutationResult{}, fmt.Errorf(
			"%w: direct execution claimed_at is required",
			taskpkg.ErrValidation,
		)
	}
	params := nominalRunFenceParams(mutation.Fence())
	affected, err := sqlcgen.New(exec).AdmitTaskRunDirectExecution(
		ctx,
		sqlcgen.AdmitTaskRunDirectExecutionParams{
			ClaimedByKind:          nullableTaskString(string(actor.Actor.Kind.Normalize())),
			ClaimedByRef:           nullableTaskString(actor.Actor.Ref),
			ClaimedAt:              nullableTaskTime(claimedAt),
			ID:                     params.id,
			ExpectedTaskID:         params.taskID,
			ExpectedWorkspaceID:    params.workspaceID,
			ExpectedSessionID:      params.sessionID,
			ExpectedClaimTokenHash: params.claimTokenHash,
			ExpectedLeaseUntil:     params.leaseUntil,
		},
	)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, fmt.Errorf("store: admit task run direct execution: %w", err)
	}
	return g.finishNominalTaskRunMutation(ctx, exec, previous, affected)
}

func (g *TaskRunRepo) transitionRunStartingWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation taskpkg.RunStartingMutation,
) (taskpkg.NominalRunMutationResult, error) {
	previous, err := g.requireFencedTaskRun(ctx, exec, mutation.Fence(), true)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	if previous.Status.Normalize() != taskpkg.TaskRunStatusClaimed {
		return taskpkg.NominalRunMutationResult{}, invalidNominalRunTransition(previous, "starting")
	}
	params := nominalRunFenceParams(mutation.Fence())
	affected, err := sqlcgen.New(exec).TransitionTaskRunStarting(
		ctx,
		sqlcgen.TransitionTaskRunStartingParams{
			ID:                     params.id,
			ExpectedTaskID:         params.taskID,
			ExpectedWorkspaceID:    params.workspaceID,
			ExpectedSessionID:      params.sessionID,
			ExpectedClaimTokenHash: params.claimTokenHash,
			ExpectedLeaseUntil:     params.leaseUntil,
		},
	)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, fmt.Errorf("store: transition task run starting: %w", err)
	}
	return g.finishNominalTaskRunMutation(ctx, exec, previous, affected)
}

func (g *TaskRunRepo) bindRunSessionWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation taskpkg.RunSessionBindingMutation,
) (taskpkg.NominalRunMutationResult, error) {
	previous, err := g.requireFencedTaskRun(ctx, exec, mutation.Fence(), true)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	status := previous.Status.Normalize()
	if status != taskpkg.TaskRunStatusClaimed && status != taskpkg.TaskRunStatusStarting {
		return taskpkg.NominalRunMutationResult{}, invalidNominalRunTransition(previous, "bind session")
	}
	sessionID, err := requireTaskValue(mutation.SessionID(), "task run session id")
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	if current := strings.TrimSpace(previous.SessionID); current != "" && current != sessionID &&
		!allowsManagedSessionTransfer(previous) {
		return taskpkg.NominalRunMutationResult{}, taskpkg.ErrSessionAlreadyBound
	}
	params := nominalRunFenceParams(mutation.Fence())
	affected, err := sqlcgen.New(exec).BindTaskRunSession(ctx, sqlcgen.BindTaskRunSessionParams{
		SessionID:              nullableTaskString(sessionID),
		ID:                     params.id,
		ExpectedTaskID:         params.taskID,
		ExpectedWorkspaceID:    params.workspaceID,
		ExpectedStatus:         params.status,
		ExpectedSessionID:      params.sessionID,
		ExpectedClaimTokenHash: params.claimTokenHash,
		ExpectedLeaseUntil:     params.leaseUntil,
	})
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, fmt.Errorf("store: bind task run session: %w", err)
	}
	if affected == 0 {
		return taskpkg.NominalRunMutationResult{}, taskpkg.ErrSessionAlreadyBound
	}
	return g.finishNominalTaskRunMutation(ctx, exec, previous, affected)
}

func (g *TaskRunRepo) transitionRunRunningWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation taskpkg.RunRunningMutation,
) (taskpkg.NominalRunMutationResult, error) {
	previous, err := g.requireFencedTaskRun(ctx, exec, mutation.Fence(), true)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	if previous.Status.Normalize() != taskpkg.TaskRunStatusStarting ||
		(strings.TrimSpace(previous.SessionID) == "" && previous.RunKind.Normalize() != taskpkg.RunKindCoordinator) {
		return taskpkg.NominalRunMutationResult{}, invalidNominalRunTransition(previous, "running")
	}
	startedAt := mutation.StartedAt().UTC()
	if startedAt.IsZero() {
		return taskpkg.NominalRunMutationResult{}, fmt.Errorf("%w: run started_at is required", taskpkg.ErrValidation)
	}
	params := nominalRunFenceParams(mutation.Fence())
	affected, err := sqlcgen.New(exec).TransitionTaskRunRunning(
		ctx,
		sqlcgen.TransitionTaskRunRunningParams{
			StartedAt:              nullableTaskTime(startedAt),
			ID:                     params.id,
			ExpectedTaskID:         params.taskID,
			ExpectedWorkspaceID:    params.workspaceID,
			ExpectedSessionID:      params.sessionID,
			ExpectedClaimTokenHash: params.claimTokenHash,
			ExpectedLeaseUntil:     params.leaseUntil,
		},
	)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, fmt.Errorf("store: transition task run running: %w", err)
	}
	return g.finishNominalTaskRunMutation(ctx, exec, previous, affected)
}

func (g *TaskRunRepo) recoverTaskRunOnBootWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation taskpkg.RunBootRecoveryMutation,
) (taskpkg.NominalRunMutationResult, error) {
	previous, err := g.requireFencedTaskRun(ctx, exec, mutation.Fence(), true)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	affected, err := applyBootRecoveryNominalWrite(ctx, exec, mutation.Fence(), mutation.Action(), mutation.StartedAt())
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	return g.finishNominalTaskRunMutation(ctx, exec, previous, affected)
}

type nominalFenceParams struct {
	id             string
	taskID         sql.NullString
	workspaceID    sql.NullString
	status         string
	sessionID      sql.NullString
	claimTokenHash sql.NullString
	leaseUntil     sql.NullString
}

func nominalRunFenceParams(fence taskpkg.RunMutationFence) nominalFenceParams {
	return nominalFenceParams{
		id:             strings.TrimSpace(fence.RunID()),
		taskID:         nullableTaskString(fence.TaskID()),
		workspaceID:    nullableTaskString(fence.WorkspaceID()),
		status:         fence.Status().String(),
		sessionID:      nullableTaskString(fence.SessionID()),
		claimTokenHash: nullableTaskString(fence.ClaimTokenHash()),
		leaseUntil:     nullableTaskTime(fence.LeaseUntil()),
	}
}

func (g *TaskRunRepo) requireFencedTaskRun(
	ctx context.Context,
	exec taskSQLExecutor,
	fence taskpkg.RunMutationFence,
	taskAnchored bool,
) (taskpkg.Run, error) {
	runID, err := requireTaskValue(fence.RunID(), "task run id")
	if err != nil {
		return taskpkg.Run{}, err
	}
	current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if !fence.Matches(current) {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run %q ownership changed before nominal mutation",
			taskpkg.ErrInvalidStatusTransition,
			runID,
		)
	}
	if current.IsTaskAnchored() != taskAnchored {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run %q has the wrong ownership kind",
			taskpkg.ErrInvalidStatusTransition,
			runID,
		)
	}
	return current, nil
}

func (g *TaskRunRepo) finishNominalTaskRunMutation(
	ctx context.Context,
	exec taskSQLExecutor,
	previous taskpkg.Run,
	affected int64,
) (taskpkg.NominalRunMutationResult, error) {
	if affected == 0 {
		return taskpkg.NominalRunMutationResult{}, invalidNominalRunTransition(previous, "requested state")
	}
	updated, err := g.tasks.getTaskRunWithExecutor(ctx, exec, previous.ID)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	if updated.IsTaskAnchored() {
		if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, previous, updated); err != nil {
			return taskpkg.NominalRunMutationResult{}, err
		}
	}
	return taskpkg.NominalRunMutationResult{Previous: previous, Run: updated}, nil
}

func invalidNominalRunTransition(run taskpkg.Run, operation string) error {
	return fmt.Errorf(
		"%w: task run %q in %q cannot apply %s",
		taskpkg.ErrInvalidStatusTransition,
		run.ID,
		run.Status.Normalize(),
		operation,
	)
}

func allowsManagedSessionTransfer(run taskpkg.Run) bool {
	return run.ClaimedBy != nil &&
		run.ClaimedBy.Kind.Normalize() == taskpkg.ActorKindAgentSession &&
		strings.TrimSpace(run.ClaimedBy.Ref) == strings.TrimSpace(run.SessionID)
}

func applyBootRecoveryNominalWrite(
	ctx context.Context,
	exec taskSQLExecutor,
	fence taskpkg.RunMutationFence,
	action taskpkg.RunBootRecoveryAction,
	startedAt time.Time,
) (int64, error) {
	params := nominalRunFenceParams(fence)
	queries := sqlcgen.New(exec)
	switch action.Normalize() {
	case taskpkg.RunBootRecoveryRequeue:
		return queries.RecoverTaskRunByRequeue(ctx, sqlcgen.RecoverTaskRunByRequeueParams{
			ID: params.id, ExpectedTaskID: params.taskID, ExpectedWorkspaceID: params.workspaceID,
			ExpectedStatus: params.status, ExpectedSessionID: params.sessionID,
			ExpectedClaimTokenHash: params.claimTokenHash, ExpectedLeaseUntil: params.leaseUntil,
		})
	case taskpkg.RunBootRecoveryMarkRunning:
		if strings.TrimSpace(fence.SessionID()) == "" || startedAt.IsZero() {
			return 0, fmt.Errorf("%w: mark-running recovery requires session_id and started_at", taskpkg.ErrValidation)
		}
		return queries.RecoverTaskRunByMarkRunning(ctx, sqlcgen.RecoverTaskRunByMarkRunningParams{
			StartedAt: nullableTaskTime(startedAt.UTC()), ID: params.id,
			ExpectedTaskID: params.taskID, ExpectedWorkspaceID: params.workspaceID,
			ExpectedStatus:    params.status,
			ExpectedSessionID: params.sessionID, ExpectedClaimTokenHash: params.claimTokenHash,
			ExpectedLeaseUntil: params.leaseUntil,
		})
	default:
		return 0, fmt.Errorf("%w: unsupported nominal boot recovery %q", taskpkg.ErrValidation, action)
	}
}
