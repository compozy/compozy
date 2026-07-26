package store

import (
	"math"
	"strings"
)

const (
	tokenCostStatusActual    = "actual"
	tokenCostStatusEstimated = "estimated"
	tokenCostStatusIncluded  = "included"
	tokenCostStatusUnknown   = "unknown"

	tokenCostSourceAgentReported = "agent_reported"
	tokenCostSourceCatalogConfig = "catalog_config"
	tokenCostSourceModelsDev     = "models_dev"
	tokenCostSourceBuiltin       = "builtin"
	tokenCostSourceNone          = "none"
)

// TokenStatsCostSummary is the compatible cost projection across token-stat rows.
type TokenStatsCostSummary struct {
	TotalCost *float64
	Currency  *string
	Status    string
	Source    string
}

// AggregateTokenStatsCost sums only rows with identical truthful provenance and currency.
func AggregateTokenStatsCost(stats []TokenStats) TokenStatsCostSummary {
	if len(stats) == 0 {
		return TokenStatsCostSummary{}
	}

	summary := TokenStatsCostSummary{}
	initialized := false
	for index := range stats {
		row, ok := tokenStatsRowCost(stats[index])
		if !ok {
			return unknownTokenStatsCost()
		}
		if !initialized {
			summary = row
			initialized = true
			continue
		}
		if summary.Status != row.Status || summary.Source != row.Source ||
			normalizedCostCurrency(summary.Currency) != normalizedCostCurrency(row.Currency) {
			return unknownTokenStatsCost()
		}
		if row.TotalCost != nil {
			if summary.TotalCost == nil {
				return unknownTokenStatsCost()
			}
			total := *summary.TotalCost + *row.TotalCost
			if !validAggregatedCostAmount(total) {
				return unknownTokenStatsCost()
			}
			summary.TotalCost = &total
		}
	}
	return summary
}

func tokenStatsRowCost(stat TokenStats) (TokenStatsCostSummary, bool) {
	status := strings.TrimSpace(stat.CostStatus)
	source := strings.TrimSpace(stat.CostSource)
	switch status {
	case tokenCostStatusActual:
		if source != tokenCostSourceAgentReported || stat.TotalCost == nil ||
			!validAggregatedCostAmount(*stat.TotalCost) ||
			normalizedCostCurrency(stat.CostCurrency) == "" {
			return TokenStatsCostSummary{}, false
		}
	case tokenCostStatusEstimated:
		if source != tokenCostSourceCatalogConfig &&
			source != tokenCostSourceModelsDev && source != tokenCostSourceBuiltin {
			return TokenStatsCostSummary{}, false
		}
		if stat.TotalCost == nil || !validAggregatedCostAmount(*stat.TotalCost) ||
			normalizedCostCurrency(stat.CostCurrency) == "" {
			return TokenStatsCostSummary{}, false
		}
	case tokenCostStatusIncluded, tokenCostStatusUnknown:
		if source != tokenCostSourceNone || stat.TotalCost != nil || stat.CostCurrency != nil {
			return TokenStatsCostSummary{}, false
		}
	default:
		return TokenStatsCostSummary{}, false
	}

	result := TokenStatsCostSummary{Status: status, Source: source}
	if stat.TotalCost != nil {
		total := *stat.TotalCost
		result.TotalCost = &total
	}
	if currency := normalizedCostCurrency(stat.CostCurrency); currency != "" {
		result.Currency = &currency
	}
	return result, true
}

func validAggregatedCostAmount(amount float64) bool {
	return amount >= 0 && !math.IsNaN(amount) && !math.IsInf(amount, 0)
}

func normalizedCostCurrency(currency *string) string {
	if currency == nil {
		return ""
	}
	return strings.TrimSpace(*currency)
}

func unknownTokenStatsCost() TokenStatsCostSummary {
	return TokenStatsCostSummary{Status: tokenCostStatusUnknown, Source: tokenCostSourceNone}
}
