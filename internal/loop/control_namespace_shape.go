package loop

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

type historyToolSchemaSource map[string]ToolSchemaSnapshot

func (s historyToolSchemaSource) Snapshot(toolID string) (ToolSchemaSnapshot, bool) {
	snapshot, ok := s[toolID]
	return snapshot, ok
}

func historyNodeZeroValues(
	definition dsl.Definition,
	toolSchemas map[string]ToolSchemaSnapshot,
) map[dsl.NodeID]any {
	ctx := newLintContext(definition, &DefinitionLinter{tools: historyToolSchemaSource(toolSchemas)})
	values := make(map[dsl.NodeID]any, len(definition.Graph.Nodes))
	for _, node := range definition.Graph.Nodes {
		schema, ok := ctx.declaredSchema(node)
		if !ok {
			values[node.ID] = map[string]any{}
			continue
		}
		values[node.ID] = schemaZeroValue(schema)
	}
	return values
}

func historyGateIDs(definition dsl.Definition) map[string]struct{} {
	gates := map[string]struct{}{}
	for _, node := range definition.Graph.Nodes {
		if isControlKind(node, dsl.ControlGate) {
			gates[string(node.ID)] = struct{}{}
		}
	}
	if len(definition.Contract.Verification) > 0 {
		gates["contract"] = struct{}{}
	}
	return gates
}

func historyPreviousNodeDefaults(topology controlTopology) map[string]any {
	nodes := make(map[string]any, len(topology.historyNodes))
	for nodeID, output := range topology.historyNodes {
		nodes[string(nodeID)] = map[string]any{
			namespaceStatusKey: "",
			namespaceOutputKey: cloneAny(output),
		}
	}
	return nodes
}

func historyBestNodeDefaults(topology controlTopology) map[string]any {
	nodes := make(map[string]any, len(topology.historyNodes))
	for nodeID, output := range topology.historyNodes {
		nodes[string(nodeID)] = map[string]any{namespaceOutputKey: cloneAny(output)}
	}
	return nodes
}

func historyVerdictDefaults(topology controlTopology) map[string]any {
	verdicts := make(map[string]any, len(topology.historyGates))
	for gateID := range topology.historyGates {
		verdicts[gateID] = map[string]any{
			namespaceOutcomeKey:        "",
			namespaceScoreKey:          nil,
			namespaceBlockingIssuesKey: []any{},
			namespaceCriteriaKey:       []any{},
		}
	}
	return verdicts
}

func mergeHistoryOutput(base any, actual any) any {
	baseMap, baseOK := cloneAny(base).(map[string]any)
	actualMap, actualOK := actual.(map[string]any)
	if !baseOK || !actualOK {
		if actual == nil {
			return cloneAny(base)
		}
		return cloneAny(actual)
	}
	for key, value := range actualMap {
		baseMap[key] = mergeHistoryOutput(baseMap[key], value)
	}
	return baseMap
}

func schemaZeroValue(schema refs.Schema) any {
	return schemaValueZero(map[string]any(schema))
}

func schemaValueZero(value any) any {
	switch typed := value.(type) {
	case string:
		return schemaTypeZero(typed, nil)
	case dsl.Schema:
		return schemaValueZero(map[string]any(typed))
	case refs.Schema:
		return schemaValueZero(map[string]any(typed))
	case map[string]any:
		if typeName, ok := typed[jsonSchemaTypeKey].(string); ok {
			return schemaTypeZero(typeName, typed)
		}
		if properties, ok := schemaProperties(typed); ok {
			return schemaObjectZero(properties)
		}
		object := make(map[string]any, len(typed))
		for key, child := range typed {
			if isJSONSchemaKeyword(key) {
				continue
			}
			object[key] = schemaValueZero(child)
		}
		return object
	case []any:
		return []any{}
	default:
		return nil
	}
}

func schemaTypeZero(typeName string, schema map[string]any) any {
	switch typeName {
	case jsonSchemaObjectType:
		properties, _ := schemaProperties(schema)
		return schemaObjectZero(properties)
	case jsonSchemaArrayType:
		return []any{}
	case jsonSchemaStringType:
		return ""
	case jsonSchemaBooleanType:
		return false
	case jsonSchemaNumberType, jsonSchemaIntegerType:
		return json.Number("0")
	case jsonSchemaNullType:
		return nil
	default:
		return nil
	}
}

func schemaProperties(schema map[string]any) (map[string]any, bool) {
	properties, ok := schema[jsonSchemaPropertiesKey].(map[string]any)
	if ok {
		return properties, true
	}
	if typed, ok := schema[jsonSchemaPropertiesKey].(refs.Schema); ok {
		return map[string]any(typed), true
	}
	return nil, false
}

func schemaObjectZero(properties map[string]any) map[string]any {
	object := make(map[string]any, len(properties))
	for key, child := range properties {
		object[key] = schemaValueZero(child)
	}
	return object
}

func isJSONSchemaKeyword(key string) bool {
	switch key {
	case jsonSchemaTypeKey, jsonSchemaRequiredKey, jsonSchemaAdditionalPropertiesKey,
		jsonSchemaTitleKey, jsonSchemaAllOfKey, jsonSchemaAnyOfKey, jsonSchemaOneOfKey,
		jsonSchemaDependentSchemasKey, jsonSchemaThenKey, "description", jsonSchemaItemsKey, "$schema", "$id":
		return true
	default:
		return false
	}
}
