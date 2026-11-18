package refs

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

func availableNodeIDs(nodes map[string]NodeSchema) []string {
	available := slices.AppendSeq(make([]string, 0, len(nodes)), maps.Keys(nodes))
	slices.Sort(available)
	return available
}

func availableSchemaFields(current any, jsonSchema bool) []string {
	switch typed := current.(type) {
	case Schema:
		return availableSchemaFields(map[string]any(typed), jsonSchema)
	case map[string]any:
		if jsonSchema {
			if properties, ok := typed["properties"]; ok {
				return availablePropertyKeys(properties)
			}
			if typed["type"] == schemaArrayType {
				return availableSchemaFields(typed["items"], true)
			}
			return nil
		}
		if properties, ok := typed["properties"]; ok {
			return availablePropertyKeys(properties)
		}
		if typed["type"] == schemaArrayType {
			return availableSchemaFields(typed["items"], false)
		}
		return sortedMapKeys(typed)
	case map[string]Schema:
		available := slices.AppendSeq(make([]string, 0, len(typed)), maps.Keys(typed))
		slices.Sort(available)
		return available
	case []any:
		if len(typed) == 0 {
			return nil
		}
		return availableSchemaFields(typed[0], jsonSchema)
	default:
		return nil
	}
}

func availablePropertyKeys(properties any) []string {
	switch typed := properties.(type) {
	case map[string]any:
		return sortedMapKeys(typed)
	case map[string]Schema:
		available := slices.AppendSeq(make([]string, 0, len(typed)), maps.Keys(typed))
		slices.Sort(available)
		return available
	default:
		return nil
	}
}

func sortedMapKeys(values map[string]any) []string {
	available := slices.AppendSeq(make([]string, 0, len(values)), maps.Keys(values))
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
