package gate

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	resultKeyPass          = "pass"
	resultKeyApproval      = "approval"
	resultKeyFail          = "fail"
	resultKeyBlocked       = "blocked"
	resultKeyError         = "error"
	resultKeyTimeout       = "timeout"
	resultKeyInvalidOutput = "invalid_output"

	terminalBlocked     = "blocked"
	terminalFailed      = "failed"
	terminalExhausted   = "exhausted"
	terminalStalled     = "stalled"
	statusNeedsApproval = "needs-approval"
)

func (e *Evaluator) route(gate Gate, verdict Verdict, in GateInput) RouteDecision {
	placement := normalizePlacement(in.Placement)
	if verdict.Broken {
		limit := in.BrokenJudgeStreakLimit
		if limit <= 0 {
			limit = e.brokenJudgeStreakLimit
		}
		if verdict.NextBrokenJudgeStreak >= limit {
			return RouteDecision{
				Placement:      placement,
				Action:         RouteHalt,
				TerminalStatus: terminalStalled,
				ReasonCode:     "broken_judge_streak",
			}
		}
		// Broken judge transport is unknowable, so ADR-012 fail-open routing wins
		// until the streak guard trips; max_revisions only applies to real verdicts.
		if placement == PlacementDefinitionOfDone {
			return RouteDecision{
				Placement:  placement,
				Action:     RouteNextGeneration,
				ReasonCode: "broken_judge_fail_open",
			}
		}
		return RouteDecision{
			Placement:  placement,
			Action:     RouteContinue,
			ReasonCode: "broken_judge_fail_open",
		}
	}
	if shouldExhaust(gate, verdict, in) {
		return RouteDecision{
			Placement:      placement,
			Action:         RouteHalt,
			TerminalStatus: terminalExhausted,
			ReasonCode:     "gate_max_revisions_exhausted",
		}
	}
	resultKey := routeResultKey(verdict.Outcome)
	if resultKey == resultKeyApproval {
		decision, ok := parseRouteOverride(gate.OnResult[resultKey], placement, in.Contract)
		if ok && approvalRouteAllowed(decision.Action) {
			return decision
		}
		return defaultRoute(placement, verdict.Outcome)
	}
	if decision, ok := parseRouteOverride(gate.OnResult[resultKey], placement, in.Contract); ok {
		return decision
	}
	return defaultRoute(placement, verdict.Outcome)
}

func normalizePlacement(placement Placement) Placement {
	switch placement {
	case PlacementDefinitionOfDone:
		return PlacementDefinitionOfDone
	default:
		return PlacementInBody
	}
}

func shouldExhaust(gate Gate, verdict Verdict, in GateInput) bool {
	if gate.MaxRevisions <= 0 {
		return false
	}
	if verdict.Outcome == VerdictOutcomeApproved || verdict.Outcome == VerdictOutcomeAwaitingApproval {
		return false
	}
	return in.Revision >= gate.MaxRevisions
}

func routeResultKey(outcome VerdictOutcome) string {
	switch outcome {
	case VerdictOutcomeApproved:
		return resultKeyPass
	case VerdictOutcomeAwaitingApproval:
		return resultKeyApproval
	case VerdictOutcomeRejected:
		return resultKeyFail
	case VerdictOutcomeBlocked:
		return resultKeyBlocked
	case VerdictOutcomeTimeout:
		return resultKeyTimeout
	case VerdictOutcomeInvalidOutput:
		return resultKeyInvalidOutput
	default:
		return resultKeyError
	}
}

func parseRouteOverride(value any, placement Placement, contract *dsl.Contract) (RouteDecision, bool) {
	if value == nil {
		return RouteDecision{}, false
	}
	switch typed := value.(type) {
	case string:
		action := RouteAction(strings.TrimSpace(typed))
		return normalizeRouteDecision(RouteDecision{Placement: placement, Action: action}, placement)
	case map[string]any:
		return parseRouteMap(typed, placement, contract)
	case map[string]string:
		routeMap := make(map[string]any, len(typed))
		for key, routeValue := range typed {
			routeMap[key] = routeValue
		}
		return parseRouteMap(routeMap, placement, contract)
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(typed, &decoded); err != nil {
			return RouteDecision{}, false
		}
		return parseRouteOverride(decoded, placement, contract)
	default:
		return RouteDecision{}, false
	}
}

func parseRouteMap(routeMap map[string]any, placement Placement, contract *dsl.Contract) (RouteDecision, bool) {
	if target, ok := routeString(routeMap["route"]); ok {
		if len(routeMap) != 1 || placement != PlacementInBody {
			return RouteDecision{}, false
		}
		return normalizeRouteDecision(RouteDecision{
			Placement:  placement,
			Action:     RouteContinue,
			Target:     target,
			ReasonCode: fmt.Sprintf("gate_route_%s", target),
		}, placement)
	}
	rawAction, ok := routeString(routeMap["action"])
	if !ok {
		return RouteDecision{}, false
	}
	for key := range routeMap {
		if key != "action" && key != "terminal_status" {
			return RouteDecision{}, false
		}
	}
	decision := RouteDecision{Placement: placement, Action: RouteAction(rawAction)}
	if rawTerminal, exists := routeMap["terminal_status"]; exists {
		terminal, terminalOK := routeString(rawTerminal)
		if !terminalOK || decision.Action != RouteHalt {
			return RouteDecision{}, false
		}
		if !authoredTerminalStatusAllowed(terminal, contract) {
			return RouteDecision{}, false
		}
		decision.TerminalStatus = terminal
	}
	return normalizeRouteDecision(decision, placement)
}

// authoredTerminalStatusAllowed validates an authored terminal_status against the
// closed terminal enum and, when declared, the contract's terminal_states set.
func authoredTerminalStatusAllowed(value string, contract *dsl.Contract) bool {
	state := dsl.TerminalState(value)
	if !dsl.IsKnownTerminalState(state) {
		return false
	}
	if contract == nil || contract.ContractLifecycleState == nil || len(contract.TerminalStates) == 0 {
		return true
	}
	return slices.Contains(contract.TerminalStates, state)
}

func routeString(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	return text, text != ""
}

func normalizeRouteDecision(decision RouteDecision, placement Placement) (RouteDecision, bool) {
	decision.Placement = placement
	if placement == PlacementDefinitionOfDone && decision.Action == RouteContinue {
		decision.Action = RouteDone
	}
	if !routeAllowed(decision.Action, placement) {
		return RouteDecision{}, false
	}
	if decision.TerminalStatus == "" {
		decision.TerminalStatus = DefaultTerminalStatus(decision.Action)
	}
	if decision.ReasonCode == "" {
		decision.ReasonCode = fmt.Sprintf("gate_route_%s", decision.Action)
	}
	return decision, true
}

// DefaultTerminalStatus is the status an action settles with when the author
// did not override it. It is shared by routing and terminal construction so the
// two conventions cannot drift.
func DefaultTerminalStatus(action RouteAction) string {
	switch action {
	case RouteHalt:
		return terminalFailed
	case RouteEscalate:
		return statusNeedsApproval
	default:
		return ""
	}
}

// RouteAllowed reports whether action is legal for the gate placement.
func RouteAllowed(action RouteAction, placement Placement) bool {
	return routeAllowed(action, placement)
}

func routeAllowed(action RouteAction, placement Placement) bool {
	switch placement {
	case PlacementDefinitionOfDone:
		switch action {
		case RouteDone, RouteNextGeneration, RouteHalt, RouteEscalate:
			return true
		default:
			return false
		}
	default:
		switch action {
		case RouteContinue, RouteRevise, RouteNextGeneration, RouteHalt, RouteEscalate:
			return true
		default:
			return false
		}
	}
}

func approvalRouteAllowed(action RouteAction) bool {
	switch action {
	case RouteEscalate, RouteHalt:
		return true
	default:
		return false
	}
}

func defaultRoute(placement Placement, outcome VerdictOutcome) RouteDecision {
	decision := RouteDecision{Placement: placement}
	switch outcome {
	case VerdictOutcomeApproved:
		if placement == PlacementDefinitionOfDone {
			decision.Action = RouteDone
			decision.ReasonCode = "gate_passed_done"
		} else {
			decision.Action = RouteContinue
			decision.ReasonCode = "gate_passed_continue"
		}
	case VerdictOutcomeAwaitingApproval:
		decision.Action = RouteEscalate
		decision.TerminalStatus = statusNeedsApproval
		decision.ReasonCode = blockerHumanPending
	case VerdictOutcomeRejected:
		if placement == PlacementDefinitionOfDone {
			decision.Action = RouteNextGeneration
			decision.ReasonCode = "gate_rejected_next_generation"
		} else {
			decision.Action = RouteRevise
			decision.ReasonCode = "gate_rejected_revise"
		}
	case VerdictOutcomeBlocked:
		decision.Action = RouteHalt
		decision.TerminalStatus = terminalBlocked
		decision.ReasonCode = "gate_blocked"
	case VerdictOutcomeTimeout:
		decision.Action = RouteHalt
		decision.TerminalStatus = terminalFailed
		decision.ReasonCode = "gate_timeout"
	case VerdictOutcomeInvalidOutput:
		decision.Action = RouteHalt
		decision.TerminalStatus = terminalFailed
		decision.ReasonCode = "gate_invalid_output"
	default:
		decision.Action = RouteHalt
		decision.TerminalStatus = terminalFailed
		decision.ReasonCode = "gate_error"
	}
	return decision
}
