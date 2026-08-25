package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func resolveDeclaredOutputSchema(
	definition dsl.Definition,
	tools ToolSchemaSource,
	node dsl.Node,
) (refs.Schema, bool, error) {
	if len(node.Produces) > 0 {
		if node.Class == dsl.NodeClassControl {
			switch dsl.ControlKind(node.Kind) {
			case dsl.ControlBranch, dsl.ControlRoute:
				return nil, false, fmt.Errorf("control %q has no payload contract", node.Kind)
			}
		}
		return convertSchema(node.Produces), true, nil
	}
	switch node.Class {
	case dsl.NodeClassSource:
		return resolveSourceOutputSchema(definition, node)
	case dsl.NodeClassAction:
		return resolveActionOutputSchema(tools, node)
	case dsl.NodeClassControl:
		if dsl.ControlKind(node.Kind) != dsl.ControlAsk {
			return nil, false, nil
		}
		var params dsl.AskParams
		if err := node.Params.Decode(&params); err != nil {
			return nil, false, fmt.Errorf("decode ask output schema: %w", err)
		}
		if len(params.Expect) == 0 {
			return nil, false, nil
		}
		return convertSchema(params.Expect), true, nil
	default:
		return nil, false, nil
	}
}

func resolveSourceOutputSchema(definition dsl.Definition, node dsl.Node) (refs.Schema, bool, error) {
	switch dsl.SourceKind(node.Kind) {
	case dsl.SourceInput:
		input, ok := definition.Inputs[node.InputRef]
		if !ok {
			return nil, false, nil
		}
		return inputSchema(input), true, nil
	case dsl.SourceWatchEvents:
		return watchEventsOutputSchema(), true, nil
	default:
		return nil, false, nil
	}
}

func resolveActionOutputSchema(tools ToolSchemaSource, node dsl.Node) (refs.Schema, bool, error) {
	if !dsl.IsReservedActionKind(node.Kind) {
		if tools == nil {
			return nil, false, nil
		}
		snapshot, ok := tools.Snapshot(node.Kind)
		if !ok || len(snapshot.OutputSchema) == 0 {
			return nil, false, nil
		}
		schema, err := schemaFromJSON(snapshot.OutputSchema)
		if err != nil {
			return nil, false, err
		}
		return schema, true, nil
	}
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionGoal:
		params, err := decodeGoalNodeParams(node.Params)
		if err != nil {
			return nil, false, err
		}
		if params.OutputSchema == nil {
			return nil, false, nil
		}
		return convertSchema(*params.OutputSchema), true, nil
	case dsl.ActionRunAgent:
		params, err := decodeRunAgentNodeParams(node.Params)
		if err != nil {
			return nil, false, err
		}
		if len(params.OutputSchema) == 0 {
			return nil, false, nil
		}
		return convertSchema(params.OutputSchema), true, nil
	case dsl.ActionRunLoop:
		return refs.Schema{reasonMetaStatus: jsonSchemaStringType, "outputs": map[string]any{}}, true, nil
	case dsl.ActionTransform:
		var params dsl.TransformParams
		if err := node.Params.Decode(&params); err != nil {
			return nil, false, err
		}
		if len(params.Map) == 0 {
			return nil, false, nil
		}
		schema := refs.Schema{}
		for key := range params.Map {
			schema[key] = "any"
		}
		return schema, true, nil
	default:
		return nil, false, nil
	}
}

func (c *lintContext) declaredSchema(node dsl.Node) (refs.Schema, bool) {
	schema, ok, err := resolveDeclaredOutputSchema(c.def, c.linter.tools, node)
	if err != nil {
		return nil, false
	}
	return schema, ok
}

func resolvedDefinitionOutputSchema(resolved *ResolvedDefinition, node dsl.Node) (refs.Schema, error) {
	if resolved == nil {
		return nil, fmt.Errorf("%w: resolved definition is required", ErrValidation)
	}
	schema, ok, err := resolveDeclaredOutputSchema(
		resolved.Definition,
		historyToolSchemaSource(resolved.ToolSchemas),
		node,
	)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrAmendSchemaMissing
	}
	return schema, nil
}
