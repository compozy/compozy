package loop

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func rosterKey(generation int, nodeID NodeID, itemIndex int) string {
	return fmt.Sprintf("%d/%s/%d", generation, nodeID, itemIndex)
}

func indexOutputs(items []GenerationOutput) map[string]*GenerationOutput {
	indexed := map[string]*GenerationOutput{}
	for index := range items {
		item := &items[index]
		indexed[rosterKey(item.Generation, NodeID(item.NodeID), item.ItemIndex)] = item
	}
	return indexed
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

func excludedRoutes(
	graph dsl.Graph,
	causes []RouteCause,
	pruned map[string]bool,
	generation int,
) map[NodeID]bool {
	excluded := map[NodeID]bool{}
	for _, node := range flattenGraphNodes(graph) {
		if pruned[rosterKey(generation, node.ID, 0)] {
			excluded[node.ID] = true
		}
	}
	for _, cause := range causes {
		if cause.Generation != int64(generation) {
			continue
		}
		for _, node := range flattenGraphNodes(graph) {
			if node.ID != cause.NodeID {
				continue
			}
			for _, route := range node.Routes {
				if route.To != cause.Route {
					excluded[route.To] = true
				}
			}
			if node.Default != "" && node.Default != cause.Route {
				excluded[node.Default] = true
			}
		}
	}
	return excluded
}

func rosterGenerations(source *RosterSource) []int {
	if len(source.Generations) == 0 {
		return []int{source.Run.Generation}
	}
	generations := make([]int, 0, len(source.Generations))
	for _, item := range source.Generations {
		generations = append(generations, int(item.Generation))
	}
	sort.Ints(generations)
	return generations
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

func rosterItems(
	runID RunID,
	generation int,
	node dsl.Node,
	outputs map[string]*GenerationOutput,
	attempts map[string][]NodeAttempt,
	controls map[NodeID]*NodeControl,
	waits map[string]*NodeWait,
	excluded map[NodeID]bool,
) []RosterNode {
	maxItem := -1
	prefix := fmt.Sprintf("%d/%s/", generation, node.ID)
	for key, output := range outputs {
		if strings.HasPrefix(key, prefix) && output.ItemIndex > maxItem {
			maxItem = output.ItemIndex
		}
	}
	if maxItem < 1 {
		return nil
	}
	items := make([]RosterNode, 0, maxItem+1)
	for itemIndex := 0; itemIndex <= maxItem; itemIndex++ {
		key := rosterKey(generation, node.ID, itemIndex)
		items = append(items, newRosterNode(
			runID,
			generation,
			node,
			itemIndex,
			outputs[key],
			attempts[key],
			controls[node.ID],
			waits[key],
			excluded[node.ID],
		))
	}
	return items
}

func filterRosterNodes(items []RosterNode, state NodeStateFilter) []RosterNode {
	if state == NodeStateFilterAll {
		return items
	}
	filtered := make([]RosterNode, 0, len(items))
	for _, item := range items {
		if string(item.State) == string(state) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func buildFanoutRollups(items []RosterNode) []FanoutRollup {
	groups := map[string]*FanoutRollup{}
	for _, item := range items {
		key := fmt.Sprintf("%d/%s", item.Generation, item.NodeID)
		rollup := groups[key]
		if rollup == nil {
			rollup = &FanoutRollup{Generation: item.Generation, NodeID: item.NodeID}
			groups[key] = rollup
		}
		rollup.Total++
		if rosterNodeIsDone(item.State) {
			rollup.Done++
		}
		if item.State == NodeStateFailed {
			rollup.Failed++
		}
	}
	rollups := make([]FanoutRollup, 0, len(groups))
	for _, rollup := range groups {
		if rollup.Total > 1 {
			rollups = append(rollups, *rollup)
		}
	}
	sort.Slice(rollups, func(i, j int) bool {
		if rollups[i].Generation != rollups[j].Generation {
			return rollups[i].Generation < rollups[j].Generation
		}
		return rollups[i].NodeID < rollups[j].NodeID
	})
	return rollups
}

func rosterNodeIsDone(state NodeState) bool {
	switch state {
	case NodeStateSucceeded, NodeStatePartial, NodeStateFailed, NodeStateCanceled, NodeStateNotTaken:
		return true
	default:
		return false
	}
}

func encodeRosterCursor(cursor rosterCursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode roster cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeRosterCursor(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, fmt.Errorf("%w: malformed roster cursor", ErrInvalidRosterCursor)
	}
	var cursor rosterCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.Offset < 0 {
		return 0, fmt.Errorf("%w: malformed roster cursor", ErrInvalidRosterCursor)
	}
	return cursor.Offset, nil
}
