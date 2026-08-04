package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

const waitExpiryRouteOutputRefPrefix = "wait_expired_route:"

// WaitExpiryRouteOutputRef records the authored timeout branch selected by the due scanner.
func WaitExpiryRouteOutputRef(route string) string {
	return waitExpiryRouteOutputRefPrefix + strings.TrimSpace(route)
}

// WaitExpiryOrigin identifies a coordinator wake caused by authored wait expiry.
func WaitExpiryOrigin(route string) task.Origin {
	return task.Origin{Kind: task.OriginKindDaemon, Ref: "loop.wait-expiry:" + strings.TrimSpace(route)}
}

func applyWaitExpiryRoutes(
	graph dsl.Graph,
	topology controlTopology,
	outputs *[]GenerationOutput,
) {
	for _, output := range append([]GenerationOutput(nil), (*outputs)...) {
		route, found := strings.CutPrefix(strings.TrimSpace(output.OutputRef), waitExpiryRouteOutputRefPrefix)
		if !found || strings.TrimSpace(route) == "" {
			continue
		}
		node, ok := graphNode(graph, dsl.NodeID(output.NodeID))
		if !ok || !isControlKind(node, dsl.ControlWait) {
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
