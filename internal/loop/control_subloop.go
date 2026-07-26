package loop

import (
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
)

const subLoopNodeSeparator = "__"

type controlGraphExpansion struct {
	graph     dsl.Graph
	starts    []dsl.NodeID
	terminals []dsl.NodeID
}

type expandedNodeRef struct {
	id        dsl.NodeID
	terminals []dsl.NodeID
}

func controlExecutionResolved(resolved *ResolvedDefinition) (*ResolvedDefinition, error) {
	if resolved == nil {
		return nil, fmt.Errorf("%w: resolved loop definition is required", ErrValidation)
	}
	graph, err := expandControlExecutionGraph(resolved.Definition.Graph)
	if err != nil {
		return nil, err
	}
	execution := *resolved
	execution.Definition = resolved.Definition
	execution.Definition.Graph = graph
	return &execution, nil
}

func expandControlExecutionGraph(graph dsl.Graph) (dsl.Graph, error) {
	expanded, err := expandControlGraph(graph, "")
	if err != nil {
		return dsl.Graph{}, err
	}
	if err := validateControlExecutionNodeIDs(expanded.graph); err != nil {
		return dsl.Graph{}, err
	}
	return expanded.graph, nil
}

func expandControlGraph(graph dsl.Graph, prefix string) (controlGraphExpansion, error) {
	incoming := map[dsl.NodeID]int{}
	outgoing := map[dsl.NodeID]int{}
	for _, node := range graph.Nodes {
		incoming[node.ID] = 0
		outgoing[node.ID] = 0
	}
	for _, edge := range graph.Edges {
		incoming[edge.To]++
		outgoing[edge.From]++
	}

	expanded := controlGraphExpansion{}
	nodeRefs := make(map[dsl.NodeID]expandedNodeRef, len(graph.Nodes))
	for _, node := range graph.Nodes {
		qualifiedID := qualifySubLoopNodeID(prefix, node.ID)
		copied := node
		copied.ID = qualifiedID
		copied.Body = nil
		ref := expandedNodeRef{id: qualifiedID, terminals: []dsl.NodeID{qualifiedID}}
		expanded.graph.Nodes = append(expanded.graph.Nodes, copied)
		if isControlKind(node, dsl.ControlSubLoop) && node.Body != nil {
			body, err := expandControlGraph(*node.Body, string(qualifiedID))
			if err != nil {
				return controlGraphExpansion{}, err
			}
			expanded.graph.Nodes = append(expanded.graph.Nodes, body.graph.Nodes...)
			expanded.graph.Edges = append(expanded.graph.Edges, body.graph.Edges...)
			for _, start := range body.starts {
				expanded.graph.Edges = append(expanded.graph.Edges, dsl.Edge{
					From: qualifiedID,
					To:   start,
				})
			}
			ref.terminals = body.terminals
		}
		nodeRefs[node.ID] = ref
	}

	for _, edge := range graph.Edges {
		fromRef, fromOK := nodeRefs[edge.From]
		toRef, toOK := nodeRefs[edge.To]
		if !fromOK || !toOK {
			return controlGraphExpansion{}, fmt.Errorf(
				"%w: sub-loop execution edge references unknown node %s -> %s",
				ErrValidation,
				edge.From,
				edge.To,
			)
		}
		for _, from := range fromRef.terminals {
			expanded.graph.Edges = append(expanded.graph.Edges, dsl.Edge{
				From:  from,
				To:    toRef.id,
				Extra: cloneStringAnyMap(edge.Extra),
			})
		}
	}

	for _, node := range graph.Nodes {
		ref := nodeRefs[node.ID]
		if incoming[node.ID] == 0 {
			expanded.starts = append(expanded.starts, ref.id)
		}
		if outgoing[node.ID] == 0 {
			expanded.terminals = append(expanded.terminals, ref.terminals...)
		}
	}
	return expanded, nil
}

func qualifySubLoopNodeID(prefix string, nodeID dsl.NodeID) dsl.NodeID {
	if prefix == "" {
		return nodeID
	}
	return dsl.NodeID(prefix + subLoopNodeSeparator + string(nodeID))
}

func validateControlExecutionNodeIDs(graph dsl.Graph) error {
	seen := make(map[dsl.NodeID]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if _, ok := seen[node.ID]; ok {
			return fmt.Errorf(
				"%w: sub-loop execution node id %q collides with an authored node id",
				ErrValidation,
				node.ID,
			)
		}
		seen[node.ID] = struct{}{}
	}
	return nil
}

func cloneStringAnyMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneAny(value)
	}
	return cloned
}
