package gate

import (
	"encoding/json"
	"fmt"
	"strings"
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
		decision, ok := parseRouteOverride(gate.OnResult[resultKey], placement)
		if ok && approvalRouteAllowed(decision.Action) {
			return decision
		}
		return defaultRoute(placement, verdict.Outcome)
	}
	if decision, ok := parseRouteOverride(gate.OnResult[resultKey], placement); ok {
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

func parseRouteOverride(value any, placement Placement) (RouteDecision, bool) {
	if value == nil {
		return RouteDecision{}, false
	}
	switch typed := value.(type) {
	case string:
		action := RouteAction(strings.TrimSpace(typed))
		return normalizeRouteDecision(RouteDecision{Placement: placement, Action: action}, placement)
	case map[string]any:
		return parseRouteMap(typed, placement)
	case map[string]string:
		routeMap := make(map[string]any, len(typed))
		for key, routeValue := range typed {
			routeMap[key] = routeValue
		}
		return parseRouteMap(routeMap, placement)
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(typed, &decoded); err != nil {
			return RouteDecision{}, false
		}
		return parseRouteOverride(decoded, placement)
	default:
		return RouteDecision{}, false
	}
}

func parseRouteMap(routeMap map[string]any, placement Placement) (RouteDecision, bool) {
	decision := RouteDecision{Placement: placement}
	if action, ok := routeString(routeMap["route"]); ok {
		decision.Action = RouteAction(action)
	}
	if branch, ok := routeString(routeMap["branch"]); ok {
		decision.Branch = branch
	}
	if status, ok := routeString(routeMap["terminal_status"]); ok {
		decision.TerminalStatus = status
	}
	if reason, ok := routeString(routeMap["reason_code"]); ok {
		decision.ReasonCode = reason
	}
	if decision.Action == "" {
		return RouteDecision{}, false
	}
	return normalizeRouteDecision(decision, placement)
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
	if decision.Action == RouteBranch && decision.Branch == "" {
		return RouteDecision{}, false
	}
	if decision.Action == RouteHalt && decision.TerminalStatus == "" {
		decision.TerminalStatus = terminalFailed
	}
	if decision.Action == RouteEscalate && decision.TerminalStatus == "" {
		decision.TerminalStatus = statusNeedsApproval
	}
	if decision.ReasonCode == "" {
		decision.ReasonCode = fmt.Sprintf("gate_route_%s", decision.Action)
	}
	return decision, true
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
		case RouteContinue, RouteRevise, RouteBranch, RouteHalt, RouteEscalate:
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
