package contextusage

import (
	"cmp"
	"slices"

	"github.com/compozy/compozy/internal/store"
)

func Turns(in Input) ([]Turn, []Compaction) {
	rows := make(map[string]Turn)
	usageEvents := slices.Clone(in.UsageEvents)
	slices.SortStableFunc(usageEvents, func(a, b UsageEvent) int { return cmp.Compare(a.Sequence, b.Sequence) })
	for _, event := range usageEvents {
		row := rows[event.TurnID]
		row.TurnID = event.TurnID
		row.Sequence = max(row.Sequence, event.Sequence)
		row.Usage = mergeUsage(row.Usage, event.Usage)
		row.UsageSequence = new(event.Sequence)
		rows[event.TurnID] = row
	}
	for _, delivery := range in.Deliveries {
		row := rows[delivery.TurnID]
		row.TurnID = delivery.TurnID
		row.Sequence = max(row.Sequence, delivery.Sequence)
		if row.Injected == nil || row.Injected.Sequence < delivery.Sequence {
			row.Injected = new(delivery)
		}
		rows[delivery.TurnID] = row
	}
	turns := make([]Turn, 0, len(rows))
	for _, row := range rows {
		turns = append(turns, row)
	}
	slices.SortFunc(turns, func(a, b Turn) int {
		return cmp.Or(cmp.Compare(a.Sequence, b.Sequence), cmp.Compare(a.TurnID, b.TurnID))
	})
	compactions := slices.Clone(in.Compactions)
	if compactions == nil {
		compactions = []Compaction{}
	}
	slices.SortStableFunc(compactions, func(a, b Compaction) int { return cmp.Compare(a.Sequence, b.Sequence) })
	return turns, compactions
}

func mergeUsage(previous *store.TokenUsage, next store.TokenUsage) *store.TokenUsage {
	if previous == nil {
		return new(next)
	}
	merged := *previous
	merged.TurnID = next.TurnID
	merged.Timestamp = next.Timestamp
	for _, pair := range []struct {
		target **int64
		value  *int64
	}{
		{&merged.InputTokens, next.InputTokens}, {&merged.OutputTokens, next.OutputTokens},
		{&merged.TotalTokens, next.TotalTokens}, {&merged.ThoughtTokens, next.ThoughtTokens},
		{&merged.CacheReadTokens, next.CacheReadTokens}, {&merged.CacheWriteTokens, next.CacheWriteTokens},
		{&merged.ContextUsed, next.ContextUsed}, {&merged.ContextSize, next.ContextSize},
	} {
		if pair.value != nil {
			*pair.target = pair.value
		}
	}
	if next.CostAmount != nil {
		merged.CostAmount = next.CostAmount
	}
	if next.CostCurrency != nil {
		merged.CostCurrency = next.CostCurrency
	}
	return &merged
}
