package spec

import "maps"

// addLoopContractOnResultSchemas adds the optional authored on_result map to every
// historical Loop contract schema. Existing fields and the required set stay
// unchanged, so the addition remains optional.
func addLoopContractOnResultSchemas(value any) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addLoopContractOnResultSchemas(child)
		}
	case map[string]any:
		for child := range maps.Values(node) {
			count += addLoopContractOnResultSchemas(child)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok || properties["goal"] == nil || properties["definition_of_done"] == nil ||
			properties["verification"] == nil || properties["iteration_cap"] == nil {
			return count
		}
		if properties["on_result"] == nil {
			properties["on_result"] = map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{},
			}
			count++
		}
	}
	return count
}
