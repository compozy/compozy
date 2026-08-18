package loop

import (
	"slices"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func fanOutProgressValue(
	topology controlTopology,
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
) map[string]any {
	total, ok := fanOutBranchCount(outputs, fanOutID)
	if !ok {
		total = 0
	}
	progress := map[string]any{
		"total": int64(total), generationOutputSucceeded: int64(0), "failed": int64(0),
		generationOutputCanceled: int64(0), "running": int64(0), string(joinSettlementPending): int64(total),
		"settled": int64(0), "success_rate": float64(0), "failure_rate": float64(0),
	}
	collectID, ok := firstFanOutCollect(topology, fanOutID)
	if !ok || total == 0 {
		return progress
	}
	var succeeded, failed, canceled, running int
	for itemIndex := range total {
		lane := collectLaneState(topology, outputs, fanOutID, collectID, itemIndex)
		switch lane.Status {
		case generationOutputSucceeded:
			succeeded++
		case generationOutputFailed:
			if lane.Definitive {
				failed++
			}
		case generationOutputCanceled:
			canceled++
		case generationOutputRunning, generationOutputEnqueued:
			running++
		}
	}
	settled := succeeded + failed + canceled
	progress["succeeded"] = int64(succeeded)
	progress["failed"] = int64(failed)
	progress["canceled"] = int64(canceled)
	progress["running"] = int64(running)
	progress["pending"] = int64(max(total-settled-running, 0))
	progress["settled"] = int64(settled)
	progress["success_rate"] = coverageRate(succeeded, total)
	progress["failure_rate"] = coverageRate(failed, total)
	return progress
}

func firstFanOutCollect(topology controlTopology, fanOutID dsl.NodeID) (dsl.NodeID, bool) {
	items := make([]dsl.NodeID, 0, len(topology.fanOutScopes[fanOutID].collect))
	for nodeID := range topology.fanOutScopes[fanOutID].collect {
		items = append(items, nodeID)
	}
	slices.Sort(items)
	if len(items) == 0 {
		return "", false
	}
	return items[0], true
}
