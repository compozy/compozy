package loop

import (
	"maps"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func (c *lintContext) lintReservedActionNode(node dsl.Node) {
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionRunAgent:
		c.lintRunAgentNode(node)
	case dsl.ActionRunLoop:
		c.lintRunLoopNode(node)
	case dsl.ActionTransform:
		c.lintTransformNode(node)
	case dsl.ActionGoal:
		c.lintGoalNode(node)
	}
}

func (c *lintContext) lintRunAgentNode(node dsl.Node) {
	params, err := decodeRunAgentNodeParams(node.Params)
	if err != nil {
		c.add(node.ID, refs.CodeUnresolvablePath, "run-agent params are invalid: %v", err)
		return
	}
	if _, exists := params.Extra["cwd"]; exists {
		c.add(node.ID, CodeEnvironmentCWDRemoved, "params.cwd is retired; use params.environment.directory")
	}
	if _, err := ResolveActionEnvironment(params.Environment, dsl.EnvironmentSpec{}); err != nil {
		c.add(node.ID, CodeEnvironmentInvalid, "run-agent environment is invalid: %v", err)
	}
	if strings.TrimSpace(params.Agent) == "" {
		c.add(node.ID, refs.CodeUnresolvablePath, "run-agent params.agent is required")
	}
	if strings.TrimSpace(params.Prompt) == "" {
		c.add(node.ID, refs.CodeUnresolvablePath, "run-agent params.prompt is required")
	}
}

func (c *lintContext) lintRunLoopNode(node dsl.Node) {
	var params dsl.RunLoopParams
	if err := node.Params.Decode(&params); err != nil {
		c.add(node.ID, refs.CodeUnresolvablePath, "run-loop params are invalid: %v", err)
		return
	}
	if strings.TrimSpace(params.Loop) == "" {
		c.add(node.ID, refs.CodeUnresolvablePath, "run-loop params.loop is required")
	}
	if params.Mode != "" && params.Mode != dsl.RunLoopAwait && params.Mode != dsl.RunLoopDetach {
		c.add(node.ID, refs.CodeUnresolvablePath, "run-loop params.mode must be await or detach")
	}
	if len(node.Produces) > 0 {
		if params.Mode == dsl.RunLoopDetach {
			if len(node.Produces) != 1 || !runLoopOutputFieldIsString(node.Produces["loop_run_id"]) {
				c.add(
					node.ID,
					CodeRunLoopOutputShapeInvalid,
					"detached run-loop produces must declare only loop_run_id as a string",
				)
			}
		} else if !declaresCompletedRunLoopResult(node) {
			c.add(
				node.ID,
				CodeRunLoopOutputShapeInvalid,
				"awaited run-loop produces must declare loop_run_id and status as strings",
			)
		}
	}
	c.lintRunLoopConfigOverrides(node.ID, params.ConfigOverrides)
}

func declaresCompletedRunLoopResult(node dsl.Node) bool {
	return len(node.Produces) == 2 &&
		runLoopOutputFieldIsString(node.Produces["loop_run_id"]) &&
		runLoopOutputFieldIsString(node.Produces["status"])
}

func runLoopOutputFieldIsString(value any) bool {
	switch schema := normalizeSchemaValue(value).(type) {
	case string:
		return schema == jsonSchemaStringType
	case map[string]any:
		return len(schema) == 1 && schema[jsonSchemaTypeKey] == jsonSchemaStringType
	default:
		return false
	}
}

func (c *lintContext) lintRunLoopConfigOverrides(nodeID dsl.NodeID, raw map[string]any) {
	literals := maps.Clone(raw)
	for key, value := range literals {
		templateValue, ok := value.(string)
		if !ok {
			continue
		}
		if _, direct := directTemplateReferencePath(templateValue); direct {
			literals[key] = nil
		}
	}
	if _, err := decodeRunLoopConfigOverrides(literals); err != nil {
		c.add(nodeID, refs.CodeUnresolvablePath, "run-loop params.config_overrides are invalid: %v", err)
	}
}

func (c *lintContext) lintTransformNode(node dsl.Node) {
	var params dsl.TransformParams
	if err := node.Params.Decode(&params); err != nil {
		c.add(node.ID, refs.CodeUnresolvablePath, "transform params are invalid: %v", err)
		return
	}
	if len(params.Map) == 0 {
		c.add(node.ID, refs.CodeUnresolvablePath, "transform params.map is required")
	}
}
