package spec

import "encoding/json"

// addLoopOnResultSchemas refreshes every historical authored on_result schema:
// gate nodes take the in-body grammar, and contracts take the forward-target-free
// definition-of-done grammar.
func addLoopOnResultSchemas(baseline any) int {
	nodeSchema := schemaJSON(loopNodeOnResultSchema())
	contractSchema := schemaJSON(loopContractOnResultSchema())
	return walkSchemaProperties(baseline, func(properties map[string]any) int {
		contractShape := properties["goal"] != nil && properties["definition_of_done"] != nil &&
			properties["verification"] != nil && properties["iteration_cap"] != nil
		if contractShape {
			properties["on_result"] = cloneSchemaValue(contractSchema)
			return 1
		}
		gateShape := properties["criteria"] != nil && properties["on_result"] != nil
		if gateShape {
			properties["on_result"] = cloneSchemaValue(nodeSchema)
			return 1
		}
		return 0
	})
}

// addLoopRunTerminalSchemas adds the optional terminal read projection to every
// historical Loop run payload and registers the new component for $ref resolution.
func addLoopRunTerminalSchemas(baseline, current any) int {
	terminalSchema := map[string]any{
		"oneOf": []any{
			map[string]any{"$ref": "#/components/schemas/LoopRunTerminal"},
			map[string]any{"enum": []any{nil}, "nullable": true},
		},
	}
	count := walkSchemaProperties(baseline, func(properties map[string]any) int {
		if properties["loop_name"] == nil || properties["completion_state"] == nil ||
			properties["attention"] == nil {
			return 0
		}
		properties["terminal"] = cloneSchemaValue(terminalSchema)
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

func walkSchemaProperties(value any, visit func(map[string]any) int) int {
	count := 0
	switch node := value.(type) {
	case map[string]any:
		if properties, ok := node["properties"].(map[string]any); ok {
			count += visit(properties)
		}
		for _, child := range node {
			count += walkSchemaProperties(child, visit)
		}
	case []any:
		for _, child := range node {
			count += walkSchemaProperties(child, visit)
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

func schemaJSON(value any) map[string]any {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		panic(err)
	}
	return decoded
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
