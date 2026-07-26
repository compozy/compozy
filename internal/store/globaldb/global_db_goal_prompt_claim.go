package globaldb

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// ClaimPreparedWorkPrompt is the work-effect linearization: queue claim, token digest, and open turn commit together.
func (g *GoalRepo) ClaimPreparedWorkPrompt(
	ctx context.Context,
	req goal.ClaimPreparedWorkPromptRequest,
) (goal.ClaimedPrompt, error) {
	if err := g.checkReady(ctx, "claim prepared Goal work prompt"); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := validateClaimPreparedWorkRequest(req); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	now := g.now().UTC()
	var claimed goal.ClaimedPrompt
	err := g.withTaskImmediateTransaction(ctx, "claim prepared Goal work prompt", func(exec taskSQLExecutor) error {
		var err error
		claimed, err = g.claimPreparedWorkPromptWithExecutor(ctx, exec, req, now)
		return err
	})
	if err != nil {
		return claimed, err
	}
	return claimed, nil
}

func (g *GoalRepo) claimPreparedWorkPromptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.ClaimPreparedWorkPromptRequest,
	now time.Time,
) (goal.ClaimedPrompt, error) {
	checkpoint, row, err := validatePreparedGoalClaim(
		ctx,
		exec,
		req.Key,
		req.ExpectedControlEpoch,
		req.BindingEpoch,
		req.TaskRunID,
		req.QueueEntryID,
		req.PromptID,
		req.SessionID,
		req.BindingHandle,
		[]string{goalPromptKindWork, goalPromptKindContinuation},
		req.BudgetDecision,
	)
	if err != nil {
		return goal.ClaimedPrompt{}, err
	}
	turn := checkpoint.TurnsUsed + 1
	if err := validateGoalDecisionGrant(checkpoint, req.BudgetDecision, turn, true); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	dispatchToken, tokenHash, err := newGoalDispatchToken()
	if err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := claimGoalQueueRow(
		ctx, exec, &row, req.ExpectedControlEpoch,
		req.UsageBaseTokens, req.UsageBaseReported, tokenHash, now,
	); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	seq, err := nextGoalTurnSequence(ctx, exec, req.Key.LoopRunID)
	if err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := insertOpenGoalTurn(ctx, exec, req, &row, seq, turn, now); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := startGoalWorkCheckpoint(ctx, exec, req, turn, now); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := projectGoalCheckpointCounts(
		ctx,
		exec,
		req.Key,
		goalStatusActive,
		turn,
		checkpoint.TurnLimit,
	); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := appendGoalTurnStartedEvent(ctx, exec, req, &row, turn, now); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	checkpoint.Phase = goalCheckpointPhasePrompting
	checkpoint.TurnsUsed = turn
	checkpoint.UpdatedAt = now
	return goal.ClaimedPrompt{Checkpoint: checkpoint, DispatchToken: dispatchToken}, nil
}

func insertOpenGoalTurn(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.ClaimPreparedWorkPromptRequest,
	row *goalPromptRow,
	sequence int64,
	turn int,
	now time.Time,
) error {
	err := sqlcgen.New(exec).InsertOpenGoalTurn(ctx, sqlcgen.InsertOpenGoalTurnParams{
		LoopRunID: string(req.Key.LoopRunID), Seq: sequence, Generation: int64(req.Key.Generation),
		NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex), Turn: int64(turn),
		SessionID: strings.TrimSpace(req.SessionID), BindingHandle: strings.TrimSpace(req.BindingHandle),
		BindingEpoch: req.BindingEpoch, PromptID: strings.TrimSpace(req.PromptID),
		PromptAttempt: int64(row.promptAttempt), UsageBaseTokens: req.UsageBaseTokens,
		ActorKind: strings.TrimSpace(req.ActorKind), ActorID: strings.TrimSpace(req.ActorID),
		StartedAt: store.FormatTimestamp(now),
	})
	if err != nil {
		return fmt.Errorf("store: insert open Goal turn: %w", err)
	}
	return nil
}

func startGoalWorkCheckpoint(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.ClaimPreparedWorkPromptRequest,
	turn int,
	now time.Time,
) error {
	affected, err := sqlcgen.New(exec).StartGoalWorkCheckpoint(ctx, sqlcgen.StartGoalWorkCheckpointParams{
		TurnsUsed: int64(turn), UpdatedAt: store.FormatTimestamp(now), LoopRunID: string(req.Key.LoopRunID),
		Generation: int64(req.Key.Generation), NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
		ControlEpoch: req.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&req.BindingEpoch),
		TaskRunID: goalNullableString(req.TaskRunID), QueueEntryID: goalNullableString(req.QueueEntryID),
		PromptID: goalNullableString(req.PromptID),
	})
	if err != nil {
		return fmt.Errorf("store: start Goal work checkpoint: %w", err)
	}
	return requireGoalAffectedCount(affected, "start Goal work checkpoint")
}

func appendGoalTurnStartedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.ClaimPreparedWorkPromptRequest,
	row *goalPromptRow,
	turn int,
	now time.Time,
) error {
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		req.Key.LoopRunID,
		req.Key.WorkspaceID,
		loopRunEventGoalTurnStarted,
		map[string]any{
			loopRunEventPayloadKeyGeneration: req.Key.Generation,
			loopRunEventPayloadKeyNodeID:     string(req.Key.NodeID),
			loopRunEventPayloadKeyItemIndex:  req.Key.ItemIndex,
			"turn":                           turn,
			columnPromptAttempt:              row.promptAttempt,
			loopRunEventPayloadKeyPromptID:   strings.TrimSpace(req.PromptID),
			columnSessionID:                  strings.TrimSpace(req.SessionID),
			"binding_handle":                 strings.TrimSpace(req.BindingHandle),
			columnBindingEpoch:               req.BindingEpoch,
			loopRunEventPayloadKeyActorKind:  strings.TrimSpace(req.ActorKind),
			loopRunEventPayloadKeyActorID:    strings.TrimSpace(req.ActorID),
		},
		now,
	)
}

// ClaimPreparedCompaction starts one compact operation without allocating a work turn.
func (g *GoalRepo) ClaimPreparedCompaction(
	ctx context.Context,
	req goal.ClaimPreparedCompactionRequest,
) (goal.ClaimedPrompt, error) {
	if err := g.checkReady(ctx, "claim prepared Goal compaction"); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := validateClaimPreparedCompactionRequest(req); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	now := g.now().UTC()
	var claimed goal.ClaimedPrompt
	err := g.withTaskImmediateTransaction(ctx, "claim prepared Goal compaction", func(exec taskSQLExecutor) error {
		var claimErr error
		claimed, claimErr = claimPreparedGoalCompactionWithExecutor(ctx, exec, req, now)
		return claimErr
	})
	if err != nil {
		return claimed, err
	}
	return claimed, nil
}

func claimPreparedGoalCompactionWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.ClaimPreparedCompactionRequest,
	now time.Time,
) (goal.ClaimedPrompt, error) {
	checkpoint, row, err := validatePreparedGoalClaim(
		ctx,
		exec,
		req.Key,
		req.ExpectedControlEpoch,
		req.BindingEpoch,
		req.TaskRunID,
		req.QueueEntryID,
		req.PromptID,
		req.SessionID,
		req.BindingHandle,
		[]string{goalPromptKindCompact},
		req.BudgetDecision,
	)
	if err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := validateGoalDecisionGrant(
		checkpoint, req.BudgetDecision, checkpoint.TurnsUsed, false,
	); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if !goalUsageSequenceMatches(checkpoint.UsageSequence, req.UsageSequence) {
		return goal.ClaimedPrompt{}, goalControlStaleError("Goal compaction usage sequence changed")
	}
	dispatchToken, tokenHash, err := newGoalDispatchToken()
	if err != nil {
		return goal.ClaimedPrompt{}, err
	}
	if err := claimGoalQueueRow(
		ctx, exec, &row, req.ExpectedControlEpoch,
		req.UsageBaseTokens, req.UsageBaseReported, tokenHash, now,
	); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	affected, err := sqlcgen.New(exec).StartGoalCompactionCheckpoint(ctx, sqlcgen.StartGoalCompactionCheckpointParams{
		UpdatedAt: store.FormatTimestamp(now), LoopRunID: string(req.Key.LoopRunID),
		Generation: int64(req.Key.Generation), NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
		ControlEpoch: req.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&req.BindingEpoch),
		TaskRunID: goalNullableString(req.TaskRunID), QueueEntryID: goalNullableString(req.QueueEntryID),
		PromptID: goalNullableString(req.PromptID),
	})
	if err != nil {
		return goal.ClaimedPrompt{}, fmt.Errorf("store: start Goal compaction checkpoint: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "start Goal compaction checkpoint"); err != nil {
		return goal.ClaimedPrompt{}, err
	}
	checkpoint.Phase = goalCheckpointPhaseCompacting
	checkpoint.UpdatedAt = now
	return goal.ClaimedPrompt{Checkpoint: checkpoint, DispatchToken: dispatchToken}, nil
}

func goalUsageSequenceMatches(left *int64, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func validatePreparedGoalClaim(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	controlEpoch int64,
	bindingEpoch int64,
	taskRunID string,
	queueEntryID string,
	promptID string,
	sessionID string,
	bindingHandle string,
	allowedKinds []string,
	decision goal.BudgetDecision,
) (goal.Checkpoint, goalPromptRow, error) {
	if err := validateGoalRunWorkspace(ctx, exec, key); err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, key)
	if err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	if !preparedGoalCheckpointOwnerMatches(
		checkpoint,
		controlEpoch,
		bindingEpoch,
		taskRunID,
		queueEntryID,
		promptID,
		sessionID,
		bindingHandle,
	) {
		return goal.Checkpoint{}, goalPromptRow{}, goalControlStaleError("prepared Goal checkpoint owner changed")
	}
	if !goalPromptKindAllowed(checkpoint.PromptKind, allowedKinds) {
		return goal.Checkpoint{}, goalPromptRow{}, goalControlStaleError("prepared Goal prompt kind changed")
	}
	if err := validateActiveGoalPromptBinding(
		ctx,
		exec,
		key,
		bindingHandle,
		bindingEpoch,
		sessionID,
	); err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	if err := validateGoalBudgetDecision(ctx, exec, key.LoopRunID, decision); err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	row, err := loadGoalPromptRow(ctx, exec, key.LoopRunID, promptID)
	if err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	if err := requireGoalPromptOwner(&row, key, taskRunID, queueEntryID, bindingEpoch); err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	if err := validatePreparedGoalFence(checkpoint, &row, decision); err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	if !preparedGoalQueueRowClaimable(&row, checkpoint.PromptKind, controlEpoch) {
		return goal.Checkpoint{}, goalPromptRow{}, goalPromptFencedError("prepared Goal queue row is not claimable")
	}
	return checkpoint, row, nil
}

func preparedGoalCheckpointOwnerMatches(
	checkpoint goal.Checkpoint,
	controlEpoch int64,
	bindingEpoch int64,
	taskRunID string,
	queueEntryID string,
	promptID string,
	sessionID string,
	bindingHandle string,
) bool {
	return checkpoint.ControlEpoch == controlEpoch &&
		checkpoint.BindingEpoch == bindingEpoch &&
		checkpoint.Phase == goalCheckpointPhaseQueued &&
		checkpoint.Status == goalStatusActive &&
		checkpoint.TaskRunID == strings.TrimSpace(taskRunID) &&
		checkpoint.QueueEntryID == strings.TrimSpace(queueEntryID) &&
		checkpoint.PromptID == strings.TrimSpace(promptID) &&
		checkpoint.SessionID == strings.TrimSpace(sessionID) &&
		checkpoint.BindingHandle == strings.TrimSpace(bindingHandle)
}

func goalPromptKindAllowed(promptKind string, allowedKinds []string) bool {
	return slices.Contains(allowedKinds, promptKind)
}

func preparedGoalQueueRowClaimable(row *goalPromptRow, promptKind string, controlEpoch int64) bool {
	return row.status == store.SessionInputQueueStatusQueued &&
		row.terminalAt == nil &&
		row.dispatchable == 0 &&
		row.promptKind.Valid &&
		row.promptKind.String == promptKind &&
		row.ownerEpoch.Valid &&
		row.ownerEpoch.Int64 == controlEpoch
}

func validatePreparedGoalFence(
	checkpoint goal.Checkpoint,
	row *goalPromptRow,
	decision goal.BudgetDecision,
) error {
	if !row.fenceKind.Valid {
		return nil
	}
	if row.fenceKind.String != string(looppkg.ActionPromptOutcomeBudgetFenced) ||
		!row.fenceDisposition.Valid ||
		row.fenceDisposition.String != string(looppkg.ActionDispositionNeedsApproval) ||
		!row.fenceReason.Valid {
		return goalPromptFencedError("prepared Goal prompt still has a control or terminal fence")
	}
	grant := checkpoint.ControlGrant
	if !decision.Allowed || decision.GrantID < 1 || grant == nil || grant.Consumed ||
		grant.Kind != goal.ControlGrantBudget || grant.ID != decision.GrantID ||
		grant.Cause != decision.Cause || row.fenceReason.String != string(decision.Cause) {
		return goalPromptFencedError("prepared Goal budget fence lacks its matching grant")
	}
	return nil
}

func claimGoalQueueRow(
	ctx context.Context,
	exec taskSQLExecutor,
	row *goalPromptRow,
	controlEpoch int64,
	usageBase int64,
	usageBaseReported bool,
	tokenHash string,
	now time.Time,
) error {
	baseTokens, err := goalSQLNullInt64(goalReportedTokens(usageBase, usageBaseReported))
	if err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).ClaimGoalQueueRow(ctx, sqlcgen.ClaimGoalQueueRowParams{
		ActivatedAt: store.FormatTimestamp(now), DispatchStartedAt: store.FormatTimestamp(now),
		OperationUsageBaseTokens: baseTokens, DispatchTokenHash: goalNullableString(tokenHash),
		UpdatedAt: store.FormatTimestamp(now), ID: row.id, OwnerKind: goalNullableString(goalPromptOwnerKind),
		OwnerEpoch: goalNullableInt64(&controlEpoch),
	})
	if err != nil {
		return fmt.Errorf("store: claim prepared Goal queue row: %w", err)
	}
	return requireGoalAffectedCount(affected, "claim prepared Goal queue row")
}

func nextGoalTurnSequence(ctx context.Context, exec taskSQLExecutor, runID looppkg.RunID) (int64, error) {
	seq, err := sqlcgen.New(exec).GetNextGoalTurnSequence(ctx, string(runID))
	if err != nil {
		return 0, fmt.Errorf("store: allocate Goal turn sequence: %w", err)
	}
	return seq, nil
}
