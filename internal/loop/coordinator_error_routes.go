package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func skipErrorRouteOnSuccess(
	graph dsl.Graph,
	topology controlTopology,
	node dsl.Node,
	output GenerationOutput,
	outputs *[]GenerationOutput,
) {
	errorPolicy := nodeErrorPolicy(node)
	if errorPolicy == nil || errorPolicy.Route == "" {
		return
	}
	skipRouteNodes(
		graph,
		topology,
		output,
		[]dsl.NodeID{errorPolicy.Route},
		outputs,
	)
}

func skipErrorRouteSuccessPath(
	graph dsl.Graph,
	topology controlTopology,
	node dsl.Node,
	output GenerationOutput,
	outputs *[]GenerationOutput,
) {
	errorPolicy := nodeErrorPolicy(node)
	if errorPolicy == nil || errorPolicy.Route == "" {
		return
	}
	starts := make([]dsl.NodeID, 0, len(topology.dependents[node.ID]))
	for _, dependent := range topology.dependents[node.ID] {
		if dependent != errorPolicy.Route {
			starts = append(starts, dependent)
		}
	}
	skipRouteNodes(graph, topology, output, starts, outputs)
}

func skipRouteNodes(
	graph dsl.Graph,
	topology controlTopology,
	source GenerationOutput,
	starts []dsl.NodeID,
	outputs *[]GenerationOutput,
) {
	indexes := generationOutputIndexMap(*outputs)
	outputMap := generationOutputMap(*outputs)
	queue := append([]dsl.NodeID(nil), starts...)
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		_, found := graphNode(graph, nodeID)
		if !found {
			continue
		}
		key := generationOutputKey{
			nodeID:    string(nodeID),
			itemIndex: itemIndexForSkippedNode(topology, nodeID, dsl.NodeID(source.NodeID), source.ItemIndex),
		}
		index, exists := indexes[key]
		if !exists || (*outputs)[index].Status != generationOutputPending {
			continue
		}
		(*outputs)[index].Status = generationOutputSucceeded
		setGenerationOutputRef(&(*outputs)[index], branchSkippedOutputRef)
		outputMap[key] = (*outputs)[index]
		for _, dependent := range topology.dependents[nodeID] {
			if allRouteDependenciesSkipped(topology, outputMap, dependent, source.ItemIndex) {
				queue = append(queue, dependent)
			}
		}
	}
}

func allRouteDependenciesSkipped(
	topology controlTopology,
	outputs map[generationOutputKey]GenerationOutput,
	nodeID dsl.NodeID,
	itemIndex int,
) bool {
	dependencies := topology.dependencies[nodeID]
	if len(dependencies) == 0 {
		return false
	}
	for _, dependency := range dependencies {
		output, found := dependencyOutputForNode(topology, outputs, nodeID, dependency, itemIndex)
		if !found || output.OutputRef != branchSkippedOutputRef {
			return false
		}
	}
	return true
}

func outputRefRepresentsAbsentValue(outputRef string) bool {
	trimmed := strings.TrimSpace(outputRef)
	return trimmed == branchSkippedOutputRef || trimmed == failureAbsorbedOutputRef ||
		strings.HasPrefix(trimmed, errorRoutedOutputRefPrefix) ||
		strings.HasPrefix(trimmed, waitExpiryRouteOutputRefPrefix)
}
