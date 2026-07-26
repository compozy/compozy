package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func goalPromptRowFromGenerated(row *sqlcgen.GetGoalPromptRowRow) goalPromptRow {
	var terminalAt any
	if row.TerminalAt.Valid {
		terminalAt = row.TerminalAt.Time.UTC()
	}
	return goalPromptRow{
		id: row.ID, sessionID: row.SessionID, status: row.Status, text: row.Text, taskRunID: row.TaskRunID,
		runGeneration: row.RunGeneration, loopRunID: row.LoopRunID, ownerKind: row.OwnerKind,
		ownerEpoch: row.OwnerEpoch, bindingEpoch: row.BindingEpoch, promptID: row.PromptID,
		promptKind: row.PromptKind, promptAttempt: int(row.PromptAttempt), dispatchable: int(row.Dispatchable),
		operationUsageBase: row.OperationUsageBaseTokens, dispatchTokenHash: row.DispatchTokenHash,
		fenceKind: row.FenceKind, fenceDisposition: row.FenceDisposition, fenceReason: row.FenceReasonCode,
		terminalEventStart: row.TerminalEventStartSeq, terminalEventEnd: row.TerminalEventEndSeq,
		terminalKind: row.TerminalKind, terminalStopReason: row.TerminalStopReason,
		terminalDisposition: row.TerminalDisposition, terminalReason: row.TerminalReasonCode,
		terminalTokensReported: int(row.TerminalTokensReported), terminalTokensUsed: row.TerminalTokensUsed,
		terminalAt: terminalAt,
	}
}

func goalSQLNullInt64(value any) (sql.NullInt64, error) {
	switch typed := value.(type) {
	case nil:
		return sql.NullInt64{}, nil
	case int:
		return sql.NullInt64{Int64: int64(typed), Valid: true}, nil
	case int64:
		return sql.NullInt64{Int64: typed, Valid: true}, nil
	default:
		return sql.NullInt64{}, fmt.Errorf("store: Goal integer encoded as %T", value)
	}
}

func goalNullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func goalNullablePositiveInt64(value int64) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
}

func goalNullableString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func goalRequiredSQLString(value string) sql.NullString {
	return sql.NullString{String: strings.TrimSpace(value), Valid: true}
}
