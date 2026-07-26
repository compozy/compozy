package session

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

func (m *Manager) promptPersistenceFailure(
	session *Session,
	turnID string,
	err error,
) (*store.SessionFailure, string) {
	failure := sessionFailureFromError(
		fmt.Errorf("prompt event persistence failed: %w", err),
		store.FailureTransport,
	)
	summary := failureSummary(failure, "prompt event persistence failed")
	m.sessionLogger(session).Error(
		"session: prompt event persistence failed",
		"turn_id", turnID,
		"error", summary,
	)
	return failure, summary
}

func (m *Manager) recordPromptTokenUsageProjection(
	ctx context.Context,
	session *Session,
	recorder EventRecorder,
	event acp.AgentEvent,
) {
	if event.Usage == nil {
		return
	}
	// The canonical AgentEvent, including usage, is already committed. This
	// rebuildable projection is attempted once and must never replay the event.
	usage := store.TokenUsage{
		TurnID:           event.Usage.TurnID,
		InputTokens:      event.Usage.InputTokens,
		OutputTokens:     event.Usage.OutputTokens,
		TotalTokens:      event.Usage.TotalTokens,
		ThoughtTokens:    event.Usage.ThoughtTokens,
		CacheReadTokens:  event.Usage.CacheReadTokens,
		CacheWriteTokens: event.Usage.CacheWriteTokens,
		ContextUsed:      event.Usage.ContextUsed,
		ContextSize:      event.Usage.ContextSize,
		CostAmount:       event.Usage.CostAmount,
		CostCurrency:     event.Usage.CostCurrency,
		Timestamp:        event.Usage.Timestamp,
	}
	if err := recorder.RecordTokenUsage(ctx, usage); err != nil {
		failure := sessionFailureFromError(err, store.FailureTransport)
		m.sessionLogger(session).Warn(
			"session: record rebuildable prompt token usage projection failed",
			"turn_id", usage.TurnID,
			"error", failureSummary(failure, "token usage projection failed"),
		)
	}
}
