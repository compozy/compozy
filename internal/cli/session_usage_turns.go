package cli

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/modelcatalog"
)

const sequenceHeader = "SEQ"

func sessionUsageTurnsBundle(value contract.SessionUsageTurnsResponse) outputBundle {
	rows := sessionUsageTurnRows(value)
	headers := []string{
		"TURN",
		sequenceHeader,
		"TIME",
		"USED / SIZE",
		"IN",
		"OUT",
		"CACHE R",
		"CACHE W",
		"COST",
		"COMPOZY",
	}
	return outputBundle{
		jsonValue: value,
		human:     func() (string, error) { return renderHumanTable("Session Usage Turns", headers, rows), nil },
		toon: func() (string, error) {
			return renderToonArray(
				"session_usage_turns",
				[]string{
					"turn",
					"sequence",
					"time",
					"used_size",
					"input",
					"output",
					"cache_read",
					"cache_write",
					"cost",
					"compozy",
				},
				rows,
			), nil
		},
	}
}

func sessionUsageTurnRows(value contract.SessionUsageTurnsResponse) [][]string {
	type orderedRow struct {
		sequence int64
		values   []string
	}
	ordered := make([]orderedRow, 0, len(value.Turns)+len(value.Compactions))
	for _, turn := range value.Turns {
		row := sessionUsageTurnRow(turn)
		ordered = append(ordered, orderedRow{sequence: turn.Sequence, values: row})
	}
	for _, marker := range value.Compactions {
		archived := "not archived"
		if marker.SpanArchived {
			archived = "archived"
		}
		description := fmt.Sprintf(
			"CompozyOS compaction · at %.0f%% · %d / %d · sequences %d–%d · replay span %s",
			marker.Pressure*100,
			marker.ContextUsed,
			marker.ContextSize,
			marker.FromSequence,
			marker.ToSequence,
			archived,
		)
		ordered = append(
			ordered,
			orderedRow{
				sequence: marker.Sequence,
				values: []string{
					marker.TurnID,
					strconv.FormatInt(marker.Sequence, 10),
					marker.At.Format("15:04:05"),
					description,
					"",
					"",
					"",
					"",
					"",
					"",
				},
			},
		)
	}
	slices.SortStableFunc(ordered, func(a, b orderedRow) int { return cmp.Compare(a.sequence, b.sequence) })
	rows := make([][]string, 0, len(ordered))
	for _, row := range ordered {
		rows = append(rows, row.values)
	}
	return rows
}

func sessionUsageTurnRow(turn contract.SessionUsageTurnPayload) []string {
	row := []string{turn.TurnID, strconv.FormatInt(turn.Sequence, 10), "-", "-", "-", "-", "-", "-", "-", "-"}
	if usage := turn.Usage; usage != nil {
		if !usage.Timestamp.IsZero() {
			row[2] = usage.Timestamp.Format("15:04:05")
		}
		if usage.ContextUsed != nil {
			row[3] = formatInt64Ptr(usage.ContextUsed)
			if usage.ContextSize != nil {
				row[3] += " / " + formatInt64Ptr(usage.ContextSize)
			}
		}
		row[4] = stringOrDash(formatInt64Ptr(usage.InputTokens))
		row[5] = stringOrDash(formatInt64Ptr(usage.OutputTokens))
		row[6] = stringOrDash(formatInt64Ptr(usage.CacheReadTokens))
		row[7] = stringOrDash(formatInt64Ptr(usage.CacheWriteTokens))
		row[8] = stringOrDash(
			formatCostProvenance(
				usage.CostAmount,
				formatStringPtr(usage.CostCurrency),
				modelcatalog.CostStatusActual,
			),
		)
	}
	if delivery := turn.Injected; delivery != nil {
		if row[2] == "-" {
			row[2] = delivery.SentAt.Format("15:04:05")
		}
		row[9] = fmt.Sprintf("≈ %d", delivery.Tokens)
		unchanged := len(delivery.Spans) > 0
		for _, span := range delivery.Spans {
			unchanged = unchanged && span.Unchanged
		}
		if unchanged {
			row[9] += " (unchanged)"
		}
	}
	return row
}
