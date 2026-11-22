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

// addLoopRunTerminalSchemas adds the optional terminal read projection to every
// historical Loop run payload and registers the new component for $ref resolution.
func addLoopRunTerminalSchemas(baseline, current any) int {
	count := updateSchemaPairs(baseline, current, func(oldNode, newNode map[string]any) int {
		oldProperties, _ := oldNode["properties"].(map[string]any)
		newProperties, _ := newNode["properties"].(map[string]any)
		if oldProperties == nil || newProperties == nil {
			return 0
		}
		if oldProperties["loop_name"] == nil || oldProperties["completion_state"] == nil ||
			oldProperties["attention"] == nil {
			return 0
		}
		terminal, ok := newProperties["terminal"]
		if !ok {
			return 0
		}
		oldProperties["terminal"] = cloneSchemaValue(terminal)
		return 1
	})
	oldComponents, _ := schemaComponents(baseline)
	newComponents, _ := schemaComponents(current)
	if oldComponents != nil && oldComponents["LoopRunTerminal"] == nil {
		if terminal, ok := newComponents["LoopRunTerminal"]; ok {
			oldComponents["LoopRunTerminal"] = cloneSchemaValue(terminal)
		}
	}
	return count
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

func schemaComponents(document any) (map[string]any, bool) {
	root, ok := document.(map[string]any)
	if !ok {
		return nil, false
	}
	components, ok := root["components"].(map[string]any)
	if !ok {
		return nil, false
	}
	schemas, ok := components["schemas"].(map[string]any)
	return schemas, ok
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
