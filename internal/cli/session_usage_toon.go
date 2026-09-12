package cli

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

const (
	sessionContextUsedKey  = "context_used"
	sessionContextSizeKey  = "context_size"
	sessionCostCurrencyKey = "cost_currency"
)

func sessionContextRowsToon(injected *contract.SessionContextInjectedPayload) string {
	if injected == nil {
		return ""
	}
	rows := make([][]string, 0, len(injected.Rows))
	for _, row := range injected.Rows {
		rows = append(rows, []string{row.Key, row.Label, row.Kind, row.OwnerKind,
			strconv.FormatInt(row.Bytes, 10), formatInt64Ptr(row.Tokens), row.DeliveredTurnID,
			strconv.FormatInt(row.DeliverySequence, 10), row.SentAt.Format(time.RFC3339Nano), row.LastSeenTurnID,
			strconv.FormatBool(row.Unchanged), strconv.FormatBool(row.Stale), row.Delivery,
			strconv.FormatBool(row.HookModified), row.Name})
	}
	return renderToonArray("context_rows", []string{"key", "label", "kind", "owner_kind", "bytes", observeTokensLabel,
		"delivered_turn_id", "delivery_sequence", "sent_at", "last_seen_turn_id", "unchanged", "stale",
		"delivery", "hook_modified", "name"}, rows)
}

func sessionUsageTurnsToon(value contract.SessionUsageTurnsResponse) (string, error) {
	turns, usages, deliveries, spans, compactions := [][]string{}, [][]string{}, [][]string{}, [][]string{}, [][]string{}
	for _, turn := range value.Turns {
		turns = append(turns, []string{turn.TurnID, strconv.FormatInt(turn.Sequence, 10)})
		if usage := turn.Usage; usage != nil {
			row, err := sessionUsageReportToonRow(turn.TurnID, usage)
			if err != nil {
				return "", err
			}
			usages = append(usages, row)
		}
		if injected := turn.Injected; injected != nil {
			deliveries = append(
				deliveries,
				[]string{turn.TurnID, injected.Estimate, strconv.FormatInt(injected.Tokens, 10),
					strconv.FormatInt(injected.Sequence, 10), injected.SentAt.Format(time.RFC3339Nano)},
			)
			for _, span := range injected.Spans {
				spans = append(spans, sessionContextSpanToonRow(turn.TurnID, span))
			}
		}
	}
	for _, marker := range value.Compactions {
		compactions = append(compactions, []string{marker.TurnID, strconv.FormatInt(marker.Sequence, 10),
			marker.At.Format(time.RFC3339Nano), strconv.FormatBool(marker.SpanArchived),
			strconv.FormatInt(marker.FromSequence, 10), strconv.FormatInt(marker.ToSequence, 10),
			strconv.FormatInt(marker.ContextUsed, 10), strconv.FormatInt(marker.ContextSize, 10),
			strconv.FormatFloat(marker.Pressure, 'g', -1, 64), marker.Strategy})
	}
	return renderHumanBlocks(
		renderToonArray("session_usage_turns", []string{sessionTurnIDKey, sessionSequenceKey}, turns),
		renderToonArray(
			"usage",
			[]string{
				sessionTurnIDKey,
				sessionSequenceKey,
				"reported_turn_id",
				"input_tokens",
				"output_tokens",
				"total_tokens",
				"thought_tokens",
				"cache_read_tokens",
				"cache_write_tokens",
				sessionContextUsedKey,
				sessionContextSizeKey,
				"cost_amount",
				sessionCostCurrencyKey,
				"timestamp",
				"meta",
			},
			usages,
		),
		renderToonArray(
			"deliveries",
			[]string{sessionTurnIDKey, "estimate", observeTokensLabel, sessionSequenceKey, "sent_at"},
			deliveries,
		),
		renderToonArray(
			"spans",
			[]string{sessionTurnIDKey, "key", "kind", "bytes", observeTokensLabel, "unchanged", "startup_dedup",
				"delivery", "hook_modified", "name"},
			spans,
		),
		renderToonArray(
			"compactions",
			[]string{sessionTurnIDKey, sessionSequenceKey, "at", "span_archived", "from_sequence",
				"to_sequence", sessionContextUsedKey, sessionContextSizeKey, "pressure", "strategy"},
			compactions,
		),
	), nil
}

func sessionUsageReportToonRow(turnID string, usage *contract.TokenUsagePayload) ([]string, error) {
	meta, err := json.Marshal(usage.Meta)
	if err != nil {
		return nil, err
	}
	return []string{
		turnID,
		formatInt64Ptr(usage.Sequence),
		usage.TurnID,
		formatInt64Ptr(usage.InputTokens),
		formatInt64Ptr(usage.OutputTokens),
		formatInt64Ptr(usage.TotalTokens),
		formatInt64Ptr(usage.ThoughtTokens),
		formatInt64Ptr(usage.CacheReadTokens),
		formatInt64Ptr(usage.CacheWriteTokens),
		formatInt64Ptr(usage.ContextUsed),
		formatInt64Ptr(usage.ContextSize),
		formatFloat64Ptr(usage.CostAmount),
		formatStringPtr(usage.CostCurrency),
		usage.Timestamp.Format(time.RFC3339Nano),
		string(meta),
	}, nil
}

func sessionContextSpanToonRow(turnID string, span contract.SessionContextSpanPayload) []string {
	return []string{
		turnID,
		span.Key,
		span.Kind,
		strconv.FormatInt(span.Bytes, 10),
		formatInt64Ptr(span.Tokens),
		strconv.FormatBool(span.Unchanged),
		strconv.FormatBool(span.StartupDedup),
		span.Delivery,
		strconv.FormatBool(span.HookModified),
		span.Name,
	}
}
