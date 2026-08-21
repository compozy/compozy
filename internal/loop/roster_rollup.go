package loop

import "sort"

func buildFanoutRollups(items []RosterNode) []FanoutRollup {
	groups := map[string]*FanoutRollup{}
	for _, item := range items {
		key := rosterNodeKey(item.Generation, item.NodeID)
		rollup := groups[key]
		if rollup == nil {
			rollup = &FanoutRollup{Generation: item.Generation, NodeID: item.NodeID}
			groups[key] = rollup
		}
		rollup.Total++
		if rosterNodeIsDone(item.State) {
			rollup.Done++
		}
		if item.State == NodeStateFailed {
			rollup.Failed++
		}
	}
	rollups := make([]FanoutRollup, 0, len(groups))
	for _, rollup := range groups {
		if rollup.Total > 1 {
			rollups = append(rollups, *rollup)
		}
	}
	sort.Slice(rollups, func(i, j int) bool {
		if rollups[i].Generation != rollups[j].Generation {
			return rollups[i].Generation < rollups[j].Generation
		}
		return rollups[i].NodeID < rollups[j].NodeID
	})
	return rollups
}

func rosterNodeIsDone(state NodeState) bool {
	switch state {
	case NodeStateSucceeded, NodeStatePartial, NodeStateFailed, NodeStateCanceled, NodeStateNotTaken:
		return true
	default:
		return false
	}
}
