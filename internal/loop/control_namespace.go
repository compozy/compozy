package loop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
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
	nodes := map[string]any{}
	namespace := map[string]any{
		namespaceInputsKey:    cloneAnyMap(run.Inputs),
		namespaceNodesKey:     nodes,
		metadataGenerationKey: int64(max(1, generation)),
	}
	if fanOutID, ok := topology.inFanOutBody(nodeID); ok {
		item, exists, err := fanOutItem(outputs, fanOutID, itemIndex)
		if err != nil {
			return nil, err
		}
		if exists {
			namespace["item"] = item
			namespace["index"] = int64(itemIndex)
		}
	}

	outputByKey := generationOutputMap(outputs)
	for _, node := range graph.Nodes {
		output, ok := scopedNodeOutput(outputByKey, topology, nodeID, node.ID, itemIndex)
		entry := map[string]any{}
		if ok {
			entry["status"] = output.Status
			entry["output"] = outputValue(output.OutputRef)
		}
		nodes[string(node.ID)] = entry
		if alias, ok := subLoopLocalNodeAlias(nodeID, node.ID); ok {
			nodes[alias] = entry
		}
	}
	return namespace, nil
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

func outputValue(ref string) any {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return nil
	}
	var value any
	if err := decodeSingleJSONValue(strings.NewReader(trimmed), &value); err == nil {
		return value
	}
	return trimmed
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
		for idx := 0; idx < reflected.Len(); idx++ {
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
