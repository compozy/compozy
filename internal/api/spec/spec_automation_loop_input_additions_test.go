package spec

import "maps"

// addAutomationLoopInputCommandSchema adds the optional input_command to every historical
// automation Loop target schema (the objects carrying inputs/input_mapping). The field stays
// optional, so stored jobs and triggers keep decoding unchanged.
func addAutomationLoopInputCommandSchema(value any) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addAutomationLoopInputCommandSchema(child)
		}
	case map[string]any:
		for child := range maps.Values(node) {
			count += addAutomationLoopInputCommandSchema(child)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok || properties["inputs"] == nil || properties["input_mapping"] == nil ||
			properties["loop_name"] == nil {
			return count
		}
		if _, exists := properties["input_command"]; exists {
			return count
		}
		properties["input_command"] = map[string]any{"type": "string"}
		count++
	}
	return count
}
