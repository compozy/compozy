package loop

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/loop/gate"
)

var gateResultKeys = map[string]struct{}{
	"pass": {}, "approval": {}, "fail": {}, "blocked": {}, "error": {},
	"timeout": {}, "invalid_output": {},
}

func (c *lintContext) lintRoute(node dsl.Node) {
	if node.OnEvalError == dsl.EvalErrorExit {
		c.add(
			node.ID,
			CodeEvalErrorPolicyInvalid,
			"route conditions are fail-closed; on_eval_error must be fail",
		)
	}
	if strings.TrimSpace(string(node.Default)) == "" {
		c.add(node.ID, CodeRouteDefaultMissing, "route node %q must declare default", node.ID)
	}
	if len(node.Routes) == 0 {
		c.add(node.ID, CodeRouteTargetInvalid, "route node %q must declare at least one conditional route", node.ID)
	}

	declared := make(map[dsl.NodeID]string, len(node.Routes)+1)
	for index, route := range node.Routes {
		path := fmt.Sprintf("routes[%d].to", index)
		if strings.TrimSpace(route.When) == "" {
			c.add(node.ID, refs.CodeConditionNotBool, "routes[%d].when is required", index)
		}
		c.lintRouteDestination(node, route.To, path, declared)
	}
	if node.Default != "" {
		c.lintRouteDestination(node, node.Default, "default", declared)
	}
	for _, target := range c.adjacency[node.ID] {
		if _, ok := declared[target]; !ok {
			c.add(
				node.ID,
				CodeRouteTargetInvalid,
				"forward edge %q -> %q is not declared by routes or default",
				node.ID,
				target,
			)
		}
	}
}

func (c *lintContext) lintRouteDestination(
	node dsl.Node,
	target dsl.NodeID,
	path string,
	declared map[dsl.NodeID]string,
) {
	if strings.TrimSpace(string(target)) == "" || !containsNodeID(c.adjacency[node.ID], target) {
		c.add(
			node.ID,
			CodeRouteTargetInvalid,
			"%s %q must name a declared direct forward destination from %q",
			path,
			target,
			node.ID,
		)
		return
	}
	if previous, exists := declared[target]; exists {
		c.add(
			node.ID,
			CodeRouteTargetInvalid,
			"%s duplicates destination %q already declared by %s",
			path,
			target,
			previous,
		)
		return
	}
	declared[target] = path
}

func (c *lintContext) lintGateRoutes(node dsl.Node) {
	for outcome, mapping := range node.OnResult {
		if _, ok := gateResultKeys[outcome]; !ok {
			c.add(node.ID, CodeRouteMappingInvalid, "on_result outcome %q is not supported", outcome)
			continue
		}
		switch typed := mapping.(type) {
		case string:
			c.lintGateStringRoute(node, outcome, typed)
		case map[string]any:
			c.lintGateObjectRoute(node, outcome, typed)
		case map[string]string:
			converted := make(map[string]any, len(typed))
			for key, value := range typed {
				converted[key] = value
			}
			c.lintGateObjectRoute(node, outcome, converted)
		default:
			c.add(node.ID, CodeRouteMappingInvalid, "on_result.%s must be a string action or {route: node_id}", outcome)
		}
	}
}

func (c *lintContext) lintGateStringRoute(node dsl.Node, outcome, raw string) {
	action := gate.RouteAction(strings.TrimSpace(raw))
	if action == gate.RouteAction("branch") {
		c.add(
			node.ID,
			CodeRouteActionRemoved,
			"on_result.%s action branch was removed; use {route: node_id}",
			outcome,
		)
		return
	}
	if !gateStringRouteAllowed(action) || outcome == "approval" &&
		action != gate.RouteEscalate && action != gate.RouteHalt {
		c.add(node.ID, CodeRouteMappingInvalid, "on_result.%s action %q is not legal", outcome, raw)
	}
}

func (c *lintContext) lintGateObjectRoute(node dsl.Node, outcome string, mapping map[string]any) {
	if len(mapping) != 1 {
		c.add(
			node.ID,
			CodeRouteMappingInvalid,
			"on_result.%s must contain exactly one route field",
			outcome,
		)
		return
	}
	raw, ok := mapping["route"]
	target, stringOK := raw.(string)
	target = strings.TrimSpace(target)
	if !ok || !stringOK || target == "" {
		c.add(node.ID, CodeRouteMappingInvalid, "on_result.%s.route must be a node id", outcome)
		return
	}
	if outcome == "approval" {
		c.add(node.ID, CodeRouteMappingInvalid, "on_result.approval cannot bypass a pending approval")
		return
	}
	if !containsNodeID(c.adjacency[node.ID], dsl.NodeID(target)) {
		c.add(
			node.ID,
			CodeRouteTargetInvalid,
			"on_result.%s.route %q must be a declared direct forward destination from %q",
			outcome,
			target,
			node.ID,
		)
	}
}

func gateStringRouteAllowed(action gate.RouteAction) bool {
	switch action {
	case gate.RouteContinue, gate.RouteRevise, gate.RouteNextGeneration,
		gate.RouteHalt, gate.RouteEscalate:
		return true
	default:
		return false
	}
}
