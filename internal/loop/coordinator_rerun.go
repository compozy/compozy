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
	rerun := make(map[generationOutputKey]struct{})
	found := false
	for _, output := range current {
		if output.NodeID != string(nodeID) || (itemIndex != nil && output.ItemIndex != *itemIndex) {
			continue
		}
		found = true
		if GenerationOutputStatusParked(output.Status) || !generationOutputSettled(output.Status) {
			return nil, nil, reasonError(
				ReasonCodeRerunNodeUnsettled,
				ErrRerunNodeUnsettled,
				map[string]string{"node_id": output.NodeID},
			)
		}
		rerun[generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}] = struct{}{}
	}
	if !found {
		return nil, nil, reasonError(
			ReasonCodeRerunNodeUnsettled,
			ErrRerunNodeUnsettled,
			map[string]string{"node_id": string(nodeID)},
		)
	}
	addTransitiveDependents(graph, current, rerun)
	next := reattemptGenerationOutputs(graph, current, rerun, nextGeneration)
	labels := make([]string, 0, len(rerun))
	for key := range rerun {
		labels = append(labels, fmt.Sprintf("%s[%d]", key.nodeID, key.itemIndex))
	}
	sort.Strings(labels)
	return next, labels, nil
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
