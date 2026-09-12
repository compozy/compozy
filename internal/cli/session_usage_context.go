package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func sessionContextHuman(value contract.SessionContextPayload) string {
	if (value.State == "" || value.State == contract.SessionContextStateUnknown) && value.Injected == nil {
		return ""
	}
	rows := []keyValue{{Label: "State", Value: string(value.State)}}
	if value.Used != nil {
		used := formatInt64Ptr(value.Used)
		if value.Size != nil {
			used += " / " + formatInt64Ptr(value.Size)
		}
		if value.Ratio != nil {
			used += fmt.Sprintf(" (%.0f%%)", *value.Ratio*100)
		}
		rows = append(rows, keyValue{Label: "Used", Value: used})
	}
	if value.SizeSource != "" {
		rows = append(rows, keyValue{Label: "Window Source", Value: value.SizeSource})
	}
	if value.ReportedAt != nil {
		rows = append(
			rows,
			keyValue{
				Label: "Reported",
				Value: fmt.Sprintf(
					"%s · seq %s · %s",
					value.ReportedTurnID,
					formatInt64Ptr(value.Sequence),
					value.ReportedAt.Format("2006-01-02 15:04:05"),
				),
			},
		)
	}
	if value.Stale != nil && *value.Stale {
		rows = append(rows, keyValue{Label: "Freshness", Value: "stale · as of " + value.ReportedTurnID})
	}
	if value.PressureThreshold != nil {
		rows = append(
			rows,
			keyValue{Label: "Compaction At", Value: fmt.Sprintf("%.0f%%", *value.PressureThreshold*100)},
		)
	}
	if value.Injected != nil {
		rows = append(
			rows,
			keyValue{
				Label: "Compozy Sent",
				Value: fmt.Sprintf("≈ %d (%s)", value.Injected.Tokens, value.Injected.Estimate),
			},
		)
		for _, row := range value.Injected.Rows {
			rows = append(rows, keyValue{Label: "  " + row.Key, Value: sessionContextRowHuman(row)})
		}
	}
	return renderHumanSection("Context", rows)
}

func sessionContextRowHuman(row contract.SessionContextRowPayload) string {
	quantity := fmt.Sprintf("%d B", row.Bytes)
	if row.Tokens != nil {
		quantity = "≈ " + formatInt64Ptr(row.Tokens)
	}
	pieces := []string{quantity, row.DeliveredTurnID}
	if row.OwnerKind == "startup_opaque" {
		pieces = append(pieces, "included in the startup prompt")
	}
	if row.Name != "" {
		pieces = append(pieces, row.Name)
	}
	if row.Kind == "binary" {
		pieces = append(pieces, "binary, no estimate")
	}
	if row.Delivery != "" {
		pieces = append(pieces, row.Delivery)
	}
	if row.Unchanged {
		pieces = append(pieces, "unchanged since "+row.DeliveredTurnID+" (last seen "+row.LastSeenTurnID+")")
	}
	if row.Stale {
		pieces = append(pieces, "may have been summarized")
	}
	return strings.Join(pieces, " · ")
}

func sessionUsageToon(record SessionUsageRecord, cost string) string {
	value := record.Context
	fields := []string{
		cliInputTokensKey,
		cliOutputTokensKey,
		"total_tokens",
		"total_cost",
		"cost_status",
		"cost_source",
		"turn_count",
		"cache_read_tokens",
		"cache_write_tokens",
		"context_state",
		"context_used",
		"context_size",
		"context_ratio",
		"context_size_source",
		"context_stale",
		"context_sequence",
		"context_reported_turn_id",
		"context_pressure_threshold",
		"injected_tokens",
		"injected_stale",
	}
	injectedTokens, injectedStale := "", ""
	if value.Injected != nil {
		injectedTokens = strconv.FormatInt(value.Injected.Tokens, 10)
		injectedStale = strconv.FormatBool(value.Injected.Stale)
	}
	stale := ""
	if value.Stale != nil {
		stale = strconv.FormatBool(*value.Stale)
	}
	values := []string{
		formatInt64Ptr(record.InputTokens),
		formatInt64Ptr(record.OutputTokens),
		formatInt64Ptr(record.TotalTokens),
		cost,
		string(record.CostStatus),
		string(record.CostSource),
		strconv.FormatInt(record.TurnCount, 10),
		formatInt64Ptr(record.CacheReadTokens),
		formatInt64Ptr(record.CacheWriteTokens),
		string(value.State),
		formatInt64Ptr(value.Used),
		formatInt64Ptr(value.Size),
		formatFloat64Ptr(value.Ratio),
		value.SizeSource,
		stale,
		formatInt64Ptr(value.Sequence),
		value.ReportedTurnID,
		formatFloat64Ptr(value.PressureThreshold),
		injectedTokens,
		injectedStale,
	}
	return renderToonObject("session_usage", fields, values)
}
