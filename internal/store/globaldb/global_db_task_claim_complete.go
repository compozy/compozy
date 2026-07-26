package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

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
		return updateLoopNodeOutputStatusWithExecutor(
			ctx,
			exec,
			current,
			strings.TrimSpace(current.LoopRunID),
			looppkg.GenerationOutputStatusControlPending,
			"",
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

type loopNodeRunMetadata struct {
	Generation int    `json:"generation"`
	NodeID     string `json:"node_id"`
	ItemIndex  int    `json:"item_index"`
}

const (
	loopNodeOutcomeFailure  = "failure"
	loopNodeOutputFailed    = "failed"
	loopNodeOutputSucceeded = "succeeded"
)

func recordLoopNodeTerminalWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	outcome string,
	outputRef string,
	resultPayload json.RawMessage,
	terminalAt time.Time,
) error {
	loopRunID := strings.TrimSpace(run.LoopRunID)
	if !run.IsLoopWorker() {
		return nil
	}
	status := loopNodeOutputSucceeded
	if outcome == loopNodeOutcomeFailure {
		status = loopNodeOutputFailed
	}
	if err := updateLoopNodeOutputStatusWithExecutor(ctx, exec, run, loopRunID, status, outputRef); err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).UpdateLoopRunNodeTerminal(ctx, sqlcgen.UpdateLoopRunNodeTerminalParams{
		TerminalAt: store.FormatTimestamp(terminalAt), ID: loopRunID,
	})
	if err != nil {
		return fmt.Errorf("store: record loop run %q node terminal progress: %w", loopRunID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: loop run %q: %w", loopRunID, looppkg.ErrRunNotFound)
	}
	tokensUsed, err := refreshLoopTokensUsedWithExecutor(ctx, exec, loopRunID)
	if err != nil {
		return err
	}
	return appendLoopNodeTerminalEventsWithExecutor(
		ctx,
		exec,
		run,
		outcome,
		outputRef,
		normalizeTaskJSON(resultPayload),
		tokensUsed,
		terminalAt,
	)
}

func updateLoopNodeOutputStatusWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	loopRunID string,
	status string,
	outputRef string,
) error {
	// Generation advance is atomic with task-run completion here; changing that boundary requires an ADR.
	affected, err := sqlcgen.New(exec).UpdateLoopGenerationOutputForTaskRun(
		ctx,
		sqlcgen.UpdateLoopGenerationOutputForTaskRunParams{
			Status: status, OutputRef: strings.TrimSpace(outputRef),
			TaskRunID: nullableTaskString(run.ID), LoopRunID: loopRunID,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"store: update loop run %q node output for task run %q: %w",
			loopRunID,
			run.ID,
			err,
		)
	}
	if affected > 0 {
		return nil
	}
	metadata, ok, err := loopNodeMetadataFromTaskRun(run.Metadata)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	err = sqlcgen.New(exec).UpsertLoopGenerationOutputForTaskRun(
		ctx,
		sqlcgen.UpsertLoopGenerationOutputForTaskRunParams{
			LoopRunID: loopRunID, Generation: int64(metadata.Generation), NodeID: metadata.NodeID,
			ItemIndex: int64(metadata.ItemIndex), Status: status,
			OutputRef: nullableTaskString(outputRef), TaskRunID: nullableTaskString(run.ID),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"store: insert loop run %q node output for task run %q: %w",
			loopRunID,
			run.ID,
			err,
		)
	}
	return nil
}

func loopNodeMetadataFromTaskRun(raw json.RawMessage) (loopNodeRunMetadata, bool, error) {
	if len(raw) == 0 {
		return loopNodeRunMetadata{}, false, nil
	}
	var metadata loopNodeRunMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return loopNodeRunMetadata{}, false, fmt.Errorf(
			"store: decode loop node task run metadata: %w",
			err,
		)
	}
	metadata.NodeID = strings.TrimSpace(metadata.NodeID)
	if metadata.Generation <= 0 || metadata.NodeID == "" || metadata.ItemIndex < 0 {
		return loopNodeRunMetadata{}, false, nil
	}
	return metadata, true, nil
}

func loopFailureOutputRef(failure taskpkg.RunFailure) string {
	if outputRef, ok := looppkg.ActionFailureOutputRefFromMetadata(failure.Metadata); ok {
		return outputRef
	}

	type reasonEnvelope struct {
		ReasonCode string `json:"reason_code"`
		Code       string `json:"code"`
	}
	var envelope reasonEnvelope
	if len(failure.Metadata) > 0 {
		if err := json.Unmarshal(failure.Metadata, &envelope); err == nil {
			if strings.TrimSpace(envelope.ReasonCode) != "" {
				return strings.TrimSpace(envelope.ReasonCode)
			}
			if strings.TrimSpace(envelope.Code) != "" {
				return strings.TrimSpace(envelope.Code)
			}
		}
	}
	for _, code := range []string{"dependency_missing", "credential_missing", "resource_unreachable"} {
		if strings.Contains(failure.Error, code) {
			return code
		}
	}
	return ""
}

// FailRunLease marks one claimed run failed after token verification.
