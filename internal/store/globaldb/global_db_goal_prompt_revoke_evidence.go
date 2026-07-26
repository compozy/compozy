package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func revokeGoalPromptEvidence(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	row *goalPromptRow,
	request goal.RevokePromptRequest,
) (bool, error) {
	if checkpoint.Phase == goalCheckpointPhaseQueued {
		return false, terminalizePreparedGoalRevocation(ctx, exec, row, request)
	}
	if row.terminalAt == nil {
		if err := terminalizeClaimedGoalRevocation(ctx, exec, row, request); err != nil {
			return false, err
		}
		var err error
		*row, err = loadGoalPromptRow(ctx, exec, request.Key.LoopRunID, request.PromptID)
		if err != nil {
			return false, err
		}
	}
	if checkpoint.PromptKind == goalPromptKindCompact {
		return false, nil
	}
	if err := terminalizeRevokedGoalTurn(ctx, exec, row, request); err != nil {
		return false, err
	}
	return true, nil
}

func terminalizePreparedGoalRevocation(
	ctx context.Context,
	exec taskSQLExecutor,
	row *goalPromptRow,
	request goal.RevokePromptRequest,
) error {
	affected, err := sqlcgen.New(exec).
		TerminalizePreparedGoalRevocation(ctx, sqlcgen.TerminalizePreparedGoalRevocationParams{
			TerminalDisposition: goalNullableString(string(request.Disposition)),
			TerminalReasonCode:  goalNullableString(string(looppkg.ReasonCodeGoalControlRevokedInFlight)),
			TerminalAt:          store.FormatTimestamp(request.RevokedAt),
			CanceledAt:          store.FormatTimestamp(request.RevokedAt),
			UpdatedAt:           store.FormatTimestamp(request.RevokedAt),
			ID:                  row.id,
			LoopRunID:           goalNullableString(string(request.Key.LoopRunID)),
			TaskRunID:           strings.TrimSpace(request.TaskRunID),
			PromptID:            goalNullableString(request.PromptID),
			OwnerKind:           goalNullableString(goalPromptOwnerKind),
			OwnerEpoch:          goalNullableInt64(&request.ExpectedControlEpoch),
			BindingEpoch:        goalNullableInt64(&request.ExpectedBindingEpoch),
		})
	if err != nil {
		return fmt.Errorf("store: revoke prepared Goal prompt: %w", err)
	}
	return requireGoalAffectedCount(affected, "revoke prepared Goal prompt")
}

func terminalizeClaimedGoalRevocation(
	ctx context.Context,
	exec taskSQLExecutor,
	row *goalPromptRow,
	request goal.RevokePromptRequest,
) error {
	affected, err := sqlcgen.New(exec).
		TerminalizeClaimedGoalRevocation(ctx, sqlcgen.TerminalizeClaimedGoalRevocationParams{
			TerminalReasonCode: goalNullableString(string(looppkg.ReasonCodeGoalControlRevokedInFlight)),
			TerminalAt:         store.FormatTimestamp(request.RevokedAt),
			FailedAt:           store.FormatTimestamp(request.RevokedAt),
			FailureSummary:     string(looppkg.ReasonCodeGoalControlRevokedInFlight),
			UpdatedAt:          store.FormatTimestamp(request.RevokedAt),
			ID:                 row.id,
			LoopRunID:          goalNullableString(string(request.Key.LoopRunID)),
			TaskRunID:          strings.TrimSpace(request.TaskRunID),
			PromptID:           goalNullableString(request.PromptID),
			OwnerKind:          goalNullableString(goalPromptOwnerKind),
			OwnerEpoch:         goalNullableInt64(&request.ExpectedControlEpoch),
			BindingEpoch:       goalNullableInt64(&request.ExpectedBindingEpoch),
		})
	if err != nil {
		return fmt.Errorf("store: revoke claimed Goal prompt: %w", err)
	}
	return requireGoalAffectedCount(affected, "revoke claimed Goal prompt")
}

func terminalizeRevokedGoalTurn(
	ctx context.Context,
	exec taskSQLExecutor,
	row *goalPromptRow,
	request goal.RevokePromptRequest,
) error {
	if !row.terminalKind.Valid || row.terminalAt == nil {
		return fmt.Errorf("%w: revoked Goal prompt has no terminal evidence", looppkg.ErrTransitionConflict)
	}
	if !revokedGoalTurnOutcomeValid(row.terminalKind.String) {
		return fmt.Errorf(
			"%w: revoked Goal prompt terminal %q cannot settle a turn",
			looppkg.ErrTransitionConflict,
			row.terminalKind.String,
		)
	}
	endedAt, err := parseGoalTimestampValue(row.terminalAt, "revoked prompt terminal_at")
	if err != nil {
		return err
	}
	tokensUsed, err := goalSQLNullInt64(
		goalReportedTokens(row.terminalTokensUsed.Int64, row.terminalTokensReported != 0),
	)
	if err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).TerminalizeRevokedGoalTurn(ctx, sqlcgen.TerminalizeRevokedGoalTurnParams{
		ResultStatus: goalNullableString(
			row.terminalKind.String,
		),
		StopReason:   goalNullableString(row.terminalStopReason.String),
		ReasonCode:   goalNullableString(row.terminalReason.String),
		TokensUsed:   tokensUsed,
		EndedAt:      store.FormatTimestamp(endedAt),
		LoopRunID:    string(request.Key.LoopRunID),
		Generation:   int64(request.Key.Generation),
		NodeID:       string(request.Key.NodeID),
		ItemIndex:    int64(request.Key.ItemIndex),
		PromptID:     strings.TrimSpace(request.PromptID),
		BindingEpoch: request.ExpectedBindingEpoch,
	})
	if err != nil {
		return fmt.Errorf("store: terminalize revoked Goal turn: %w", err)
	}
	return requireGoalAffectedCount(affected, "terminalize revoked Goal turn")
}

func revokedGoalTurnOutcomeValid(outcome string) bool {
	switch looppkg.ActionPromptOutcome(outcome) {
	case looppkg.ActionPromptOutcomeCompleted,
		looppkg.ActionPromptOutcomeInvalidResult,
		looppkg.ActionPromptOutcomeFailed,
		looppkg.ActionPromptOutcomeAmbiguous:
		return true
	default:
		return false
	}
}
