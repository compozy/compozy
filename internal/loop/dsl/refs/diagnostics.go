package refs

import (
	"fmt"
	"slices"
	"strings"
)

func availableNodeIDs(nodes map[string]NodeSchema) []string {
	available := make([]string, 0, len(nodes))
	for nodeID := range nodes {
		available = append(available, nodeID)
	}
	slices.Sort(available)
	return available
}

func availableSchemaFields(current any) []string {
	switch typed := current.(type) {
	case Schema:
		return availableSchemaFields(map[string]any(typed))
	case map[string]any:
		if properties, ok := typed["properties"]; ok {
			return availablePropertyKeys(properties)
		}
		if typed["type"] == "array" {
			return availableSchemaFields(typed["items"])
		}
		if _, isTypedSchema := typed["type"]; isTypedSchema {
			return nil
		}
		return sortedMapKeys(typed)
	case map[string]Schema:
		available := make([]string, 0, len(typed))
		for field := range typed {
			available = append(available, field)
		}
		slices.Sort(available)
		return available
	case []any:
		if len(typed) == 0 {
			return nil
		}
		return availableSchemaFields(typed[0])
	default:
		return nil
	}
}

func availablePropertyKeys(properties any) []string {
	switch typed := properties.(type) {
	case map[string]any:
		return sortedMapKeys(typed)
	case map[string]Schema:
		available := make([]string, 0, len(typed))
		for field := range typed {
			available = append(available, field)
		}
		slices.Sort(available)
		return available
	default:
		return nil
	}
}

func sortedMapKeys(values map[string]any) []string {
	available := make([]string, 0, len(values))
	for field := range values {
		available = append(available, field)
	}
	slices.Sort(available)
	return available
}

func availableNodeMessage(missing string, available []string) string {
	message := fmt.Sprintf("unknown node %q", missing)
	if len(available) > 0 {
		message += "; available nodes: " + strings.Join(available, ", ")
	}
	return message
}

func schemaPathMessage(original, deepest, available []string) string {
	message := fmt.Sprintf("schema path %q is not declared", strings.Join(original, "."))
	if len(deepest) > 0 {
		message += fmt.Sprintf("; deepest valid segment %q", strings.Join(deepest, "."))
	}
	if len(available) > 0 {
		message += "; available fields: " + strings.Join(available, ", ")
	}
	return message
}
