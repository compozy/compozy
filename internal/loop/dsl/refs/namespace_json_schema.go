package refs

import (
	"slices"
	"strings"
)

type schemaPathProof uint8

const (
	schemaPathRejected schemaPathProof = iota
	schemaPathUnproven
	schemaPathProven
)

func validateJSONSchemaPath(schema Schema, path, original []string, allowOpen bool) *Error {
	proof, depth, fields := resolveJSONSchemaPath(map[string]any(schema), map[string]any(schema), path, 0)
	if proof == schemaPathProven || (allowOpen && proof == schemaPathUnproven) {
		return nil
	}
	base := len(original) - len(path)
	deepest := append([]string(nil), original[:base+depth]...)
	return &Error{
		Code:             CodeUnresolvablePath,
		Path:             append([]string(nil), original[:min(len(original), base+depth+1)]...),
		ReferencePath:    append([]string(nil), original...),
		DeepestValidPath: deepest,
		AvailableFields:  fields,
		Message:          schemaPathMessage(original, deepest, fields),
	}
}

func resolveJSONSchemaPath(root, schema map[string]any, path []string, budget int) (schemaPathProof, int, []string) {
	if len(path) == 0 {
		return schemaPathProven, 0, nil
	}
	if budget > 128 {
		return schemaPathRejected, 0, nil
	}
	if ref, ok := schema["$ref"].(string); ok {
		if target, ok := localSchemaReference(root, ref); ok {
			return resolveJSONSchemaPath(root, target, path, budget+1)
		}
		return schemaPathRejected, 0, nil
	}
	for _, keyword := range []string{"anyOf", "oneOf", "allOf"} {
		if branches, ok := schema[keyword].([]any); ok {
			return resolveSchemaBranches(root, branches, keyword, path, budget)
		}
	}
	if schemaHasType(schema, schemaArrayType) {
		if items, ok := schema["items"].(map[string]any); ok {
			return resolveJSONSchemaPath(root, items, path, budget+1)
		}
		return schemaPathRejected, 0, nil
	}
	if child, ok := childFromProperties(schema["properties"], path[0]); ok {
		if len(path) == 1 {
			return schemaPathProven, 1, nil
		}
		if object, ok := child.(map[string]any); ok {
			proof, depth, fields := resolveJSONSchemaPath(root, object, path[1:], budget+1)
			return proof, depth + 1, fields
		}
		return schemaPathRejected, 1, availableSchemaFields(child, true)
	}
	if schemaHasType(schema, schemaObjectType) || schema["type"] == nil {
		if additional, ok := schema["additionalProperties"].(map[string]any); ok {
			proof, depth, fields := resolveJSONSchemaPath(root, additional, path[1:], budget+1)
			return proof, depth + 1, fields
		}
		if openSchemaObject(schema) {
			return schemaPathUnproven, 0, availableSchemaFields(schema, true)
		}
	}
	return schemaPathRejected, 0, availableSchemaFields(schema, true)
}

func schemaHasType(schema map[string]any, kind string) bool {
	switch value := schema["type"].(type) {
	case string:
		return value == kind
	case []any:
		return slices.Contains(value, any(kind))
	case []string:
		return slices.Contains(value, kind)
	}
	return false
}

func localSchemaReference(root map[string]any, ref string) (map[string]any, bool) {
	if ref == "#" {
		return root, true
	}
	pointer, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return nil, false
	}
	var current any = root
	for segment := range strings.SplitSeq(pointer, "/") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current = object[strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")]
	}
	result, ok := current.(map[string]any)
	return result, ok
}

func resolveSchemaBranches(
	root map[string]any,
	branches []any,
	keyword string,
	path []string,
	budget int,
) (schemaPathProof, int, []string) {
	proof, depth := schemaPathRejected, 0
	var fields []string
	for _, value := range branches {
		branch, ok := value.(map[string]any)
		if !ok {
			continue
		}
		candidate, reached, available := resolveJSONSchemaPath(root, branch, path, budget+1)
		if keyword == "allOf" && candidate == schemaPathRejected {
			return candidate, reached, available
		}
		proof = max(proof, candidate)
		if reached >= depth {
			depth, fields = reached, available
		}
	}
	return proof, depth, fields
}
