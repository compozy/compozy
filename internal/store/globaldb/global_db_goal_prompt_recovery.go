package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.PromptTerminalRecoveryStore = (*GoalRepo)(nil)

// RecoverGoalPromptTerminal records exact session-event proof without exposing the raw dispatch token.
func (g *GoalRepo) RecoverGoalPromptTerminal(
	ctx context.Context,
	req goal.RecoverPromptTerminalRequest,
) error {
	if err := g.checkReady(ctx, "recover Goal prompt terminal"); err != nil {
		return err
	}
	if err := validateRecoveredGoalPromptRequest(req); err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "recover Goal prompt terminal", func(exec taskSQLExecutor) error {
		if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
			return err
		}
		checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
		if err != nil {
			return err
		}
		row, err := loadGoalPromptRow(ctx, exec, req.Key.LoopRunID, req.PromptID)
		if err != nil {
			return err
		}
		if err := validateGoalLivePromptOwner(
			ctx,
			exec,
			checkpoint,
			&row,
			req.Key,
			req.ExpectedControlEpoch,
			req.ExpectedBindingEpoch,
			req.TaskRunID,
			req.QueueEntryID,
			req.PromptID,
			req.SessionID,
			req.BindingHandle,
			true,
		); err != nil {
			return err
		}
		if row.terminalAt != nil {
			if goalPromptTerminalMatches(&row, req.Result) {
				return nil
			}
			return goalControlStaleError("recovered Goal prompt already has a different terminal")
		}
		if !row.dispatchTokenHash.Valid {
			return goalControlStaleError("recovered Goal prompt has no committed dispatch token digest")
		}
		finalize := goal.FinalizePromptRequest{
			Key: req.Key, ExpectedControlEpoch: req.ExpectedControlEpoch,
			ExpectedBindingEpoch: req.ExpectedBindingEpoch, TaskRunID: req.TaskRunID,
			QueueEntryID: req.QueueEntryID, PromptID: req.PromptID, SessionID: req.SessionID,
			BindingHandle: req.BindingHandle, Result: req.Result, TerminalAt: req.TerminalAt,
		}
		status := goalPromptTerminalQueueStatus(req.Result.Outcome)
		if err := persistRecoveredGoalQueueTerminal(
			ctx,
			exec,
			finalize,
			&row,
			req.Result.FenceDisposition,
			status,
		); err != nil {
			return err
		}
		phase := goalTerminalCheckpointPhase(checkpoint.PromptKind, req.Result)
		return advanceGoalTerminalCheckpoint(ctx, exec, finalize, checkpoint, phase)
	})
}

func persistRecoveredGoalQueueTerminal(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.FinalizePromptRequest,
	row *goalPromptRow,
	disposition looppkg.ActionDisposition,
	status string,
) error {
	terminalTokens := sql.NullInt64{}
	if req.Result.TokensReported {
		terminalTokens = sql.NullInt64{Int64: req.Result.TokensUsed, Valid: true}
	}
	affected, err := sqlcgen.New(exec).RecoverGoalQueueTerminal(ctx, sqlcgen.RecoverGoalQueueTerminalParams{
		Status:                 status,
		TerminalEventStartSeq:  goalNullablePositiveInt64(req.Result.EventStartSeq),
		TerminalEventEndSeq:    goalNullablePositiveInt64(req.Result.EventEndSeq),
		TerminalKind:           goalNullableString(string(req.Result.Outcome)),
		TerminalStopReason:     goalNullableString(string(req.Result.StopReason)),
		TerminalDisposition:    goalNullableString(string(disposition)),
		TerminalReasonCode:     goalNullableString(string(req.Result.ReasonCode)),
		TerminalTokensReported: int64(boolToInt(req.Result.TokensReported)),
		TerminalTokensUsed:     terminalTokens,
		TerminalAt:             store.FormatTimestamp(req.TerminalAt),
		ID:                     strings.TrimSpace(req.QueueEntryID),
		LoopRunID:              goalNullableString(string(req.Key.LoopRunID)),
		TaskRunID:              strings.TrimSpace(req.TaskRunID),
		PromptID:               goalNullableString(req.PromptID),
		OwnerKind:              goalNullableString(goalPromptOwnerKind),
		OwnerEpoch: goalNullableInt64(
			&req.ExpectedControlEpoch,
		),
		BindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
		SessionID: strings.TrimSpace(
			req.SessionID,
		),
		DispatchTokenHash: goalNullableString(row.dispatchTokenHash.String),
	})
	if err != nil {
		return fmt.Errorf("store: recover Goal queue terminal: %w", err)
	}
	if affected != 1 {
		return goalControlStaleError("recover Goal queue terminal lost compare-and-swap")
	}
	return nil
}

func validateRecoveredGoalPromptRequest(req goal.RecoverPromptTerminalRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if req.ExpectedControlEpoch < 1 || req.ExpectedBindingEpoch < 1 ||
		strings.TrimSpace(req.TaskRunID) == "" || strings.TrimSpace(req.QueueEntryID) == "" ||
		strings.TrimSpace(req.PromptID) == "" || strings.TrimSpace(req.SessionID) == "" ||
		strings.TrimSpace(req.BindingHandle) == "" || req.TerminalAt.IsZero() {
		return fmt.Errorf("%w: recovered Goal prompt identity is incomplete", looppkg.ErrValidation)
	}
	if err := validateGoalPromptResult(req.Result); err != nil {
		return err
	}
	if strings.TrimSpace(req.Result.PromptID) != strings.TrimSpace(req.PromptID) {
		return fmt.Errorf("%w: recovered Goal prompt correlation changed", looppkg.ErrValidation)
	}
	return nil
}
