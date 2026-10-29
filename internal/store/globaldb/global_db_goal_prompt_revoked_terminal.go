package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// retainRevokedGoalTerminal attaches the exact provider event witness to a control-revoked
// receipt. The control outcome, turn, checkpoint, binding and run remain terminal.
func retainRevokedGoalTerminal(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.FinalizePromptRequest,
	row *goalPromptRow,
) error {
	if err := requireGoalPromptOwner(
		row,
		req.Key,
		req.TaskRunID,
		req.QueueEntryID,
		req.ExpectedBindingEpoch,
	); err != nil {
		return err
	}
	if row.ownerEpoch.Int64 != req.ExpectedControlEpoch || row.sessionID != req.SessionID ||
		row.promptID.String != req.PromptID || !row.dispatchTokenHash.Valid ||
		row.dispatchTokenHash.String != hashGoalDispatchToken(req.DispatchToken) ||
		req.Result.EventStartSeq < 1 || req.Result.EventEndSeq < req.Result.EventStartSeq {
		return goalControlStaleError("revoked Goal terminal identity or event evidence changed")
	}
	binding, err := getSessionBindingAttemptWithExecutor(ctx, exec, goal.BindingKey{
		WorkspaceID: req.Key.WorkspaceID, LoopRunID: req.Key.LoopRunID, Handle: req.BindingHandle,
	}, req.ExpectedBindingEpoch)
	if err != nil {
		return err
	}
	if binding.SessionID != req.SessionID || binding.State != goal.BindingStateClosed {
		return goalControlStaleError("revoked Goal terminal binding changed")
	}
	if row.terminalEventStart.Valid || row.terminalEventEnd.Valid {
		if revokedGoalTerminalMatches(row, req.Result) {
			return nil
		}
		return goalControlStaleError("revoked Goal prompt already has different terminal evidence")
	}
	tokens, err := goalSQLNullInt64(goalReportedTokens(req.Result.TokensUsed, req.Result.TokensReported))
	if err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).RetainRevokedGoalTerminal(ctx, sqlcgen.RetainRevokedGoalTerminalParams{
		EventStartSeq:  goalNullablePositiveInt64(req.Result.EventStartSeq),
		EventEndSeq:    goalNullablePositiveInt64(req.Result.EventEndSeq),
		StopReason:     goalNullableString(string(req.Result.StopReason)),
		TokensReported: int64(boolToInt(req.Result.TokensReported)), TokensUsed: tokens,
		UpdatedAt: store.FormatTimestamp(req.TerminalAt), ID: row.id, DispatchTokenHash: row.dispatchTokenHash,
	})
	if err != nil {
		return fmt.Errorf("store: retain revoked Goal terminal: %w", err)
	}
	return requireGoalAffectedCount(affected, "retain revoked Goal terminal")
}

func revokedGoalTerminalMatches(row *goalPromptRow, result loop.ActionPromptResult) bool {
	return goalOptionalPositiveInt64Matches(row.terminalEventStart, result.EventStartSeq) &&
		goalOptionalPositiveInt64Matches(row.terminalEventEnd, result.EventEndSeq) &&
		row.terminalStopReason.String == string(result.StopReason) &&
		(row.terminalTokensReported != 0) == result.TokensReported &&
		(!result.TokensReported || row.terminalTokensUsed.Int64 == result.TokensUsed)
}
