package refs

import "slices"

const schemaObjectType = "object"

// OutputPathUnproven reports a path accepted only through an open output object.
func (n NodeSchema) OutputPathUnproven(path []string) bool {
	return n.AllowOpenOutput && validateSchemaPath(n.Output, path, path, false) != nil &&
		validateSchemaPath(n.Output, path, path, true) == nil
}

func openSchemaObject(current any) bool {
	if schema, ok := current.(Schema); ok {
		current = map[string]any(schema)
	}
	schema, ok := current.(map[string]any)
	if !ok {
		return false
	}
	if additional, exists := schema["additionalProperties"]; exists && additional != true {
		return false
	}
	switch kind := schema["type"].(type) {
	case nil:
		return true
	case string:
		return kind == schemaObjectType
	case []any:
		for _, value := range kind {
			if value == schemaObjectType {
				return true
			}
		}
	case []string:
		return slices.Contains(kind, schemaObjectType)
	}
	return false
}
