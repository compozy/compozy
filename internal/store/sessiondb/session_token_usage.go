package sessiondb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb/sqlcgen"
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
