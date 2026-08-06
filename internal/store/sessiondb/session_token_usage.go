package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
)

// RecordTokenUsage stores or merges per-turn usage data for the session.
func (s *SessionDB) RecordTokenUsage(ctx context.Context, usage store.TokenUsage) error {
	if s == nil {
		return errors.New("store: session database is required")
	}
	if ctx == nil {
		return errors.New("store: record token usage context is required")
	}
	if err := usage.Validate(); err != nil {
		return err
	}

	return s.enqueueWrite(ctx, sessionWriteRequest{
		ctx:    ctx,
		kind:   sessionWriteUsage,
		usage:  usage,
		result: make(chan sessionWriteResult, 1),
	})
}

// ListTokenUsage returns the per-turn usage projection recorded for the session.
func (s *SessionDB) ListTokenUsage(ctx context.Context) ([]store.TokenUsage, error) {
	if s == nil {
		return nil, errors.New("store: session database is required")
	}
	if ctx == nil {
		return nil, errors.New("store: list token usage context is required")
	}
	rows, err := sqlcgen.New(s.db).ListTokenUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list token usage: %w", err)
	}
	return tokenUsageFromGenerated(rows)
}

func tokenUsageFromGenerated(rows []sqlcgen.TokenUsage) ([]store.TokenUsage, error) {
	usages := make([]store.TokenUsage, 0, len(rows))
	for _, row := range rows {
		timestamp, err := store.ParseTimestamp(row.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("store: parse token usage timestamp for turn %q: %w", row.TurnID, err)
		}
		usages = append(usages, store.TokenUsage{
			TurnID:           row.TurnID,
			InputTokens:      nullableInt64Pointer(row.InputTokens),
			OutputTokens:     nullableInt64Pointer(row.OutputTokens),
			TotalTokens:      nullableInt64Pointer(row.TotalTokens),
			ThoughtTokens:    nullableInt64Pointer(row.ThoughtTokens),
			CacheReadTokens:  nullableInt64Pointer(row.CacheReadTokens),
			CacheWriteTokens: nullableInt64Pointer(row.CacheWriteTokens),
			ContextUsed:      nullableInt64Pointer(row.ContextUsed),
			ContextSize:      nullableInt64Pointer(row.ContextSize),
			CostAmount:       nullableFloat64Pointer(row.CostAmount),
			CostCurrency:     nullableStringPointer(row.CostCurrency),
			Timestamp:        timestamp,
		})
	}
	return usages, nil
}

func nullableInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullableFloat64Pointer(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func nullableStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func (s *SessionDB) writeTokenUsage(ctx context.Context, usage store.TokenUsage) error {
	if usage.Timestamp.IsZero() {
		usage.Timestamp = s.now()
	}

	if err := sqlcgen.New(s.db).UpsertTokenUsage(ctx, sqlcgen.UpsertTokenUsageParams{
		TurnID:           usage.TurnID,
		InputTokens:      sessionNullableInt64(usage.InputTokens),
		OutputTokens:     sessionNullableInt64(usage.OutputTokens),
		TotalTokens:      sessionNullableInt64(usage.TotalTokens),
		ThoughtTokens:    sessionNullableInt64(usage.ThoughtTokens),
		CacheReadTokens:  sessionNullableInt64(usage.CacheReadTokens),
		CacheWriteTokens: sessionNullableInt64(usage.CacheWriteTokens),
		ContextUsed:      sessionNullableInt64(usage.ContextUsed),
		ContextSize:      sessionNullableInt64(usage.ContextSize),
		CostAmount:       sessionNullableFloat64(usage.CostAmount),
		CostCurrency:     sessionNullableStringPointer(usage.CostCurrency),
		Timestamp:        store.FormatTimestamp(usage.Timestamp),
	}); err != nil {
		return fmt.Errorf("store: upsert token usage: %w", err)
	}

	return nil
}
