package loop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func runtimeNamespace(
	run Run,
	generation int,
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	nodeID dsl.NodeID,
	itemIndex int,
) (map[string]any, error) {
	return runtimeNamespaceWithHistory(
		run,
		generation,
		graph,
		topology,
		outputs,
		GenerationHistory{},
		nodeID,
		itemIndex,
	)
}

func runtimeNamespaceWithHistory(
	run Run,
	generation int,
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	history GenerationHistory,
	nodeID dsl.NodeID,
	itemIndex int,
) (map[string]any, error) {
	nodes := map[string]any{}
	namespace := map[string]any{
		namespaceInputsKey:    cloneAnyMap(run.Inputs),
		namespaceNodesKey:     nodes,
		metadataGenerationKey: int64(max(1, generation)),
		namespacePreviousKey:  history.previousNamespace(topology, nodeID, itemIndex),
		namespaceBestKey:      history.bestNamespace(topology, nodeID, itemIndex),
	}
	activeFanOuts := activeFanOutNodes(graph, topology, nodeID)
	for _, fanOut := range activeFanOuts {
		item, exists, err := fanOutItem(outputs, fanOut.ID, itemIndex)
		if err != nil {
			return nil, err
		}
		if exists {
			if name := strings.TrimSpace(fanOut.BindAs); name != "" {
				namespace[name] = item
			}
			if name := strings.TrimSpace(fanOut.IndexAs); name != "" {
				namespace[name] = int64(itemIndex)
			}
		}
	}
	if len(activeFanOuts) > 0 {
		inner := activeFanOuts[len(activeFanOuts)-1]
		item, exists, err := fanOutItem(outputs, inner.ID, itemIndex)
		if err != nil {
			return nil, err
		}
		if exists {
			namespace["item"] = item
			namespace["index"] = int64(itemIndex)
		}
		namespace["progress"] = fanOutProgressValue(topology, outputs, inner.ID)
	}

	outputByKey := generationOutputMap(outputs)
	for _, node := range graph.Nodes {
		output, ok := scopedNodeOutput(outputByKey, topology, nodeID, node.ID, itemIndex)
		entry := map[string]any{}
		if ok {
			entry[namespaceStatusKey] = output.Status
			entry[namespaceOutputKey] = generationOutputRuntimeValue(output)
		}
		if isControlKind(node, dsl.ControlFanOut) {
			entry["progress"] = fanOutProgressValue(topology, outputs, node.ID)
		}
		nodes[string(node.ID)] = entry
		if alias, ok := subLoopLocalNodeAlias(nodeID, node.ID); ok {
			nodes[alias] = entry
		}
	}
	return namespace, nil
}

func activeFanOutNodes(graph dsl.Graph, topology controlTopology, nodeID dsl.NodeID) []dsl.Node {
	active := make([]dsl.Node, 0)
	for _, node := range graph.Nodes {
		if !isControlKind(node, dsl.ControlFanOut) {
			continue
		}
		if _, ok := topology.fanOutScopes[node.ID].body[nodeID]; ok {
			active = append(active, node)
		}
	}
	return active
}

func (h GenerationHistory) previousNamespace(
	topology controlTopology,
	current dsl.NodeID,
	itemIndex int,
) map[string]any {
	nodes := historyPreviousNodeDefaults(topology)
	verdicts := historyVerdictDefaults(topology)
	if h.Previous == nil {
		return map[string]any{
			metadataGenerationKey:   int64(0),
			namespaceNodesKey:       nodes,
			namespaceVerdictsKey:    verdicts,
			namespaceRouteCausesKey: []string{},
		}
	}
	for nodeID, items := range h.Previous.Nodes {
		node, ok := items[historyProjectionItemIndex(topology, current, dsl.NodeID(nodeID), itemIndex)]
		if !ok {
			continue
		}
		entry, exists := nodes[nodeID].(map[string]any)
		if !exists {
			entry = map[string]any{namespaceStatusKey: "", namespaceOutputKey: map[string]any{}}
		}
		entry[namespaceStatusKey] = node.Status
		entry[namespaceOutputKey] = mergeHistoryOutput(entry[namespaceOutputKey], node.Output)
		if node.Failure != nil {
			entry[namespaceFailureKey] = *node.Failure
			entry[namespaceDispositionKey] = node.Disposition
		}
		nodes[nodeID] = entry
	}
	for gateID, items := range h.Previous.Verdicts {
		verdict, ok := items[historyProjectionItemIndex(topology, current, dsl.NodeID(gateID), itemIndex)]
		if !ok {
			continue
		}
		verdicts[gateID] = map[string]any{
			namespaceOutcomeKey:        verdict.Outcome,
			namespaceScoreKey:          cloneFloat64(verdict.Score),
			namespaceBlockingIssuesKey: verdict.BlockingIssues,
			namespaceCriteriaKey:       verdict.Criteria,
		}
	}
	routeCauses := make([]string, 0, len(h.Previous.RouteCauses))
	for _, cause := range h.Previous.RouteCauses {
		if topology.sameFanOutBody(current, dsl.NodeID(cause.GateID)) && cause.ItemIndex != itemIndex {
			continue
		}
		routeCauses = append(routeCauses, cause.GateID)
	}
	return map[string]any{
		metadataGenerationKey:   h.Previous.Generation,
		namespaceNodesKey:       nodes,
		namespaceVerdictsKey:    verdicts,
		namespaceRouteCausesKey: routeCauses,
	}
}

func (h GenerationHistory) bestNamespace(
	topology controlTopology,
	current dsl.NodeID,
	itemIndex int,
) map[string]any {
	nodes := historyBestNodeDefaults(topology)
	if h.Best == nil {
		return map[string]any{
			metadataGenerationKey: int64(0),
			namespaceScoreKey:     float64(0),
			namespaceNodesKey:     nodes,
		}
	}
	for nodeID, items := range h.Best.Nodes {
		node, ok := items[historyProjectionItemIndex(topology, current, dsl.NodeID(nodeID), itemIndex)]
		if !ok {
			continue
		}
		entry, exists := nodes[nodeID].(map[string]any)
		if !exists {
			entry = map[string]any{namespaceOutputKey: map[string]any{}}
		}
		entry[namespaceOutputKey] = mergeHistoryOutput(entry[namespaceOutputKey], node.Output)
		nodes[nodeID] = entry
	}
	return map[string]any{
		metadataGenerationKey: h.Best.Generation,
		namespaceScoreKey:     h.Best.Score,
		namespaceNodesKey:     nodes,
	}
}

func historyProjectionItemIndex(
	topology controlTopology,
	current dsl.NodeID,
	target dsl.NodeID,
	itemIndex int,
) int {
	if topology.sameFanOutBody(current, target) {
		return itemIndex
	}
	return 0
}

func subLoopLocalNodeAlias(current dsl.NodeID, candidate dsl.NodeID) (string, bool) {
	currentPrefix, ok := subLoopNodePrefix(current)
	if !ok {
		return "", false
	}
	candidateID := string(candidate)
	prefix := currentPrefix + subLoopNodeSeparator
	if !strings.HasPrefix(candidateID, prefix) {
		return "", false
	}
	alias := strings.TrimPrefix(candidateID, prefix)
	if alias == "" || strings.Contains(alias, subLoopNodeSeparator) {
		return "", false
	}
	return alias, true
}

func subLoopNodePrefix(nodeID dsl.NodeID) (string, bool) {
	value := string(nodeID)
	idx := strings.LastIndex(value, subLoopNodeSeparator)
	if idx <= 0 {
		return "", false
	}
	return value[:idx], true
}

func scopedNodeOutput(
	outputs map[generationOutputKey]GenerationOutput,
	topology controlTopology,
	current dsl.NodeID,
	target dsl.NodeID,
	itemIndex int,
) (GenerationOutput, bool) {
	if topology.sameFanOutBody(current, target) {
		output, ok := outputs[generationOutputKey{nodeID: string(target), itemIndex: itemIndex}]
		return output, ok
	}
	output, ok := outputs[generationOutputKey{nodeID: string(target), itemIndex: 0}]
	return output, ok
}

func valueAtPath(namespace map[string]any, path []string) (any, bool) {
	var current any = namespace
	for _, part := range path {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[part]
			if !ok {
				return nil, false
			}
			current = next
		case map[any]any:
			next, ok := typed[part]
			if !ok {
				return nil, false
			}
			current = next
		default:
			return nil, false
		}
	}
	return current, true
}

func collectionItems(value any) ([]any, error) {
	switch typed := value.(type) {
	case nil:
		return nil, fmt.Errorf("%w: fan-out collection resolved to nil", ErrValidation)
	case []any:
		return append([]any(nil), typed...), nil
	case json.RawMessage:
		return collectionItemsFromJSON([]byte(typed))
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "[") {
			return collectionItemsFromJSON([]byte(trimmed))
		}
		return nil, fmt.Errorf("%w: fan-out collection must be a finite array", ErrValidation)
	default:
		reflected := reflect.ValueOf(value)
		if !reflected.IsValid() || reflected.Kind() != reflect.Slice {
			return nil, fmt.Errorf(
				"%w: fan-out collection must be a finite array, got %T",
				ErrValidation,
				value,
			)
		}
		items := make([]any, 0, reflected.Len())
		for idx := range reflected.Len() {
			items = append(items, reflected.Index(idx).Interface())
		}
		return items, nil
	}
}

func collectionItemsFromJSON(data []byte) ([]any, error) {
	var items []any
	if err := decodeSingleJSONValue(bytes.NewReader(data), &items); err != nil {
		return nil, fmt.Errorf("%w: decode fan-out collection: %w", ErrValidation, err)
	}
	return items, nil
}

func decodeSingleJSONValue(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return fmt.Errorf("unexpected trailing JSON value")
}

func cloneAnyMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneAny(value)
	}
	return cloned
}
