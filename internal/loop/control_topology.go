package loop

import (
	"slices"

	"github.com/compozy/agh/internal/loop/dsl"
)

type controlTopology struct {
	dependencies map[dsl.NodeID][]dsl.NodeID
	dependents   map[dsl.NodeID][]dsl.NodeID
	nodeFanOut   map[dsl.NodeID]dsl.NodeID
	collectScope map[dsl.NodeID]dsl.NodeID
	fanOutScopes map[dsl.NodeID]fanOutScope
}

type fanOutScope struct {
	fanOut  dsl.NodeID
	body    map[dsl.NodeID]struct{}
	collect map[dsl.NodeID]struct{}
}

func newControlTopology(graph dsl.Graph) controlTopology {
	topology := controlTopology{
		dependencies: graphDependencies(graph),
		dependents:   graphDependents(graph),
		nodeFanOut:   map[dsl.NodeID]dsl.NodeID{},
		collectScope: map[dsl.NodeID]dsl.NodeID{},
		fanOutScopes: map[dsl.NodeID]fanOutScope{},
	}
	for _, node := range graph.Nodes {
		if !isControlKind(node, dsl.ControlFanOut) {
			continue
		}
		scope := topology.buildFanOutScope(graph, node.ID)
		topology.fanOutScopes[node.ID] = scope
		for bodyNode := range scope.body {
			if _, exists := topology.nodeFanOut[bodyNode]; !exists {
				topology.nodeFanOut[bodyNode] = node.ID
			}
		}
		for collectNode := range scope.collect {
			if _, exists := topology.collectScope[collectNode]; !exists {
				topology.collectScope[collectNode] = node.ID
			}
		}
	}
	return topology
}

func (t controlTopology) buildFanOutScope(graph dsl.Graph, fanOut dsl.NodeID) fanOutScope {
	scope := fanOutScope{
		fanOut:  fanOut,
		body:    map[dsl.NodeID]struct{}{},
		collect: map[dsl.NodeID]struct{}{},
	}
	visited := map[dsl.NodeID]struct{}{fanOut: {}}
	queue := append([]dsl.NodeID(nil), t.dependents[fanOut]...)
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if _, ok := visited[nodeID]; ok {
			continue
		}
		visited[nodeID] = struct{}{}
		node, ok := graphNode(graph, nodeID)
		if !ok {
			continue
		}
		if isControlKind(node, dsl.ControlCollect) {
			scope.collect[nodeID] = struct{}{}
			continue
		}
		scope.body[nodeID] = struct{}{}
		queue = append(queue, t.dependents[nodeID]...)
	}
	return scope
}

func (t controlTopology) inFanOutBody(nodeID dsl.NodeID) (dsl.NodeID, bool) {
	fanOut, ok := t.nodeFanOut[nodeID]
	return fanOut, ok
}

func (t controlTopology) isCollectForFanOut(nodeID dsl.NodeID) (dsl.NodeID, bool) {
	fanOut, ok := t.collectScope[nodeID]
	return fanOut, ok
}

func (t controlTopology) sameFanOutBody(left dsl.NodeID, right dsl.NodeID) bool {
	leftFanOut, leftOK := t.inFanOutBody(left)
	rightFanOut, rightOK := t.inFanOutBody(right)
	return leftOK && rightOK && leftFanOut == rightFanOut
}

func isControlKind(node dsl.Node, kind dsl.ControlKind) bool {
	return node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == kind
}

func isCoordinatorOwnedControl(node dsl.Node) bool {
	if node.Class != dsl.NodeClassControl {
		return false
	}
	switch dsl.ControlKind(node.Kind) {
	case dsl.ControlFanOut, dsl.ControlCollect, dsl.ControlBranch, dsl.ControlSubLoop:
		return true
	default:
		return false
	}
}

func isCoordinatorOwnedNode(node dsl.Node) bool {
	return isCoordinatorOwnedControl(node) ||
		isInputSourceNode(node) ||
		isWatchSourceNode(node) ||
		isWatchEventsNode(node) ||
		isFileImportSourceNode(node)
}

func isCoordinatorOwnedNodeWithGates(node dsl.Node, gatesEnabled bool) bool {
	if gatesEnabled && isControlKind(node, dsl.ControlGate) {
		return true
	}
	return isCoordinatorOwnedNode(node)
}

func sortedGenerationOutputs(outputs []GenerationOutput) []GenerationOutput {
	ordered := append([]GenerationOutput(nil), outputs...)
	slices.SortFunc(ordered, func(left GenerationOutput, right GenerationOutput) int {
		if left.NodeID < right.NodeID {
			return -1
		}
		if left.NodeID > right.NodeID {
			return 1
		}
		if left.ItemIndex < right.ItemIndex {
			return -1
		}
		if left.ItemIndex > right.ItemIndex {
			return 1
		}
		return 0
	})
	return ordered
}
