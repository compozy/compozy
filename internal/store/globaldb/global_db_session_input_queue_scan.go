package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func sessionInputQueueFromGenerated(row *sqlcgen.SessionInputQueue) (store.SessionInputQueueEntry, error) {
	entry := store.SessionInputQueueEntry{
		ID:                       row.ID,
		SessionID:                row.SessionID,
		Status:                   row.Status,
		Mode:                     row.Mode,
		Text:                     row.Text,
		SessionGeneration:        row.SessionGeneration,
		TaskRunID:                row.TaskRunID,
		RunGeneration:            nullableInt64Pointer(row.RunGeneration),
		AttemptCount:             int(row.AttemptCount),
		FailureSummary:           row.FailureSummary,
		LoopRunID:                strings.TrimSpace(row.LoopRunID.String),
		OwnerKind:                strings.TrimSpace(row.OwnerKind.String),
		OwnerEpoch:               nullableInt64Pointer(row.OwnerEpoch),
		BindingEpoch:             nullableInt64Pointer(row.BindingEpoch),
		PromptID:                 strings.TrimSpace(row.PromptID.String),
		PromptKind:               strings.TrimSpace(row.PromptKind.String),
		OperationUsageBaseTokens: nullableInt64Pointer(row.OperationUsageBaseTokens),
		PromptAttempt:            int(row.PromptAttempt),
		Dispatchable:             row.Dispatchable != 0,
		DispatchTokenHash:        strings.TrimSpace(row.DispatchTokenHash.String),
		FenceKind: strings.TrimSpace(
			row.FenceKind.String,
		),
		FenceDisposition:       strings.TrimSpace(row.FenceDisposition.String),
		FenceReasonCode:        strings.TrimSpace(row.FenceReasonCode.String),
		TerminalEventStartSeq:  nullableInt64Pointer(row.TerminalEventStartSeq),
		TerminalEventEndSeq:    nullableInt64Pointer(row.TerminalEventEndSeq),
		TerminalKind:           strings.TrimSpace(row.TerminalKind.String),
		TerminalStopReason:     strings.TrimSpace(row.TerminalStopReason.String),
		TerminalDisposition:    strings.TrimSpace(row.TerminalDisposition.String),
		TerminalReasonCode:     strings.TrimSpace(row.TerminalReasonCode.String),
		TerminalTokensReported: row.TerminalTokensReported != 0,
		TerminalTokensUsed:     nullableInt64Pointer(row.TerminalTokensUsed),
	}
	if err := parseSessionInputQueueTimes(
		&entry, row.EnqueuedAt, row.UpdatedAt, row.DispatchStartedAt, row.SentAt,
		row.FailedAt, row.CanceledAt, sqlNullStringFromTime(row.ActivatedAt),
		sqlNullStringFromTime(row.FencedAt), sqlNullStringFromTime(row.TerminalAt),
	); err != nil {
		return store.SessionInputQueueEntry{}, fmt.Errorf("store: parse session input queue entry times: %w", err)
	}
	return entry, nil
}

func sqlNullStringFromTime(value sql.NullTime) sql.NullString {
	if !value.Valid {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value.Time), Valid: true}
}

func getSessionInputQueueEntry(
	ctx context.Context,
	exec globalSQLExecutor,
	sessionID string,
	entryID string,
) (store.SessionInputQueueEntry, error) {
	row, err := sqlcgen.New(exec).GetSessionInputQueueEntry(ctx, sqlcgen.GetSessionInputQueueEntryParams{
		SessionID: sessionID, ID: entryID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionInputQueueEntry{}, fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, entryID)
	}
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return sessionInputQueueFromGenerated(&row)
}

func parseSessionInputQueueTimes(
	entry *store.SessionInputQueueEntry,
	enqueuedAtRaw string,
	updatedAtRaw string,
	dispatchStartedAt sql.NullString,
	sentAt sql.NullString,
	failedAt sql.NullString,
	canceledAt sql.NullString,
	activatedAt sql.NullString,
	fencedAt sql.NullString,
	terminalAt sql.NullString,
) error {
	parsedEnqueuedAt, err := store.ParseTimestamp(enqueuedAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse session input enqueued_at: %w", err)
	}
	entry.EnqueuedAt = parsedEnqueuedAt
	parsedUpdatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse session input updated_at: %w", err)
	}
	entry.UpdatedAt = parsedUpdatedAt
	for name, target := range map[string]struct {
		raw sql.NullString
		set func(*time.Time)
	}{
		"dispatch_started_at": {dispatchStartedAt, func(value *time.Time) { entry.DispatchStartedAt = value }},
		"sent_at":             {sentAt, func(value *time.Time) { entry.SentAt = value }},
		"failed_at":           {failedAt, func(value *time.Time) { entry.FailedAt = value }},
		"canceled_at":         {canceledAt, func(value *time.Time) { entry.CanceledAt = value }},
		"activated_at":        {activatedAt, func(value *time.Time) { entry.ActivatedAt = value }},
		"fenced_at":           {fencedAt, func(value *time.Time) { entry.FencedAt = value }},
		"terminal_at":         {terminalAt, func(value *time.Time) { entry.TerminalAt = value }},
	} {
		value, err := parseOptionalSessionInputTimestamp(target.raw)
		if err != nil {
			return fmt.Errorf("store: parse session input %s: %w", name, err)
		}
		target.set(value)
	}
	return nil
}

func nullableInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func parseOptionalSessionInputTimestamp(value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	parsed, err := store.ParseTimestamp(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
