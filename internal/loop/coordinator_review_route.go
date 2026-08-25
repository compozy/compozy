package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const reviewRejectedRouteOutputRefPrefix = "review_rejected_route:"

func applyReviewRejectRoutes(
	graph dsl.Graph,
	topology controlTopology,
	outputs *[]GenerationOutput,
) {
	for _, output := range append([]GenerationOutput(nil), (*outputs)...) {
		if !generationOutputHasKind(output, GenerationResultReviewRejectRouted) {
			continue
		}
		route, found := strings.CutPrefix(strings.TrimSpace(output.OutputRef), reviewRejectedRouteOutputRefPrefix)
		if !found || strings.TrimSpace(route) == "" {
			continue
		}
		node, ok := graphNode(graph, dsl.NodeID(output.NodeID))
		if !ok || node.Class != dsl.NodeClassAction || node.Review == nil || node.Review.OnReject == nil ||
			node.Review.OnReject.Route != dsl.NodeID(route) {
			continue
		}
		starts := make([]dsl.NodeID, 0, len(topology.dependents[node.ID]))
		for _, dependent := range topology.dependents[node.ID] {
			if dependent != dsl.NodeID(route) {
				starts = append(starts, dependent)
			}
		}
		skipRouteNodes(graph, topology, output, starts, outputs)
	}
}
