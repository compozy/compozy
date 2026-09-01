package loop

import (
	"fmt"
	"slices"
)

func rosterKey(generation int, nodeID NodeID, itemIndex int) string {
	return fmt.Sprintf("%d/%s/%d", generation, nodeID, itemIndex)
}

func rosterNodeKey(generation int, nodeID NodeID) string {
	return fmt.Sprintf("%d/%s", generation, nodeID)
}

func indexOutputs(items []GenerationOutput) (map[string]*GenerationOutput, map[string][]int) {
	indexed := map[string]*GenerationOutput{}
	itemIndexes := map[string][]int{}
	for index := range items {
		item := &items[index]
		indexed[rosterKey(item.Generation, NodeID(item.NodeID), item.ItemIndex)] = item
		key := rosterNodeKey(item.Generation, NodeID(item.NodeID))
		itemIndexes[key] = append(itemIndexes[key], item.ItemIndex)
	}
	for key := range itemIndexes {
		slices.Sort(itemIndexes[key])
	}
	return indexed, itemIndexes
}

func indexAttempts(items []NodeAttempt) map[string][]NodeAttempt {
	indexed := map[string][]NodeAttempt{}
	for _, item := range items {
		key := rosterKey(item.Generation, item.NodeID, item.ItemIndex)
		indexed[key] = append(indexed[key], item)
	}
	return indexed
}

func indexControls(items []NodeControl) map[NodeID]*NodeControl {
	indexed := map[NodeID]*NodeControl{}
	for index := range items {
		indexed[items[index].NodeID] = &items[index]
	}
	return indexed
}

func indexWaits(items []NodeWait) map[string]*NodeWait {
	indexed := map[string]*NodeWait{}
	for index := range items {
		item := &items[index]
		indexed[rosterKey(item.Generation, item.NodeID, item.ItemIndex)] = item
	}
	return indexed
}
