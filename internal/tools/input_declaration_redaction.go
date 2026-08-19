package tools

import (
	"fmt"
	"sort"
)

var publicInputDeclarationKeys = map[string]struct{}{
	"default":  {},
	"enum":     {},
	"ref":      {},
	"required": {},
	"type":     {},
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
	return declaration, true
}

// IsPublicInputDeclaration reports whether a dynamic object is the closed
// Loop input-declaration shape emitted by catalog projections.
func IsPublicInputDeclaration(value any) bool {
	_, ok := publicInputDeclaration(value)
	return ok
}

func redactSensitiveDeclarationValues(
	declaration map[string]any,
	path string,
	redactions []Redaction,
) (bool, []Redaction) {
	changed := false
	for _, key := range []string{"default", "enum"} {
		value, present := declaration[key]
		if !present {
			continue
		}
		declaration[key] = redactedJSONValue
		redactions = append(redactions, Redaction{
			Path: path + "." + key, Reason: ReasonSecretMetadata,
			Bytes: int64(len(fmt.Sprint(value))),
		})
		changed = true
	}
	return changed, redactions
}

func sortedAnyKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
