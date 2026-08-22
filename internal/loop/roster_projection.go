package loop

import (
	"sort"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func rosterGenerations(source *RosterSource) []int {
	if len(source.Generations) == 0 {
		return []int{source.Run.Generation}
	}
	generations := make([]int, 0, len(source.Generations))
	for _, item := range source.Generations {
		generations = append(generations, int(item.Generation))
	}
	sort.Ints(generations)
	return generations
}

func rosterItems(
	runID RunID,
	generation int,
	node dsl.Node,
	outputs map[string]*GenerationOutput,
	maxOutputItems map[string]int,
	attempts map[string][]NodeAttempt,
	controls map[NodeID]*NodeControl,
	waits map[string]*NodeWait,
	excluded map[string]bool,
) []RosterNode {
	maxItem, ok := maxOutputItems[rosterNodeKey(generation, node.ID)]
	if !ok || maxItem < 1 {
		return nil
	}
	items := make([]RosterNode, 0, maxItem+1)
	for itemIndex := 0; itemIndex <= maxItem; itemIndex++ {
		key := rosterKey(generation, node.ID, itemIndex)
		items = append(items, newRosterNode(
			runID, generation, node, itemIndex, outputs[key], attempts[key],
			controls[node.ID], waits[key], excluded[key],
		))
	}
	return items
}

func filterRosterNodes(items []RosterNode, state NodeStateFilter) []RosterNode {
	if state == NodeStateFilterAll {
		return items
	}
	filtered := make([]RosterNode, 0, len(items))
	for _, item := range items {
		if string(item.State) == string(state) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
