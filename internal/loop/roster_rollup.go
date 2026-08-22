package loop

import (
	"sort"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func buildFanoutRollups(items []RosterNode, graph dsl.Graph) []FanoutRollup {
	targets, containers := fanoutTargets(graph)
	branchGroups := make(map[string]bool)
	for _, item := range items {
		if container, ok := targets[item.NodeID]; ok {
			branchGroups[rosterNodeKey(item.Generation, container)] = true
		}
	}
	groups := map[string]*FanoutRollup{}
	for _, item := range items {
		rollupNodeID := item.NodeID
		if container, ok := targets[item.NodeID]; ok {
			rollupNodeID = container
		} else if containers[item.NodeID] && branchGroups[rosterNodeKey(item.Generation, item.NodeID)] {
			// The control row describes the authored fan-out itself. Its direct
			// branch rows own the rollup denominator, so counting both would add a
			// fictional worker.
			continue
		}
		key := rosterNodeKey(item.Generation, rollupNodeID)
		rollup := groups[key]
		if rollup == nil {
			rollup = &FanoutRollup{Generation: item.Generation, NodeID: rollupNodeID}
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

func fanoutTargets(graph dsl.Graph) (map[NodeID]NodeID, map[NodeID]bool) {
	targets := make(map[NodeID]NodeID)
	containers := make(map[NodeID]bool)
	var visit func(dsl.Graph)
	visit = func(current dsl.Graph) {
		for _, node := range current.Nodes {
			if node.Kind == string(dsl.ControlFanOut) {
				containers[node.ID] = true
				for _, edge := range current.Edges {
					if edge.From == node.ID {
						targets[edge.To] = node.ID
					}
				}
			}
			if node.Body != nil {
				visit(*node.Body)
			}
		}
	}
	visit(graph)
	return targets, containers
}

func rosterNodeIsDone(state NodeState) bool {
	switch state {
	case NodeStateSucceeded, NodeStatePartial, NodeStateFailed, NodeStateCanceled, NodeStateNotTaken:
		return true
	default:
		return false
	}
}
