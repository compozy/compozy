package refs

import "slices"

const (
	schemaObjectType = "object"
	schemaArrayType  = "array"
)

// OutputPathUnproven reports a path accepted without proof from its output contract.
func (n NodeSchema) OutputPathUnproven(path []string) bool {
	if validateSchemaPath(n.Output, path, path, false, n.OutputJSONSchema) == nil {
		return false
	}
	if n.authoredPathAccepted(path, path) {
		return true
	}
	return n.AllowOpenOutput && validateSchemaPath(n.Output, path, path, true, n.OutputJSONSchema) == nil
}

func (n NodeSchema) validateOutputPath(path, original []string) *Error {
	err := validateSchemaPath(n.Output, path, original, n.AllowOpenOutput, n.OutputJSONSchema)
	if err != nil && n.authoredPathAccepted(path, original) {
		return nil
	}
	return err
}

func (n NodeSchema) authoredPathAccepted(path, original []string) bool {
	return n.AuthoredOutput != nil && validateSchemaPath(n.AuthoredOutput, path, original, false, false) == nil
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
