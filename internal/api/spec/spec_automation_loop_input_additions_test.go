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

// addAutomationJobUpdateLoopInputCommandSchema adds the optional command-only update field to every
// historical automation job update request (the payload carrying prompt/schedule/task plus a
// loop_target, but none of the create-time scope fields).
func addAutomationJobUpdateLoopInputCommandSchema(value any) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addAutomationJobUpdateLoopInputCommandSchema(child)
		}
	case map[string]any:
		for child := range maps.Values(node) {
			count += addAutomationJobUpdateLoopInputCommandSchema(child)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok || properties["loop_target"] == nil || properties["prompt"] == nil ||
			properties["schedule"] == nil || properties["task"] == nil ||
			properties["enabled"] == nil || properties["fire_limit"] == nil ||
			properties["scope"] != nil || properties["target_kind"] != nil {
			return count
		}
		if _, exists := properties["loop_input_command"]; exists {
			return count
		}
		properties["loop_input_command"] = map[string]any{"nullable": true, "type": "string"}
		count++
	}
	return count
}
