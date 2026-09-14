package core

import (
	"context"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/session/contextusage"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) sessionContextInput(
	ctx context.Context,
	id string,
	info *session.Info,
) (contextusage.Input, error) {
	input := contextusage.Input{}
	usage, err := h.Sessions.UsageEvents(ctx, id)
	if err != nil {
		return input, err
	}
	deliveries, err := h.Sessions.Deliveries(ctx, id)
	if err != nil {
		return input, err
	}
	compactions, err := h.Sessions.Compactions(ctx, id)
	if err != nil {
		return input, err
	}
	settled, err := h.Sessions.LatestSettledTurn(ctx, id)
	if err != nil {
		return input, err
	}
	populateContextEvents(&input, usage, deliveries, compactions)
	if settled.Sequence > 0 {
		input.Settled = &contextusage.SettledTurn{TurnID: settled.TurnID, Sequence: settled.Sequence}
	}
	if h.Config.Session.Compaction.Enabled && h.Config.Session.Compaction.PressureThreshold > 0 {
		input.Threshold = new(h.Config.Session.Compaction.PressureThreshold)
	}
	if h.ContextWindowResolver != nil && info != nil {
		window, err := h.ContextWindowResolver.ContextWindow(ctx, info.Provider, info.Model)
		if err != nil {
			h.sessionStreamLogger().
				WarnContext(ctx, "api: session catalog context window unavailable", "session_id", id, "error", err)
		} else {
			input.CatalogWindow = window
		}
	}
	input.Available = true
	return input, nil
}

func (h *BaseHandlers) SessionUsageTurns(c *gin.Context) {
	_, id, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	input, err := h.sessionContextInput(c.Request.Context(), id, info)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, fmt.Errorf("api: read session usage turns: %w", err))
		return
	}
	turns, compactions := contextusage.Turns(input)
	c.JSON(http.StatusOK, sessionUsageTurnsPayload(turns, compactions))
}

func populateContextEvents(
	input *contextusage.Input,
	usage []session.UsageEventEnvelope,
	deliveries []session.DeliveryEventEnvelope,
	compactions []session.CompactionEnvelope,
) {
	for _, event := range usage {
		u := event.Usage
		input.UsageEvents = append(input.UsageEvents, contextusage.UsageEvent{
			Sequence: event.Sequence, At: event.At, TurnID: event.TurnID, Usage: store.TokenUsage{
				TurnID:           event.TurnID,
				InputTokens:      u.InputTokens,
				OutputTokens:     u.OutputTokens,
				TotalTokens:      u.TotalTokens,
				ThoughtTokens:    u.ThoughtTokens,
				CacheReadTokens:  u.CacheReadTokens,
				CacheWriteTokens: u.CacheWriteTokens,
				ContextUsed:      u.ContextUsed,
				ContextSize:      u.ContextSize,
				CostAmount:       u.CostAmount,
				CostCurrency:     u.CostCurrency,
				Timestamp:        u.Timestamp,
			},
		})
	}
	for _, event := range deliveries {
		delivery := contextusage.Delivery{
			Sequence: event.Sequence,
			At:       event.At,
			SentAt:   event.Manifest.SentAt,
			TurnID:   event.Manifest.TurnID,
			Estimate: event.Manifest.Estimate,
		}
		for _, span := range event.Manifest.Spans {
			delivery.Spans = append(
				delivery.Spans,
				contextusage.Span{
					Key:          span.Key,
					Kind:         span.Kind,
					Delivery:     span.Delivery,
					Name:         span.Name,
					Bytes:        span.Bytes,
					Tokens:       span.Tokens,
					Unchanged:    span.Unchanged,
					StartupDedup: span.StartupDedup,
					HookModified: span.HookModified,
				},
			)
		}
		input.Deliveries = append(input.Deliveries, delivery)
	}
	for _, event := range compactions {
		p := event.Payload
		input.Compactions = append(
			input.Compactions,
			contextusage.Compaction{
				Sequence:     event.Sequence,
				At:           event.At,
				TurnID:       p.TurnID,
				FromSequence: p.FromSequence,
				ToSequence:   p.ToSequence,
				Used:         p.ContextUsed,
				Size:         p.ContextSize,
				Pressure:     p.Pressure,
				Strategy:     p.Strategy,
				SpanArchived: event.SpanArchived,
			},
		)
	}
}
