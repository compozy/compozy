package loop

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func inputSchema(input dsl.Input) refs.Schema {
	return refs.Schema{jsonSchemaTypeKey: string(input.Type)}
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
		return c.toolOutputSchema(node.Kind)
	}
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionGoal:
		var params dsl.GoalParams
		// Invalid params expose no trustworthy output schema, so downstream reference checks stop here.
		if err := node.Params.Decode(&params); err != nil || params.OutputSchema == nil {
			return nil, false
		}
		return convertSchema(*params.OutputSchema), true
	case dsl.ActionRunAgent:
		var params dsl.RunAgentParams
		// Invalid params expose no trustworthy output schema, so downstream reference checks stop here.
		if err := node.Params.Decode(&params); err != nil || len(params.OutputSchema) == 0 {
			return nil, false
		}
		return convertSchema(params.OutputSchema), true
	case dsl.ActionRunLoop:
		return refs.Schema{reasonMetaStatus: jsonSchemaStringType, "outputs": map[string]any{}}, true
	case dsl.ActionTransform:
		var params dsl.TransformParams
		// Invalid params expose no trustworthy output schema, so downstream reference checks stop here.
		if err := node.Params.Decode(&params); err != nil || len(params.Map) == 0 {
			return nil, false
		}
		schema := refs.Schema{}
		for key := range params.Map {
			schema[key] = "any"
		}
		return schema, true
	default:
		return nil, false
	}
}

func (c *lintContext) toolOutputSchema(kind string) (refs.Schema, bool) {
	if c.linter.tools == nil {
		return nil, false
	}
	snapshot, ok := c.linter.tools.Snapshot(kind)
	if !ok || len(snapshot.OutputSchema) == 0 {
		return nil, false
	}
	schema, err := schemaFromJSON(snapshot.OutputSchema)
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
