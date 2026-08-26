package loop

import (
	"fmt"
	"sort"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func planOperatorRerun(
	graph dsl.Graph,
	current []GenerationOutput,
	nodeID NodeID,
	itemIndex *int,
	nextGeneration int,
) ([]GenerationOutput, []string, error) {
	rerun, err := operatorRerunSet(graph, current, nodeID, itemIndex)
	if err != nil {
		return nil, nil, err
	}
	next := reattemptGenerationOutputs(graph, current, rerun, nextGeneration)
	labels := make([]string, 0, len(rerun))
	for key := range rerun {
		labels = append(labels, generationOutputLabel(key))
	}
	sort.Strings(labels)
	return next, labels, nil
}

type operatorRerunCursor struct {
	nodeID     dsl.NodeID
	itemIndex  int
	laneScoped bool
}

func operatorRerunSet(
	graph dsl.Graph,
	current []GenerationOutput,
	nodeID NodeID,
	itemIndex *int,
) (map[generationOutputKey]struct{}, error) {
	topology := newControlTopology(graph)
	rerun := make(map[generationOutputKey]struct{})
	queue := make([]operatorRerunCursor, 0, len(current))
	found := false
	for _, output := range current {
		if output.NodeID != string(nodeID) || (itemIndex != nil && output.ItemIndex != *itemIndex) {
			continue
		}
		found = true
		if GenerationOutputStatusParked(output.Status) || !generationOutputSettled(output.Status) {
			return nil, reasonError(
				ReasonCodeRerunNodeUnsettled,
				ErrRerunNodeUnsettled,
				map[string]string{metadataNodeIDKey: output.NodeID},
			)
		}
		key := generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}
		rerun[key] = struct{}{}
		_, laneScoped := topology.inFanOutBody(dsl.NodeID(output.NodeID))
		queue = append(queue, operatorRerunCursor{
			nodeID: dsl.NodeID(output.NodeID), itemIndex: output.ItemIndex, laneScoped: laneScoped,
		})
	}
	if !found {
		return nil, reasonError(
			ReasonCodeRerunNodeUnsettled,
			ErrRerunNodeUnsettled,
			map[string]string{"node_id": string(nodeID)},
		)
	}
	visited := make(map[operatorRerunCursor]struct{}, len(queue))
	for len(queue) > 0 {
		cursor := queue[0]
		queue = queue[1:]
		if _, ok := visited[cursor]; ok {
			continue
		}
		visited[cursor] = struct{}{}
		for _, dependent := range topology.dependents[cursor.nodeID] {
			matches := operatorDependentOutputs(topology, current, cursor, dependent)
			if len(matches) == 0 {
				queue = append(queue, virtualOperatorRerunCursor(topology, cursor, dependent))
				continue
			}
			for _, output := range matches {
				if GenerationOutputStatusParked(output.Status) {
					continue
				}
				key := generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}
				rerun[key] = struct{}{}
				_, laneScoped := topology.inFanOutBody(dependent)
				queue = append(queue, operatorRerunCursor{
					nodeID: dependent, itemIndex: output.ItemIndex, laneScoped: laneScoped,
				})
			}
		}
	}
	return rerun, nil
}

func operatorDependentOutputs(
	topology controlTopology,
	outputs []GenerationOutput,
	cursor operatorRerunCursor,
	dependent dsl.NodeID,
) []GenerationOutput {
	_, dependentInBody := topology.inFanOutBody(dependent)
	sameBody := topology.sameFanOutBody(cursor.nodeID, dependent)
	matches := make([]GenerationOutput, 0)
	for _, output := range outputs {
		if output.NodeID != string(dependent) {
			continue
		}
		if cursor.laneScoped && sameBody && output.ItemIndex != cursor.itemIndex {
			continue
		}
		if !dependentInBody && output.ItemIndex != 0 {
			continue
		}
		matches = append(matches, output)
	}
	return matches
}

func virtualOperatorRerunCursor(
	topology controlTopology,
	cursor operatorRerunCursor,
	dependent dsl.NodeID,
) operatorRerunCursor {
	sameBody := topology.sameFanOutBody(cursor.nodeID, dependent)
	if cursor.laneScoped && sameBody {
		return operatorRerunCursor{nodeID: dependent, itemIndex: cursor.itemIndex, laneScoped: true}
	}
	return operatorRerunCursor{nodeID: dependent}
}

func generationOutputLabel(key generationOutputKey) string {
	return fmt.Sprintf("%s[%d]", key.nodeID, key.itemIndex)
}

func generationOutputSettled(status string) bool {
	switch status {
	case generationOutputSucceeded,
		generationOutputPartial,
		generationOutputFailed,
		generationOutputCanceled:
		return true
	default:
		return false
	}
}
