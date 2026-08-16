package loop

import "github.com/compozy/compozy/internal/loop/dsl"

type fanOutWindowState struct {
	Total        int
	MaxParallel  int
	Materialized map[int]bool
	Settled      map[int]bool
}

func nextFanOutWindowIndexes(state fanOutWindowState) []int {
	if state.Total <= 0 || state.MaxParallel <= 0 {
		return nil
	}
	active := 0
	for itemIndex := range state.Total {
		if state.Materialized[itemIndex] && !state.Settled[itemIndex] {
			active++
		}
	}
	available := min(max(state.MaxParallel-active, 0), state.Total)
	indexes := make([]int, 0, available)
	for itemIndex := 0; itemIndex < state.Total && len(indexes) < available; itemIndex++ {
		if state.Materialized[itemIndex] {
			continue
		}
		indexes = append(indexes, itemIndex)
	}
	return indexes
}

func materializeFanOutWindow(
	graph dsl.Graph,
	topology controlTopology,
	generation int,
	fanOutID dsl.NodeID,
	materialization fanOutMaterialization,
	outputs *[]GenerationOutput,
) bool {
	scope := topology.fanOutScopes[fanOutID]
	materialized := make(map[int]bool)
	settled := make(map[int]bool)
	for _, output := range *outputs {
		if _, inScope := scope.body[dsl.NodeID(output.NodeID)]; inScope {
			materialized[output.ItemIndex] = true
		}
	}
	for itemIndex := range materialization.Branches {
		if materialized[itemIndex] && branchComplete(graph, topology, *outputs, fanOutID, itemIndex) {
			settled[itemIndex] = true
		}
	}
	indexes := nextFanOutWindowIndexes(fanOutWindowState{
		Total: materialization.Branches, MaxParallel: materialization.MaxParallel,
		Materialized: materialized, Settled: settled,
	})
	if len(indexes) == 0 {
		return false
	}
	outputIndexes := generationOutputIndexMap(*outputs)
	for _, itemIndex := range indexes {
		for _, node := range graph.Nodes {
			if _, inScope := scope.body[node.ID]; !inScope {
				continue
			}
			key := generationOutputKey{nodeID: string(node.ID), itemIndex: itemIndex}
			if _, exists := outputIndexes[key]; exists {
				continue
			}
			outputIndexes[key] = len(*outputs)
			*outputs = append(*outputs, GenerationOutput{
				Generation: generation, NodeID: string(node.ID), ItemIndex: itemIndex,
				Status: generationOutputPending, Attempt: 1,
			})
		}
	}
	return true
}

func advanceFanOutWindows(
	graph dsl.Graph,
	topology controlTopology,
	generation int,
	outputs *[]GenerationOutput,
) (bool, error) {
	changed := false
	for _, node := range graph.Nodes {
		if !isControlKind(node, dsl.ControlFanOut) {
			continue
		}
		output, ok := generationOutputMap(*outputs)[generationOutputKey{
			nodeID: string(node.ID), itemIndex: 0,
		}]
		if !ok || output.Status != generationOutputSucceeded {
			continue
		}
		if outputRefRepresentsAbsentValue(output.OutputRef) {
			continue
		}
		materialization, ok, err := parseFanOutMaterialization(generationOutputRuntimePayload(output))
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		if materializeFanOutWindow(graph, topology, generation, node.ID, materialization, outputs) {
			changed = true
		}
	}
	return changed, nil
}
