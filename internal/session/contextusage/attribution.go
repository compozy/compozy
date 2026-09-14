package contextusage

import (
	"cmp"
	"slices"
	"time"
)

func deriveInjected(deliveries []Delivery, observations []UsageEvent) *Injected {
	if len(deliveries) == 0 {
		return nil
	}
	ordered := slices.Clone(deliveries)
	slices.SortStableFunc(ordered, func(a, b Delivery) int { return cmp.Compare(a.Sequence, b.Sequence) })
	rows := make([]Row, 0)
	owners := make(map[string]int)
	for _, delivery := range ordered {
		for _, span := range delivery.Spans {
			index, exists := owners[span.Key]
			if span.Key == "attachment" {
				rows = append(rows, fullRow(delivery, span))
				continue
			}
			if span.Unchanged && exists {
				rows[index].Unchanged = true
				rows[index].LastSeenTurnID = delivery.TurnID
				continue
			}
			row := fullRow(delivery, span)
			if span.Unchanged && span.StartupDedup {
				row.OwnerKind = "startup_opaque"
				row.Label = "included in the startup prompt"
				row.Tokens = nil
				row.Unchanged = true
				row.LastSeenTurnID = delivery.TurnID
			} else if span.Unchanged {
				// A stub cannot establish a measured owner without the original delivery.
				continue
			}
			if exists {
				rows[index] = row
			} else {
				owners[span.Key] = len(rows)
				rows = append(rows, row)
			}
		}
	}
	drops := contextDrops(observations)
	result := &Injected{Estimate: ordered[len(ordered)-1].Estimate, Rows: rows}
	for index := range result.Rows {
		row := &result.Rows[index]
		row.Stale = slices.ContainsFunc(drops, func(drop time.Time) bool { return row.SentAt.Before(drop) })
		if row.Tokens != nil {
			result.Tokens += *row.Tokens
		}
		result.Stale = result.Stale || row.Stale
	}
	return result
}

func fullRow(delivery Delivery, span Span) Row {
	return Row{
		Key: span.Key, Label: sectionLabel(span.Key), Kind: span.Kind, Delivery: span.Delivery,
		Name: span.Name, OwnerKind: "full", Bytes: span.Bytes, Tokens: span.Tokens,
		DeliveredTurnID: delivery.TurnID, DeliverySequence: delivery.Sequence, SentAt: delivery.SentAt,
		HookModified: span.HookModified,
	}
}

func contextDrops(observations []UsageEvent) []time.Time {
	var drops []time.Time
	for index := 1; index < len(observations); index++ {
		if *observations[index].Usage.ContextUsed < *observations[index-1].Usage.ContextUsed {
			drops = append(drops, observations[index].At)
		}
	}
	return drops
}

func sectionLabel(key string) string {
	switch key {
	case "system_prompt":
		return "System prompt"
	case "runtime_identity":
		return "Runtime instructions"
	case "agent_prompt":
		return "Agent prompt"
	case "situation":
		return "Situation"
	case "memory":
		return "Memory"
	case "soul":
		return "Soul"
	case "skills":
		return "Skills catalog"
	case "tools":
		return "Tool manuals"
	case "network":
		return "Network"
	case "knowledge":
		return "Workspace knowledge"
	case "attachment":
		return "Attachments"
	default:
		return key
	}
}
