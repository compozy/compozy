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
	if n.fallbackPathAccepted(path, path) {
		return true
	}
	return n.AllowOpenOutput && validateSchemaPath(n.Output, path, path, true, n.OutputJSONSchema) == nil
}

// CheckOutputPath distinguishes proven paths, unproven acceptance, and rejected paths.
func (n NodeSchema) CheckOutputPath(path, original []string) (bool, *Error) {
	if err := n.validateOutputPath(path, original); err != nil {
		return false, err
	}
	return n.OutputPathUnproven(path), nil
}

func (n NodeSchema) validateOutputPath(path, original []string) *Error {
	err := validateSchemaPath(n.Output, path, original, n.AllowOpenOutput, n.OutputJSONSchema)
	if err != nil && n.fallbackPathAccepted(path, original) {
		return nil
	}
	return err
}

func (n NodeSchema) fallbackPathAccepted(path, original []string) bool {
	return n.FallbackOutput != nil && validateSchemaPath(n.FallbackOutput, path, original, false, false) == nil
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

// InputPathUnproven reports inferred input paths accepted only by field-map compatibility.
func (n Namespace) InputPathUnproven(path []string) bool {
	if len(path) < 3 || path[0] != namespaceInputs {
		return false
	}
	schema, ok := n.Inputs[path[1]]
	return ok && validateSchemaPath(schema, path[2:], path, false, true) != nil &&
		validateSchemaPath(schema, path[2:], path, false, false) == nil
}
