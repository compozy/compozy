package loop

import "github.com/compozy/compozy/internal/loop/dsl"

func excludedRoutes(
	nodes []dsl.Node,
	causes []RouteCause,
	pruned map[string]bool,
	generation int,
) map[string]bool {
	excluded := make(map[string]bool, len(pruned))
	for key, isPruned := range pruned {
		if isPruned {
			excluded[key] = true
		}
	}
	for _, cause := range causes {
		if cause.Generation != int64(generation) {
			continue
		}
		for _, node := range nodes {
			if node.ID != cause.NodeID {
				continue
			}
			for _, route := range node.Routes {
				if route.To != cause.Route {
					excluded[rosterKey(generation, route.To, cause.ItemIndex)] = true
				}
			}
			if node.Default != "" && node.Default != cause.Route {
				excluded[rosterKey(generation, node.Default, cause.ItemIndex)] = true
			}
			break
		}
	}
	return excluded
}

func flattenGraphNodes(graph dsl.Graph) []dsl.Node {
	nodes := []dsl.Node{}
	var visit func(dsl.Graph)
	visit = func(current dsl.Graph) {
		for _, node := range current.Nodes {
			nodes = append(nodes, node)
			if node.Body != nil {
				visit(*node.Body)
			}
		}
	}
	visit(graph)
	return nodes
}
