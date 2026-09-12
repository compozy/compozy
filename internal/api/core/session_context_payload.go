package core

import (
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session/contextusage"
)

func contextUsagePayload(value contextusage.ContextUsage) contract.SessionContextPayload {
	payload := contract.SessionContextPayload{
		State:             contract.SessionContextState(value.State),
		Used:              value.Used,
		Size:              value.Size,
		Ratio:             value.Ratio,
		SizeSource:        value.SizeSource,
		Stale:             value.Stale,
		Sequence:          value.Sequence,
		ReportedTurnID:    value.ReportedTurnID,
		ReportedAt:        value.ReportedAt,
		PressureThreshold: value.PressureThreshold,
	}
	if value.Injected == nil {
		return payload
	}
	payload.Injected = &contract.SessionContextInjectedPayload{
		Estimate: value.Injected.Estimate,
		Tokens:   value.Injected.Tokens,
		Stale:    value.Injected.Stale,
		Rows:     make([]contract.SessionContextRowPayload, 0, len(value.Injected.Rows)),
	}
	for _, row := range value.Injected.Rows {
		payload.Injected.Rows = append(
			payload.Injected.Rows,
			contract.SessionContextRowPayload{
				Key:              row.Key,
				Label:            row.Label,
				Kind:             row.Kind,
				OwnerKind:        row.OwnerKind,
				Bytes:            row.Bytes,
				Tokens:           row.Tokens,
				DeliveredTurnID:  row.DeliveredTurnID,
				DeliverySequence: row.DeliverySequence,
				SentAt:           row.SentAt,
				LastSeenTurnID:   row.LastSeenTurnID,
				Unchanged:        row.Unchanged,
				Stale:            row.Stale,
				Delivery:         row.Delivery,
				HookModified:     row.HookModified,
				Name:             row.Name,
			},
		)
	}
	return payload
}

func sessionUsageTurnsPayload(
	turns []contextusage.Turn,
	compactions []contextusage.Compaction,
) contract.SessionUsageTurnsResponse {
	response := contract.SessionUsageTurnsResponse{
		Turns:       make([]contract.SessionUsageTurnPayload, 0, len(turns)),
		Compactions: make([]contract.SessionCompactionPayload, 0, len(compactions)),
	}
	for _, turn := range turns {
		row := contract.SessionUsageTurnPayload{TurnID: turn.TurnID, Sequence: turn.Sequence}
		if u := turn.Usage; u != nil {
			usage := acp.TokenUsage{
				TurnID:           turn.TurnID,
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
			}
			if turn.UsageSequence != nil {
				usage.Sequence = *turn.UsageSequence
			}
			row.Usage = TokenUsagePayloadFromUsage(&usage)
		}
		if delivery := turn.Injected; delivery != nil {
			row.Injected = &contract.SessionTurnInjectedPayload{
				Estimate: delivery.Estimate,
				Sequence: delivery.Sequence,
				SentAt:   delivery.SentAt,
				Spans:    make([]contract.SessionContextSpanPayload, 0, len(delivery.Spans)),
			}
			for _, span := range delivery.Spans {
				row.Injected.Spans = append(
					row.Injected.Spans,
					contract.SessionContextSpanPayload{
						Key:          span.Key,
						Kind:         span.Kind,
						Bytes:        span.Bytes,
						Tokens:       span.Tokens,
						Unchanged:    span.Unchanged,
						StartupDedup: span.StartupDedup,
						Delivery:     span.Delivery,
						HookModified: span.HookModified,
						Name:         span.Name,
					},
				)
				if span.Tokens != nil {
					row.Injected.Tokens += *span.Tokens
				}
			}
		}
		response.Turns = append(response.Turns, row)
	}
	for _, marker := range compactions {
		response.Compactions = append(
			response.Compactions,
			contract.SessionCompactionPayload{
				TurnID:       marker.TurnID,
				Sequence:     marker.Sequence,
				At:           marker.At,
				SpanArchived: marker.SpanArchived,
				FromSequence: marker.FromSequence,
				ToSequence:   marker.ToSequence,
				ContextUsed:  marker.Used,
				ContextSize:  marker.Size,
				Pressure:     marker.Pressure,
				Strategy:     marker.Strategy,
			},
		)
	}
	return response
}
