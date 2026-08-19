package tools

import (
	"sort"
)

var publicInputDeclarationKeys = map[string]struct{}{
	"default":     {},
	"description": {},
	"enum":        {},
	"ref":         {},
	"required":    {},
	"type":        {},
}

func publicInputDeclaration(value any) (map[string]any, bool) {
	declaration, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	inputType, ok := declaration["type"].(string)
	if !ok {
		return nil, false
	}
	switch inputType {
	case "string", "number", "boolean", "file", "agent", "ref", "runtime":
	default:
		return nil, false
	}
	for key := range declaration {
		if _, allowed := publicInputDeclarationKeys[key]; !allowed {
			return nil, false
		}
	}
	if description, exists := declaration["description"]; exists {
		if _, ok := description.(string); !ok {
			return nil, false
		}
	}
	if required, exists := declaration["required"]; exists {
		if _, ok := required.(bool); !ok {
			return nil, false
		}
	}
	if enum, exists := declaration["enum"]; exists {
		if _, ok := enum.([]any); !ok {
			return nil, false
		}
	}
	if ref, exists := declaration["ref"]; exists {
		refValue, ok := ref.(map[string]any)
		if !ok || len(refValue) != 1 {
			return nil, false
		}
		if _, ok := refValue["kind"].(string); !ok {
			return nil, false
		}
	}
	return declaration, true
}

// IsPublicInputDeclaration reports whether a dynamic object is the closed
// Loop input-declaration shape emitted by catalog projections.
func IsPublicInputDeclaration(value any) bool {
	_, ok := publicInputDeclaration(value)
	return ok
}

func sortedAnyKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
