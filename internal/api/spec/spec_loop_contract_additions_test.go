package spec

// addLoopOnResultSchemas refreshes every historical authored on_result schema from
// the current document: gate nodes tighten their object grammar, and contracts gain
// the optional map.
func addLoopOnResultSchemas(baseline, current any) int {
	return updateSchemaPairs(baseline, current, func(oldNode, newNode map[string]any) int {
		oldProperties, _ := oldNode["properties"].(map[string]any)
		newProperties, _ := newNode["properties"].(map[string]any)
		if oldProperties == nil || newProperties == nil {
			return 0
		}
		contractShape := oldProperties["goal"] != nil && oldProperties["definition_of_done"] != nil &&
			oldProperties["verification"] != nil && oldProperties["iteration_cap"] != nil
		gateShape := oldProperties["criteria"] != nil && oldProperties["on_result"] != nil
		if !contractShape && !gateShape {
			return 0
		}
		onResult, ok := newProperties["on_result"]
		if !ok {
			return 0
		}
		oldProperties["on_result"] = cloneSchemaValue(onResult)
		return 1
	})
}

func updateSchemaPairs(oldValue, newValue any, update func(map[string]any, map[string]any) int) int {
	count := 0
	switch oldNode := oldValue.(type) {
	case map[string]any:
		newNode, ok := newValue.(map[string]any)
		if !ok {
			return 0
		}
		count += update(oldNode, newNode)
		for key, child := range oldNode {
			count += updateSchemaPairs(child, newNode[key], update)
		}
	case []any:
		newArray, ok := newValue.([]any)
		if !ok || len(newArray) != len(oldNode) {
			return count
		}
		for index := range oldNode {
			count += updateSchemaPairs(oldNode[index], newArray[index], update)
		}
	}
	return count
}

func cloneSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, child := range typed {
			cloned[key] = cloneSchemaValue(child)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for index, child := range typed {
			cloned[index] = cloneSchemaValue(child)
		}
		return cloned
	default:
		return value
	}
}
