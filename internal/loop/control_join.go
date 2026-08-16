package loop

import (
	"math"
	"slices"

	"github.com/compozy/compozy/internal/loop/dsl"
)

type joinSettlementState string

const (
	joinSettlementPending   joinSettlementState = "pending"
	joinSettlementSucceeded joinSettlementState = "succeeded"
	joinSettlementPartial   joinSettlementState = "partial"
	joinSettlementFailed    joinSettlementState = "failed"
)

type joinLaneState struct {
	ItemIndex  int
	Status     string
	OutputRef  string
	Definitive bool
}

// CollectOutput is the stable coverage value produced by a strategy-aware collect cell.
type CollectOutput struct {
	Total        int     `json:"total"`
	Succeeded    int     `json:"succeeded"`
	Failed       int     `json:"failed"`
	Canceled     int     `json:"canceled"`
	CoverageRate float64 `json:"coverage_rate"`
	Partial      bool    `json:"partial"`
}

type joinSettlement struct {
	State       joinSettlementState
	Coverage    CollectOutput
	CancelItems []int
	TriggerItem *int
	WinnerItem  *int
	WinnerRef   string
}

func settleJoin(
	strategy dsl.StrategySpec,
	total int,
	lanes []joinLaneState,
	prior *joinSettlement,
) joinSettlement {
	if prior != nil && prior.State != joinSettlementPending {
		return cloneJoinSettlement(*prior)
	}
	ordered := append([]joinLaneState(nil), lanes...)
	slices.SortFunc(ordered, func(left, right joinLaneState) int { return left.ItemIndex - right.ItemIndex })
	settlement := joinSettlement{State: joinSettlementPending}
	settlement.Coverage.Total = max(total, 0)
	terminal := 0
	for _, lane := range ordered {
		switch lane.Status {
		case generationOutputSucceeded:
			settlement.Coverage.Succeeded++
			terminal++
		case generationOutputCanceled:
			settlement.Coverage.Canceled++
			terminal++
		case generationOutputFailed:
			if lane.Definitive {
				settlement.Coverage.Failed++
				terminal++
			}
		}
	}
	if total == 0 {
		settlement.State = joinSettlementSucceeded
		return settlement
	}
	settlement.Coverage.CoverageRate = coverageRate(settlement.Coverage.Succeeded, total)
	kind := strategy.Kind
	if kind == "" {
		kind = dsl.StrategyWaitAll
	}
	switch kind {
	case dsl.StrategyFailFast:
		for _, lane := range ordered {
			if lane.Status == generationOutputFailed && lane.Definitive {
				settlement.State = joinSettlementFailed
				settlement.TriggerItem = intPointer(lane.ItemIndex)
				settlement.CancelItems = unsettledLaneItems(ordered, lane.ItemIndex)
				return settlement
			}
		}
		if settlement.Coverage.Succeeded == total {
			settlement.State = joinSettlementSucceeded
		} else if terminal == total {
			settlement.State = joinSettlementFailed
		}
	case dsl.StrategyBestEffort:
		required := requiredStrategySuccesses(strategy.Threshold, total)
		if settlement.Coverage.Succeeded >= required {
			settlement.State = joinSettlementSucceeded
			if settlement.Coverage.Succeeded < total {
				settlement.State = joinSettlementPartial
				settlement.Coverage.Partial = true
			}
			settlement.CancelItems = unsettledLaneItems(ordered, -1)
		} else if terminal == total {
			settlement.State = joinSettlementFailed
		}
	case dsl.StrategyRace:
		for _, lane := range ordered {
			if lane.Status == generationOutputSucceeded {
				settlement.State = joinSettlementSucceeded
				settlement.WinnerItem = intPointer(lane.ItemIndex)
				settlement.WinnerRef = lane.OutputRef
				settlement.CancelItems = unsettledLaneItems(ordered, lane.ItemIndex)
				return settlement
			}
		}
		if terminal == total {
			settlement.State = joinSettlementFailed
		}
	default:
		if terminal != total {
			return settlement
		}
		if settlement.Coverage.Succeeded == total {
			settlement.State = joinSettlementSucceeded
		} else {
			settlement.State = joinSettlementFailed
		}
	}
	return settlement
}

func requiredStrategySuccesses(threshold *dsl.StrategyThreshold, total int) int {
	if threshold == nil {
		return total
	}
	if threshold.Kind == dsl.ThresholdCount {
		return threshold.Count
	}
	return int(math.Ceil(float64(total) * float64(threshold.Percent) / 100))
}

func coverageRate(succeeded, total int) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(succeeded)/float64(total)*100) / 100
}

func unsettledLaneItems(lanes []joinLaneState, exclude int) []int {
	items := make([]int, 0)
	for _, lane := range lanes {
		if lane.ItemIndex == exclude || lane.Status == generationOutputSucceeded ||
			lane.Status == generationOutputCanceled ||
			lane.Status == generationOutputFailed && lane.Definitive {
			continue
		}
		items = append(items, lane.ItemIndex)
	}
	return items
}

func intPointer(value int) *int { return &value }

func cloneJoinSettlement(value joinSettlement) joinSettlement {
	value.CancelItems = append([]int(nil), value.CancelItems...)
	if value.TriggerItem != nil {
		value.TriggerItem = intPointer(*value.TriggerItem)
	}
	if value.WinnerItem != nil {
		value.WinnerItem = intPointer(*value.WinnerItem)
	}
	return value
}
