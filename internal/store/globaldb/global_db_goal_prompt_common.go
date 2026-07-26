package globaldb

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const goalPromptOwnerKind = "goal"

var _ goal.TurnStore = (*GoalRepo)(nil)

type goalPromptRow struct {
	id                     string
	sessionID              string
	status                 string
	text                   string
	taskRunID              string
	runGeneration          sql.NullInt64
	loopRunID              sql.NullString
	ownerKind              sql.NullString
	ownerEpoch             sql.NullInt64
	bindingEpoch           sql.NullInt64
	promptID               sql.NullString
	promptKind             sql.NullString
	promptAttempt          int
	dispatchable           int
	operationUsageBase     sql.NullInt64
	dispatchTokenHash      sql.NullString
	fenceKind              sql.NullString
	fenceDisposition       sql.NullString
	fenceReason            sql.NullString
	terminalEventStart     sql.NullInt64
	terminalEventEnd       sql.NullInt64
	terminalKind           sql.NullString
	terminalStopReason     sql.NullString
	terminalDisposition    sql.NullString
	terminalReason         sql.NullString
	terminalTokensReported int
	terminalTokensUsed     sql.NullInt64
	terminalAt             any
}

func loadGoalPromptRow(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	promptID string,
) (goalPromptRow, error) {
	generated, err := sqlcgen.New(exec).GetGoalPromptRow(ctx, sqlcgen.GetGoalPromptRowParams{
		LoopRunID: goalNullableString(string(runID)), PromptID: goalNullableString(promptID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goalPromptRow{}, fmt.Errorf("%w: Goal prompt %s", goal.ErrTurnNotFound, promptID)
	}
	if err != nil {
		return goalPromptRow{}, fmt.Errorf("store: scan Goal prompt row: %w", err)
	}
	return goalPromptRowFromGenerated(&generated), nil
}

func findGoalPromptRow(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	promptID string,
) (goalPromptRow, bool, error) {
	row, err := loadGoalPromptRow(ctx, exec, runID, promptID)
	if errors.Is(err, goal.ErrTurnNotFound) {
		return goalPromptRow{}, false, nil
	}
	return row, err == nil, err
}

func validateGoalPromptResult(result looppkg.ActionPromptResult) error {
	if strings.TrimSpace(result.PromptID) == "" || result.TokensUsed < 0 ||
		result.EventStartSeq < 0 || result.EventEndSeq < result.EventStartSeq {
		return fmt.Errorf("%w: Goal prompt terminal evidence is invalid", looppkg.ErrValidation)
	}
	if err := validateStoredGoalPromptOutcome(result); err != nil {
		return err
	}
	if !result.TokensReported && result.TokensUsed != 0 {
		return fmt.Errorf("%w: unreported Goal prompt tokens must remain absent", looppkg.ErrValidation)
	}
	return nil
}

func validateStoredGoalPromptOutcome(result looppkg.ActionPromptResult) error {
	switch result.Outcome {
	case looppkg.ActionPromptOutcomeCompleted:
		return validateStoredCompletedPrompt(result)
	case looppkg.ActionPromptOutcomeInvalidResult:
		return validateStoredReasonPrompt(
			result,
			looppkg.ReasonCodeGoalStopReasonInvalid,
			"invalid Goal prompt",
		)
	case looppkg.ActionPromptOutcomeFailed:
		return validateStoredReasonPrompt(
			result,
			looppkg.ReasonCodeGoalPromptRequestFailed,
			"failed Goal prompt",
		)
	case looppkg.ActionPromptOutcomeAmbiguous:
		return validateStoredAmbiguousPrompt(result)
	case looppkg.ActionPromptOutcomeBudgetFenced, looppkg.ActionPromptOutcomeControlFenced:
		return validateStoredFencedPrompt(result)
	case looppkg.ActionPromptOutcomeRejectedBeforeSubmit:
		return validateStoredRejectedPrompt(result)
	default:
		return fmt.Errorf("%w: Goal prompt outcome %q is invalid", looppkg.ErrValidation, result.Outcome)
	}
}

func validateStoredCompletedPrompt(result looppkg.ActionPromptResult) error {
	if !result.StopReason.Valid() || result.ReasonCode != "" {
		return fmt.Errorf("%w: completed Goal prompt requires one stop reason", looppkg.ErrValidation)
	}
	return nil
}

func validateStoredReasonPrompt(
	result looppkg.ActionPromptResult,
	want looppkg.ReasonCode,
	label string,
) error {
	if result.StopReason != "" || result.ReasonCode != want {
		return fmt.Errorf("%w: %s shape changed", looppkg.ErrValidation, label)
	}
	return nil
}

func validateStoredAmbiguousPrompt(result looppkg.ActionPromptResult) error {
	validCause := result.ReasonCode == looppkg.ReasonCodeGoalRecoveryAmbiguous ||
		result.ReasonCode == looppkg.ReasonCodeGoalControlRevokedInFlight
	if result.StopReason != "" || !validCause {
		return fmt.Errorf("%w: ambiguous Goal prompt shape changed", looppkg.ErrValidation)
	}
	return nil
}

func validateStoredFencedPrompt(result looppkg.ActionPromptResult) error {
	if result.StopReason != "" || !result.FenceDisposition.Valid() || result.ReasonCode == "" {
		return fmt.Errorf("%w: fenced Goal prompt shape changed", looppkg.ErrValidation)
	}
	return nil
}

func validateStoredRejectedPrompt(result looppkg.ActionPromptResult) error {
	if result.StopReason != "" || result.ReasonCode == "" {
		return fmt.Errorf("%w: rejected Goal prompt shape changed", looppkg.ErrValidation)
	}
	return nil
}

func newGoalDispatchToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("store: create Goal dispatch token: %w", err)
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

func hashGoalDispatchToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func requireGoalPromptOwner(
	row *goalPromptRow,
	key goal.TurnKey,
	taskRunID string,
	queueEntryID string,
	bindingEpoch int64,
) error {
	if row.id != strings.TrimSpace(queueEntryID) || row.taskRunID != strings.TrimSpace(taskRunID) ||
		!row.loopRunID.Valid || row.loopRunID.String != string(key.LoopRunID) ||
		!row.ownerKind.Valid || row.ownerKind.String != goalPromptOwnerKind ||
		!row.ownerEpoch.Valid || row.ownerEpoch.Int64 < 1 ||
		!row.bindingEpoch.Valid || row.bindingEpoch.Int64 != bindingEpoch ||
		!row.runGeneration.Valid || int(row.runGeneration.Int64) != key.Generation {
		return goalControlStaleError("Goal prompt owner identity changed")
	}
	return nil
}
