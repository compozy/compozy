package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// CompleteRunLeaseSettlement completes one fenced run and atomically applies
// successful task-hierarchy aggregation.
func (g *TaskRunRepo) CompleteRunLeaseSettlement(
	ctx context.Context,
	completion taskpkg.LeaseCompletion,
) (taskpkg.CompletedRunSettlement, error) {
	if err := g.checkReady(ctx, "complete task run lease"); err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}
	normalized, err := completion.Normalize(g.now())
	if err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}
	if err := normalized.Actor.Validate(); err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}

	var settlement taskpkg.CompletedRunSettlement
	if err := g.tasks.withTaskImmediateTransaction(ctx, "complete task run lease", func(exec taskSQLExecutor) error {
		current, loadErr := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
		if loadErr != nil {
			return loadErr
		}
		if current.IsNetworkWake() {
			return fmt.Errorf(
				"%w: network_wake runs must be completed through network settlement",
				taskpkg.ErrValidation,
			)
		}
		updated, err := g.completeRunLeaseWithExecutor(ctx, exec, normalized)
		if err != nil {
			return err
		}
		if updated.IsNetworkWake() {
			settlement = taskpkg.CompletedRunSettlement{Run: updated}
			return nil
		}
		settlement, err = g.tasks.settleCompletedTaskHierarchyWithExecutor(
			ctx,
			exec,
			updated.TaskID,
			normalized.Actor,
			normalized.Now,
		)
		if err != nil {
			return err
		}
		settlement.Run = updated
		return nil
	}); err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}
	return settlement, nil
}

func (g *TaskRunRepo) completeRunLeaseWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	normalized taskpkg.LeaseCompletion,
) (taskpkg.Run, error) {
	current, err := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := requireCurrentRunLease(current, normalized.ClaimToken, normalized.Now); err != nil {
		return taskpkg.Run{}, err
	}
	if err := requireLeaseTerminalTransition(current, taskpkg.TaskRunStatusCompleted); err != nil {
		return taskpkg.Run{}, err
	}
	if err := g.verifyCompletionCreatedTaskClaims(ctx, exec, current, normalized.CreatedTaskIDs); err != nil {
		return taskpkg.Run{}, err
	}
	resultPayload, err := normalized.Result.StoredValue()
	if err != nil {
		return taskpkg.Run{}, err
	}
	resultPayload = normalizeTaskJSON(resultPayload)
	outputRef := ""
	if normalized.Result.CoordinatorControl == nil {
		resultPayload, outputRef, err = g.storeLoopResultPayloadByRefIfLarge(ctx, exec, current, normalized)
		if err != nil {
			return taskpkg.Run{}, err
		}
	}
	if err := completeRunLeaseRowWithExecutor(ctx, exec, current, normalized, resultPayload); err != nil {
		return taskpkg.Run{}, err
	}
	if err := recordCompletedRunLoopOutput(ctx, exec, current, normalized, outputRef, resultPayload); err != nil {
		return taskpkg.Run{}, err
	}
	if current.IsTaskAnchored() {
		if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
			return taskpkg.Run{}, err
		}
		if err := resetTaskBlockRecurrencesWithExecutor(ctx, exec, current.TaskID); err != nil {
			return taskpkg.Run{}, err
		}
		if err := appendTaskEventPayloadWithExecutor(
			ctx,
			exec,
			current.TaskID,
			current.ID,
			string(hookspkg.HookTaskRunCompleted),
			normalized.Actor,
			normalized.Now,
			taskRunCompletedWatchEventPayload{
				Status:         taskpkg.TaskRunStatusCompleted,
				Result:         resultPayload,
				ClaimTokenHash: current.ClaimTokenHash,
			},
		); err != nil {
			return taskpkg.Run{}, err
		}
	}
	updated, err := g.tasks.getTaskRunWithExecutor(ctx, exec, current.ID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	return updated, nil
}

func recordCompletedRunLoopOutput(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.Run,
	completion taskpkg.LeaseCompletion,
	outputRef string,
	resultPayload json.RawMessage,
) error {
	if current.IsNetworkWake() {
		return nil
	}
	if completion.Result.CoordinatorControl != nil {
		recorded, err := updateLoopNodeOutputStatusWithExecutor(
			ctx,
			exec,
			current,
			strings.TrimSpace(current.LoopRunID),
			looppkg.GenerationOutputStatusControlPending,
			"",
			"",
		)
		if err != nil {
			return err
		}
		if !recorded {
			return fmt.Errorf(
				"%w: coordinator control output for task run %q was fenced",
				looppkg.ErrStaleGenerationOutput,
				current.ID,
			)
		}
		return nil
	}
	childLoopRunID, awaitingChild, err := awaitedChildLoopRunID(current, resultPayload)
	if err != nil {
		return err
	}
	if awaitingChild {
		return recordAwaitingChildLoopOutputWithExecutor(
			ctx,
			exec,
			current,
			childLoopRunID,
			loopNodeTerminalOutputRef(outputRef, resultPayload),
			completion.Now,
		)
	}
	return recordLoopNodeTerminalWithExecutor(
		ctx,
		exec,
		current,
		"success",
		loopNodeTerminalOutputRef(outputRef, resultPayload),
		resultPayload,
		completion.Now,
	)
}

func (g *TaskRunRepo) storeLoopResultPayloadByRefIfLarge(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	completion taskpkg.LeaseCompletion,
) (json.RawMessage, string, error) {
	resultPayload := normalizeTaskJSON(completion.Result.Value)
	if !loopNodeRunShouldExternalizeResult(run, resultPayload) {
		return resultPayload, "", nil
	}
	outputRef := looppkg.OutputRefForPayload(resultPayload)
	if err := upsertLoopOutputBlobWithExecutor(ctx, exec, outputRef, resultPayload, completion.Now); err != nil {
		return nil, "", err
	}
	return nil, outputRef, nil
}

func completeRunLeaseRowWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.Run,
	normalized taskpkg.LeaseCompletion,
	resultPayload json.RawMessage,
) error {
	affected, err := sqlcgen.New(exec).CompleteTaskRunLease(ctx, sqlcgen.CompleteTaskRunLeaseParams{
		Status:         taskpkg.TaskRunStatusCompleted.String(),
		EndedAt:        nullableTaskTime(normalized.Now),
		TokensUsed:     normalized.TokensUsed,
		ResultJson:     nullableTaskRawJSON(resultPayload),
		ID:             current.ID,
		ClaimTokenHash: nullableTaskString(current.ClaimTokenHash),
	})
	if err != nil {
		return fmt.Errorf("store: complete task run lease %q: %w", current.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: task run lease %q: %w", current.ID, taskpkg.ErrTaskRunNotFound)
	}
	return nil
}

func loopNodeRunShouldExternalizeResult(run taskpkg.Run, payload json.RawMessage) bool {
	if !run.IsLoopWorker() {
		return false
	}
	return looppkg.OutputPayloadRequiresRef(payload)
}

func (g *TaskRunRepo) verifyCompletionCreatedTaskClaims(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	claimedTaskIDs []string,
) error {
	if len(claimedTaskIDs) == 0 {
		return nil
	}
	taskRecord, err := g.tasks.getTaskWithExecutor(ctx, exec, run.TaskID)
	if err != nil {
		return err
	}
	sessionID := strings.TrimSpace(run.SessionID)
	invalidTaskIDs := make([]string, 0)
	for _, claimedTaskID := range claimedTaskIDs {
		ok := false
		if sessionID != "" {
			var lookupErr error
			ok, lookupErr = completionCreatedTaskClaimMatches(
				ctx,
				exec,
				claimedTaskID,
				taskRecord.WorkspaceID,
				sessionID,
			)
			if lookupErr != nil {
				return lookupErr
			}
		}
		if !ok {
			invalidTaskIDs = append(invalidTaskIDs, claimedTaskID)
		}
	}
	if len(invalidTaskIDs) > 0 {
		return taskpkg.NewHallucinatedTaskRefsError(run, claimedTaskIDs, invalidTaskIDs)
	}
	return nil
}

func completionCreatedTaskClaimMatches(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	workspaceID string,
	sessionID string,
) (bool, error) {
	found, err := sqlcgen.New(exec).CompletionCreatedTaskClaimExists(
		ctx,
		sqlcgen.CompletionCreatedTaskClaimExistsParams{
			ID:            strings.TrimSpace(taskID),
			WorkspaceID:   sql.NullString{String: strings.TrimSpace(workspaceID), Valid: true},
			CreatedByKind: string(taskpkg.ActorKindAgentSession),
			CreatedByRef:  strings.TrimSpace(sessionID),
		},
	)
	if err != nil {
		return false, fmt.Errorf("store: verify completion created task claim %q: %w", taskID, err)
	}
	return found, nil
}

// FailRunLease marks one claimed run failed after token verification.
