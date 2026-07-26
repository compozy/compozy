package loop

import (
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
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
	var params dsl.RunAgentParams
	if err := node.Params.Decode(&params); err != nil {
		c.add(node.ID, refs.CodeUnresolvablePath, "run-agent params are invalid: %v", err)
		return
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
