package loop

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/tools"
)

const schemaAnyType = "any"

func inputSchema(input dsl.Input) refs.Schema {
	switch input.Type {
	case dsl.InputTypeAgent, dsl.InputTypeRef, dsl.InputTypeFile:
		return refs.Schema{jsonSchemaTypeKey: jsonSchemaStringType}
	case dsl.InputTypeRuntime:
		return refs.Schema{
			jsonSchemaTypeKey: jsonSchemaObjectType,
			jsonSchemaPropertiesKey: map[string]any{
				runtimeFieldProvider:  map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
				runtimeFieldModel:     map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
				runtimeFieldReasoning: map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
				runtimeFieldACPOptions: map[string]any{
					jsonSchemaTypeKey: jsonSchemaArrayType,
					"items": map[string]any{
						jsonSchemaTypeKey: jsonSchemaObjectType,
						jsonSchemaPropertiesKey: map[string]any{
							runtimeACPOptionIDKey:        map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
							runtimeACPOptionValueIDKey:   map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
							runtimeACPOptionBoolValueKey: map[string]any{jsonSchemaTypeKey: jsonSchemaBooleanType},
						},
						jsonSchemaAdditionalPropertiesKey: false,
						jsonSchemaRequiredKey:             []string{runtimeACPOptionIDKey},
						jsonSchemaOneOfKey: []any{
							map[string]any{jsonSchemaRequiredKey: []string{runtimeACPOptionValueIDKey}},
							map[string]any{jsonSchemaRequiredKey: []string{runtimeACPOptionBoolValueKey}},
						},
					},
				},
			},
			jsonSchemaAdditionalPropertiesKey: false,
		}
	default:
		return refs.Schema{jsonSchemaTypeKey: string(input.Type)}
	}
}

func (c *lintContext) outputSchema(node dsl.Node) (refs.Schema, bool) {
	if len(node.Produces) > 0 {
		return convertSchema(node.Produces), true
	}
	switch node.Class {
	case dsl.NodeClassSource:
		return c.sourceOutputSchema(node)
	case dsl.NodeClassAction:
		return c.actionOutputSchema(node)
	case dsl.NodeClassControl:
		if dsl.ControlKind(node.Kind) == dsl.ControlAsk {
			var params dsl.AskParams
			if err := node.Params.Decode(&params); err == nil && len(params.Expect) > 0 {
				return convertSchema(params.Expect), true
			}
		}
		return nil, false
	default:
		return nil, false
	}
}

func (c *lintContext) sourceOutputSchema(node dsl.Node) (refs.Schema, bool) {
	switch dsl.SourceKind(node.Kind) {
	case dsl.SourceInput:
		input, ok := c.def.Inputs[node.InputRef]
		if !ok {
			return nil, false
		}
		return inputSchema(input), true
	case dsl.SourceWatchEvents:
		return watchEventsOutputSchema(), true
	default:
		return nil, false
	}
}

func (c *lintContext) actionOutputSchema(node dsl.Node) (refs.Schema, bool) {
	if !dsl.IsReservedActionKind(node.Kind) {
		return c.toolOutputSchema(node)
	}
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionGoal:
		params, err := decodeGoalNodeParams(node.Params)
		// Invalid params expose no trustworthy output schema, so downstream reference checks stop here.
		if err != nil || params.OutputSchema == nil {
			return nil, false
		}
		return convertSchema(*params.OutputSchema), true
	case dsl.ActionRunAgent:
		params, err := decodeRunAgentNodeParams(node.Params)
		// Invalid params expose no trustworthy output schema, so downstream reference checks stop here.
		if err != nil || len(params.OutputSchema) == 0 {
			return nil, false
		}
		return convertSchema(params.OutputSchema), true
	case dsl.ActionRunLoop:
		if c.legacyRunLoopOutputSchema {
			return refs.Schema{
				reasonMetaStatus:     jsonSchemaStringType,
				"outputs":            map[string]any{},
				metadataLoopRunIDKey: jsonSchemaStringType,
			}, true
		}
		var params dsl.RunLoopParams
		if err := node.Params.Decode(&params); err != nil {
			return nil, false
		}
		if params.Mode == dsl.RunLoopDetach {
			return refs.Schema{metadataLoopRunIDKey: jsonSchemaStringType}, true
		}
		return refs.Schema{jsonSchemaTypeKey: jsonSchemaStringType}, true
	case dsl.ActionTransform:
		var params dsl.TransformParams
		// Invalid params expose no trustworthy output schema, so downstream reference checks stop here.
		if err := node.Params.Decode(&params); err != nil || len(params.Map) == 0 {
			return nil, false
		}
		schema := refs.Schema{}
		for key := range params.Map {
			schema[key] = schemaAnyType
		}
		return schema, true
	default:
		return nil, false
	}
}

func (c *lintContext) toolOutputSchema(node dsl.Node) (refs.Schema, bool) {
	if c.linter.tools == nil {
		return nil, false
	}
	snapshot, ok := c.linter.tools.Snapshot(node.Kind)
	raw, _ := snapshot.OutputSchemaFor(nodeResultView(node))
	if !ok || len(raw) == 0 {
		return nil, false
	}
	schema, err := schemaFromJSON(raw)
	// An invalid tool schema cannot safely participate in downstream reference checks.
	if err != nil {
		return nil, false
	}
	return schema, true
}

func convertSchema(schema dsl.Schema) refs.Schema {
	out := refs.Schema{}
	for key, value := range schema {
		out[key] = normalizeSchemaValue(value)
	}
	return out
}

func normalizeSchemaValue(value any) any {
	switch typed := value.(type) {
	case dsl.Schema:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[key] = normalizeSchemaValue(child)
		}
		return normalized
	case map[string]any:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[key] = normalizeSchemaValue(child)
		}
		return normalized
	case map[any]any:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[fmt.Sprint(key)] = normalizeSchemaValue(child)
		}
		return normalized
	case []any:
		normalized := make([]any, 0, len(typed))
		for _, child := range typed {
			normalized = append(normalized, normalizeSchemaValue(child))
		}
		return normalized
	default:
		return typed
	}
}

func schemaFromJSON(raw json.RawMessage) (refs.Schema, error) {
	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode JSON schema: %w", err)
	}
	return refs.Schema(decoded), nil
}

func normalizedReferenceSchema(schema dsl.Schema) (refs.Schema, bool) {
	normalized, err := normalizeLoopSchema(schema)
	return refs.Schema(normalized), err == nil
}

func referenceFieldSchema(fields refs.Schema) refs.Schema {
	properties := make(map[string]any, len(fields))
	for key, value := range fields {
		if value == schemaAnyType {
			properties[key] = map[string]any{}
		} else {
			properties[key] = shorthandPropertySchema(value)
		}
	}
	return refs.Schema{"type": "object", "properties": properties, "additionalProperties": false}
}

func outputIsJSONSchema(node dsl.Node) bool {
	return node.Class != dsl.NodeClassSource ||
		(len(node.Produces) == 0 && dsl.SourceKind(node.Kind) == dsl.SourceInput)
}

func isActionKind(node dsl.Node, kind dsl.ActionKind) bool {
	return node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == kind
}

func isToolAction(node dsl.Node) bool {
	return node.Class == dsl.NodeClassAction && !dsl.IsReservedActionKind(node.Kind)
}

func (c *lintContext) referenceNodeSchema(node dsl.Node) refs.NodeSchema {
	raw, ok := c.outputSchema(node)
	result := refs.NodeSchema{
		Output:           raw,
		FallbackOutput:   raw,
		HasOutput:        ok,
		OutputJSONSchema: outputIsJSONSchema(node),
		HasProgress:      isControlKind(node, dsl.ControlFanOut),
	}
	if isToolAction(node) && len(node.Produces) == 0 {
		result.FallbackOutput = nil
		if c.linter.tools != nil {
			snapshot, found := c.linter.tools.Snapshot(node.Kind)
			result.ResultViewOutput = found && snapshot.FullOutputSchemaDigest != ""
			result.AllowOpenOutput = result.ResultViewOutput &&
				nodeResultView(node) == tools.ResultViewFull
		}
		return result
	}
	if !ok || node.Class == dsl.NodeClassSource {
		return result
	}
	if len(node.Produces) > 0 || isActionKind(node, dsl.ActionRunAgent) || isActionKind(node, dsl.ActionGoal) ||
		isControlKind(node, dsl.ControlAsk) {
		result.Output, _ = normalizedReferenceSchema(dsl.Schema(raw))
		result.AllowOpenOutput = !isToolAction(node) && isFullJSONSchema(map[string]any(raw))
	} else if isActionKind(node, dsl.ActionTransform) ||
		(isActionKind(node, dsl.ActionRunLoop) && raw[jsonSchemaTypeKey] == nil) {
		result.Output = referenceFieldSchema(raw)
	}
	return result
}
