package spec

import "maps"

// addLoopRunOutcomeDiagnosticSchemas adds the optional reason_code, node_id, and message
// diagnostics to every historical Loop run outcome schema (the objects carrying the
// actor_kind/actor_ref/cause/status/at settlement fields). Existing fields and the required
// set stay unchanged, so the additions remain optional.
func addLoopRunOutcomeDiagnosticSchemas(value any) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addLoopRunOutcomeDiagnosticSchemas(child)
		}
	case map[string]any:
		for child := range maps.Values(node) {
			count += addLoopRunOutcomeDiagnosticSchemas(child)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok || properties["actor_kind"] == nil || properties["actor_ref"] == nil ||
			properties["cause"] == nil || properties["status"] == nil || properties["at"] == nil {
			return count
		}
		for _, name := range []string{"reason_code", "node_id", "message"} {
			properties[name] = map[string]any{"type": "string"}
		}
		count++
	}
	return count
}
