package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// PrepareGoalPrompt inserts one idempotent non-dispatchable Goal queue ticket.
func (g *GoalRepo) PrepareGoalPrompt(
	ctx context.Context,
	req goal.PreparePromptRequest,
) (goal.PromptTicket, error) {
	if err := g.checkReady(ctx, "prepare Goal prompt"); err != nil {
		return goal.PromptTicket{}, err
	}
	if err := validatePrepareGoalPromptRequest(req); err != nil {
		return goal.PromptTicket{}, err
	}
	var ticket goal.PromptTicket
	err := g.withTaskImmediateTransaction(ctx, "prepare Goal prompt", func(exec taskSQLExecutor) error {
		var err error
		ticket, err = prepareGoalPromptWithExecutor(ctx, exec, req)
		return err
	})
	if err != nil {
		return goal.PromptTicket{}, err
	}
	return ticket, nil
}

func prepareGoalPromptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.PreparePromptRequest,
) (goal.PromptTicket, error) {
	if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
		return goal.PromptTicket{}, err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return goal.PromptTicket{}, err
	}
	if checkpoint.ControlEpoch != req.ExpectedControlEpoch || checkpoint.Status != goalStatusActive ||
		(checkpoint.Phase != goalCheckpointPhaseIdle && checkpoint.Phase != goalCheckpointPhasePreparing &&
			checkpoint.Phase != goalCheckpointPhaseQueued) {
		return goal.PromptTicket{}, goalControlStaleError("Goal prompt prepare checkpoint changed")
	}
	if !goalUsageSequenceMatches(checkpoint.UsageSequence, req.ContextUsageSequence) &&
		strings.TrimSpace(req.PromptKind) == goalPromptKindCompact {
		return goal.PromptTicket{}, goalControlStaleError("Goal compaction context usage changed")
	}
	if strings.TrimSpace(req.PromptKind) == goalPromptKindCompact &&
		checkpoint.CompactionBaselineUsed != nil &&
		!goalUsageSequenceMatches(checkpoint.CompactionBaselineUsed, req.ContextUsageUsed) {
		return goal.PromptTicket{}, goalControlStaleError("Goal compaction context baseline changed")
	}
	if err := validateGoalPromptReferences(ctx, exec, req); err != nil {
		return goal.PromptTicket{}, err
	}
	if err := validateActiveGoalPromptBinding(
		ctx,
		exec,
		req.Key,
		req.BindingHandle,
		req.BindingEpoch,
		req.SessionID,
	); err != nil {
		return goal.PromptTicket{}, err
	}
	existing, found, err := findGoalPromptRow(ctx, exec, req.Key.LoopRunID, req.PromptID)
	if err != nil {
		return goal.PromptTicket{}, err
	}
	if found {
		return existingPreparedGoalPromptTicket(&existing, req)
	}
	if checkpoint.Phase == goalCheckpointPhaseQueued {
		return goal.PromptTicket{}, goalControlStaleError(
			"Goal checkpoint already owns a different prepared prompt",
		)
	}
	preparedAt := req.PreparedAt.UTC()
	if err := insertPreparedGoalPrompt(ctx, exec, req, preparedAt); err != nil {
		return goal.PromptTicket{}, err
	}
	if err := checkpointPreparedGoalPrompt(ctx, exec, req, preparedAt); err != nil {
		return goal.PromptTicket{}, err
	}
	return goal.PromptTicket{
		QueueEntryID: strings.TrimSpace(req.QueueEntryID),
		PromptID:     strings.TrimSpace(req.PromptID),
	}, nil
}

func existingPreparedGoalPromptTicket(
	row *goalPromptRow,
	req goal.PreparePromptRequest,
) (goal.PromptTicket, error) {
	matches := row.status == store.SessionInputQueueStatusQueued &&
		row.terminalAt == nil &&
		row.dispatchable == 0 &&
		row.id == strings.TrimSpace(req.QueueEntryID) &&
		row.sessionID == strings.TrimSpace(req.SessionID) &&
		row.taskRunID == strings.TrimSpace(req.TaskRunID) &&
		row.text == req.Message &&
		row.bindingEpoch.Valid && row.bindingEpoch.Int64 == req.BindingEpoch &&
		row.ownerEpoch.Valid && row.ownerEpoch.Int64 == req.ExpectedControlEpoch &&
		row.promptKind.Valid && row.promptKind.String == req.PromptKind &&
		preparedGoalUsageBaseMatches(row.operationUsageBase, req) &&
		row.promptAttempt == req.PromptAttempt
	if !matches {
		return goal.PromptTicket{}, goalControlStaleError(
			"Goal prompt ID already has different prepared identity",
		)
	}
	return goal.PromptTicket{QueueEntryID: row.id, PromptID: row.promptID.String}, nil
}

func insertPreparedGoalPrompt(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.PreparePromptRequest,
	preparedAt time.Time,
) error {
	usageBase, err := goalSQLNullInt64(goalReportedTokens(req.UsageBaseTokens, req.UsageBaseReported))
	if err != nil {
		return err
	}
	generation := int64(req.Key.Generation)
	err = sqlcgen.New(exec).InsertPreparedGoalPromptQueue(ctx, sqlcgen.InsertPreparedGoalPromptQueueParams{
		ID: strings.TrimSpace(req.QueueEntryID), SessionID: strings.TrimSpace(req.SessionID),
		Mode: store.SessionInputQueueModeQueue, Text: req.Message, TaskRunID: strings.TrimSpace(req.TaskRunID),
		RunGeneration: goalNullableInt64(&generation), EnqueuedAt: store.FormatTimestamp(preparedAt),
		UpdatedAt: store.FormatTimestamp(preparedAt), LoopRunID: goalNullableString(string(req.Key.LoopRunID)),
		OwnerKind: goalNullableString(goalPromptOwnerKind), OwnerEpoch: goalNullableInt64(&req.ExpectedControlEpoch),
		BindingEpoch: goalNullableInt64(&req.BindingEpoch), PromptID: goalNullableString(req.PromptID),
		PromptKind: goalNullableString(req.PromptKind), PromptAttempt: int64(req.PromptAttempt),
		OperationUsageBaseTokens: usageBase,
	})
	if err != nil {
		return fmt.Errorf("store: insert prepared Goal prompt: %w", err)
	}
	return nil
}

func checkpointPreparedGoalPrompt(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.PreparePromptRequest,
	preparedAt time.Time,
) error {
	baseline, err := goalSQLNullInt64(nullableGoalInt64Value(req.ContextUsageUsed))
	if err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).CheckpointPreparedGoalPrompt(ctx, sqlcgen.CheckpointPreparedGoalPromptParams{
		TaskRunID: goalNullableString(req.TaskRunID), QueueEntryID: goalNullableString(req.QueueEntryID),
		PromptID: goalNullableString(req.PromptID), PromptKind: goalNullableString(req.PromptKind),
		PromptAttempt: int64(req.PromptAttempt), SessionID: goalNullableString(req.SessionID),
		BindingHandle: goalNullableString(req.BindingHandle), BindingEpoch: goalNullableInt64(&req.BindingEpoch),
		CompactionBaselineUsed: baseline, UpdatedAt: store.FormatTimestamp(preparedAt),
		LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
		NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex), ControlEpoch: req.ExpectedControlEpoch,
	})
	if err != nil {
		return fmt.Errorf("store: checkpoint prepared Goal prompt: %w", err)
	}
	return requireGoalAffectedCount(affected, "checkpoint prepared Goal prompt")
}

func validatePrepareGoalPromptRequest(req goal.PreparePromptRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	kind := strings.TrimSpace(req.PromptKind)
	if req.ExpectedControlEpoch < 1 || strings.TrimSpace(req.TaskRunID) == "" ||
		strings.TrimSpace(req.QueueEntryID) == "" || strings.TrimSpace(req.PromptID) == "" ||
		(kind != goalPromptKindWork && kind != goalPromptKindContinuation && kind != goalPromptKindCompact) ||
		req.PromptAttempt < 0 || strings.TrimSpace(req.SessionID) == "" ||
		strings.TrimSpace(req.BindingHandle) == "" || req.BindingEpoch < 1 ||
		req.UsageBaseTokens < 0 || (!req.UsageBaseReported && req.UsageBaseTokens != 0) ||
		strings.TrimSpace(req.Message) == "" || req.PreparedAt.IsZero() {
		return fmt.Errorf("%w: prepared Goal prompt identity is incomplete", looppkg.ErrValidation)
	}
	return validatePreparedGoalContextBaseline(kind, req.ContextUsageSequence, req.ContextUsageUsed)
}

func validatePreparedGoalContextBaseline(kind string, sequence *int64, used *int64) error {
	if (sequence == nil) != (used == nil) ||
		(sequence != nil && (*sequence < 0 || *used < 0)) ||
		(kind != goalPromptKindCompact && sequence != nil) {
		return fmt.Errorf("%w: prepared Goal context baseline is incomplete", looppkg.ErrValidation)
	}
	return nil
}

func preparedGoalUsageBaseMatches(base sql.NullInt64, req goal.PreparePromptRequest) bool {
	if !req.UsageBaseReported {
		return !base.Valid
	}
	return base.Valid && base.Int64 == req.UsageBaseTokens
}

func validateGoalPromptReferences(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.PreparePromptRequest,
) error {
	if _, err := sqlcgen.New(exec).ValidateGoalPromptTaskRun(ctx, sqlcgen.ValidateGoalPromptTaskRunParams{
		ID: strings.TrimSpace(req.TaskRunID), LoopRunID: goalNullableString(string(req.Key.LoopRunID)),
	}); err != nil {
		return fmt.Errorf("store: validate Goal prompt task owner: %w", err)
	}
	if _, err := sqlcgen.New(exec).ValidateGoalSessionWorkspace(ctx, sqlcgen.ValidateGoalSessionWorkspaceParams{
		ID: strings.TrimSpace(req.SessionID), WorkspaceID: string(req.Key.WorkspaceID),
	}); err != nil {
		return fmt.Errorf("store: validate Goal prompt session owner: %w", err)
	}
	return nil
}
