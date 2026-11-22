package loop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/loop/gate"
)

var gateResultKeys = map[string]struct{}{
	"pass": {}, gateResultApprovalKey: {}, "fail": {}, string(StatusBlocked): {}, "error": {},
	gateResultTimeoutKey: {}, "invalid_output": {},
}

const (
	gateResultApprovalKey = "approval"
	gateResultTimeoutKey  = "timeout"
)

// onResultScope describes where an authored on_result map lives: an in-body gate
// node or the synthetic contract/definition-of-done gate.
type onResultScope struct {
	owner     dsl.NodeID
	label     string
	placement gate.Placement
	adjacency []dsl.NodeID
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
	c.lintOnResult(onResultScope{
		owner:     node.ID,
		label:     "on_result",
		placement: gate.PlacementInBody,
		adjacency: c.adjacency[node.ID],
	}, node.OnResult)
}

func (c *lintContext) lintContractOnResult() {
	if c.def.Contract.ContractLifecycleState == nil {
		return
	}
	c.lintOnResult(onResultScope{
		owner:     "",
		label:     "contract.on_result",
		placement: gate.PlacementDefinitionOfDone,
	}, c.def.Contract.OnResult)
}

func (c *lintContext) lintOnResult(scope onResultScope, onResult map[string]any) {
	for outcome, mapping := range onResult {
		if _, ok := gateResultKeys[outcome]; !ok {
			c.add(scope.owner, CodeRouteMappingInvalid, "%s outcome %q is not supported", scope.label, outcome)
			continue
		}
		switch typed := mapping.(type) {
		case string:
			c.lintGateStringRoute(scope, outcome, typed)
		case map[string]any:
			c.lintGateObjectRoute(scope, outcome, typed)
		case map[string]string:
			converted := make(map[string]any, len(typed))
			for key, value := range typed {
				converted[key] = value
			}
			c.lintGateObjectRoute(scope, outcome, converted)
		default:
			c.add(
				scope.owner,
				CodeRouteMappingInvalid,
				"%s.%s must be a string action, {route: node_id}, or {action: action, terminal_status: state}",
				scope.label,
				outcome,
			)
		}
	}
}

func (c *lintContext) lintGateStringRoute(scope onResultScope, outcome, raw string) {
	action := gate.RouteAction(strings.TrimSpace(raw))
	if action == gate.RouteAction("branch") {
		c.add(
			scope.owner,
			CodeRouteActionRemoved,
			"%s.%s action branch was removed; use {route: node_id}",
			scope.label,
			outcome,
		)
		return
	}
	if !gateStringRouteAllowed(action, scope.placement) ||
		outcome == string(TransitionCauseApproval) &&
			action != gate.RouteEscalate && action != gate.RouteHalt {
		c.add(scope.owner, CodeRouteMappingInvalid, "%s.%s action %q is not legal", scope.label, outcome, raw)
	}
}

func (c *lintContext) lintGateObjectRoute(scope onResultScope, outcome string, mapping map[string]any) {
	if raw, ok := mapping["route"]; ok {
		c.lintGateRouteTarget(scope, outcome, raw, len(mapping))
		return
	}
	c.lintGateActionObject(scope, outcome, mapping)
}

func (c *lintContext) lintGateRouteTarget(
	scope onResultScope,
	outcome string,
	raw any,
	fieldCount int,
) {
	target, stringOK := raw.(string)
	target = strings.TrimSpace(target)
	if fieldCount != 1 {
		c.add(
			scope.owner,
			CodeRouteMappingInvalid,
			"%s.%s must contain exactly one route field",
			scope.label,
			outcome,
		)
		return
	}
	if !stringOK || target == "" {
		c.add(scope.owner, CodeRouteMappingInvalid, "%s.%s.route must be a node id", scope.label, outcome)
		return
	}
	if scope.placement != gate.PlacementInBody {
		c.add(
			scope.owner,
			CodeRouteMappingInvalid,
			"%s.%s.route is not supported on the contract gate; use {action: halt, terminal_status: state}",
			scope.label,
			outcome,
		)
		return
	}
	if outcome == gateResultApprovalKey {
		c.add(scope.owner, CodeRouteMappingInvalid, "%s.approval cannot bypass a pending approval", scope.label)
		return
	}
	if !containsNodeID(scope.adjacency, dsl.NodeID(target)) {
		c.add(
			scope.owner,
			CodeRouteTargetInvalid,
			"%s.%s.route %q must be a declared direct forward destination from %q",
			scope.label,
			outcome,
			target,
			scope.owner,
		)
	}
}

func (c *lintContext) lintGateActionObject(scope onResultScope, outcome string, mapping map[string]any) {
	rawAction, ok := mapping["action"].(string)
	action := gate.RouteAction(strings.TrimSpace(rawAction))
	if !ok || action == "" {
		c.add(
			scope.owner,
			CodeRouteMappingInvalid,
			"%s.%s must declare action (string) or route (node id)",
			scope.label,
			outcome,
		)
		return
	}
	for key := range mapping {
		if key != "action" && key != "terminal_status" {
			c.add(
				scope.owner,
				CodeRouteMappingInvalid,
				"%s.%s field %q is not supported; use action and terminal_status",
				scope.label,
				outcome,
				key,
			)
			return
		}
	}
	if !gateStringRouteAllowed(action, scope.placement) ||
		outcome == string(TransitionCauseApproval) &&
			action != gate.RouteEscalate && action != gate.RouteHalt {
		c.add(scope.owner, CodeRouteMappingInvalid, "%s.%s action %q is not legal", scope.label, outcome, rawAction)
		return
	}
	rawTerminal, exists := mapping["terminal_status"]
	if !exists {
		return
	}
	terminal, terminalOK := rawTerminal.(string)
	terminal = strings.TrimSpace(terminal)
	if !terminalOK || terminal == "" {
		c.add(
			scope.owner,
			CodeRouteMappingInvalid,
			"%s.%s.terminal_status must be a terminal state",
			scope.label,
			outcome,
		)
		return
	}
	if action != gate.RouteHalt {
		c.add(
			scope.owner,
			CodeRouteMappingInvalid,
			"%s.%s.terminal_status requires action halt, got %q",
			scope.label,
			outcome,
			rawAction,
		)
		return
	}
	c.lintAuthoredTerminalStatus(scope, outcome, terminal)
}

func (c *lintContext) lintAuthoredTerminalStatus(scope onResultScope, outcome, terminal string) {
	state := dsl.TerminalState(terminal)
	if !dsl.IsKnownTerminalState(state) {
		c.add(
			scope.owner,
			CodeRouteMappingInvalid,
			"%s.%s.terminal_status %q is not in the closed enum: %s",
			scope.label,
			outcome,
			terminal,
			joinTerminalStates(dsl.TerminalStateValues()),
		)
		return
	}
	declared := c.def.Contract.TerminalStates
	if len(declared) == 0 || slices.Contains(declared, state) {
		return
	}
	c.add(
		scope.owner,
		CodeRouteMappingInvalid,
		"%s.%s.terminal_status %q is not declared by contract.terminal_states (allowed: %s)",
		scope.label,
		outcome,
		terminal,
		joinTerminalStates(declared),
	)
}

func joinTerminalStates(states []dsl.TerminalState) string {
	values := make([]string, 0, len(states))
	for _, state := range states {
		values = append(values, string(state))
	}
	return strings.Join(values, ", ")
}

func gateStringRouteAllowed(action gate.RouteAction, placement gate.Placement) bool {
	// The DoD placement maps an authored continue to done at runtime.
	if placement == gate.PlacementDefinitionOfDone && action == gate.RouteContinue {
		return true
	}
	return gate.RouteAllowed(action, placement)
}
