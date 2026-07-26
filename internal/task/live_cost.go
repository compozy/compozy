package task

import "github.com/compozy/agh/internal/store"

func mergeTokenStatsSummary(summary *RunOperationalSummary, stats []store.TokenStats) {
	if summary == nil {
		return
	}

	var (
		inputTotal  int64
		outputTotal int64
		totalTotal  int64
		turnCount   int64
		hasInput    bool
		hasOutput   bool
		hasTotal    bool
	)
	for index := range stats {
		stat := stats[index]
		if stat.UpdatedAt.After(summary.LastActivityAt) {
			summary.LastActivityAt = stat.UpdatedAt
		}
		if stat.InputTokens != nil {
			inputTotal += *stat.InputTokens
			hasInput = true
		}
		if stat.OutputTokens != nil {
			outputTotal += *stat.OutputTokens
			hasOutput = true
		}
		if stat.TotalTokens != nil {
			totalTotal += *stat.TotalTokens
			hasTotal = true
		}
		turnCount += stat.TurnCount
	}

	if hasInput {
		summary.InputTokens = &inputTotal
	}
	if hasOutput {
		summary.OutputTokens = &outputTotal
	}
	if hasTotal {
		summary.TotalTokens = &totalTotal
	}
	if turnCount > 0 {
		summary.TurnCount = &turnCount
	}

	cost := store.AggregateTokenStatsCost(stats)
	summary.TotalCost = cost.TotalCost
	summary.CostCurrency = cost.Currency
	summary.CostStatus = cost.Status
	summary.CostSource = cost.Source
}
