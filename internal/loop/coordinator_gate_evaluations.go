package loop

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
)

type gateEvaluation struct {
	runtime        gate.Gate
	itemIndex      int
	verdict        gate.Verdict
	control        NodeControl
	evaluatedAt    time.Time
	tracksRevision bool
}

type gateEvaluationKey struct {
	gateID    string
	itemIndex int
}

type gateEvaluationCollector struct {
	values               map[gateEvaluationKey]gateEvaluation
	predicateDiagnostics []PredicateDiagnostic
}

func (c *gateEvaluationCollector) record(runtime gate.Gate, itemIndex int, verdict gate.Verdict) {
	c.recordEvaluation(gateEvaluation{runtime: runtime, itemIndex: itemIndex, verdict: verdict})
}

func (c *gateEvaluationCollector) recordWithControl(
	runtime gate.Gate,
	itemIndex int,
	verdict gate.Verdict,
	control NodeControl,
	evaluatedAt time.Time,
) {
	c.recordEvaluation(gateEvaluation{
		runtime: runtime, itemIndex: itemIndex, verdict: verdict, control: control,
		evaluatedAt: evaluatedAt.UTC(), tracksRevision: true,
	})
}

func (c *gateEvaluationCollector) recordEvaluation(evaluation gateEvaluation) {
	if c == nil {
		return
	}
	if c.values == nil {
		c.values = make(map[gateEvaluationKey]gateEvaluation)
	}
	key := gateEvaluationKey{
		gateID:    strings.TrimSpace(evaluation.runtime.ID),
		itemIndex: evaluation.itemIndex,
	}
	c.values[key] = evaluation
}

func (c *gateEvaluationCollector) recordPredicate(diagnostics ...PredicateDiagnostic) {
	if c == nil || len(diagnostics) == 0 {
		return
	}
	c.predicateDiagnostics = append(c.predicateDiagnostics, diagnostics...)
}

func (c *gateEvaluationCollector) ordered() []gateEvaluation {
	if c == nil || len(c.values) == 0 {
		return nil
	}
	keys := make([]gateEvaluationKey, 0, len(c.values))
	for key := range c.values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool {
		if keys[left].gateID != keys[right].gateID {
			return keys[left].gateID < keys[right].gateID
		}
		return keys[left].itemIndex < keys[right].itemIndex
	})
	ordered := make([]gateEvaluation, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, c.values[key])
	}
	return ordered
}

func (c *gateEvaluationCollector) routeCauses() []gateEvaluation {
	ordered := c.ordered()
	causes := make([]gateEvaluation, 0, len(ordered))
	for _, evaluation := range ordered {
		if routeCausesGeneration(evaluation.verdict.Route.Action) {
			causes = append(causes, evaluation)
		}
	}
	return causes
}

func (c *gateEvaluationCollector) gateRevisionMutations() []NodeControlMutation {
	type revisionState struct {
		control  NodeControl
		counters map[int]int
		at       time.Time
	}
	states := make(map[string]*revisionState)
	for _, evaluation := range c.ordered() {
		if !evaluation.tracksRevision || !gateRouteAdvancesRevision(evaluation.verdict.Route.Action) {
			continue
		}
		gateID := strings.TrimSpace(evaluation.runtime.ID)
		state := states[gateID]
		if state == nil {
			counters := make(map[int]int, len(evaluation.control.GateRevisions)+1)
			for itemIndex, revision := range evaluation.control.GateRevisions {
				counters[itemIndex] = revision
			}
			state = &revisionState{control: evaluation.control, counters: counters, at: evaluation.evaluatedAt}
			states[gateID] = state
		}
		state.counters[evaluation.itemIndex]++
		if evaluation.evaluatedAt.After(state.at) {
			state.at = evaluation.evaluatedAt
		}
	}
	gateIDs := make([]string, 0, len(states))
	for gateID := range states {
		gateIDs = append(gateIDs, gateID)
	}
	sort.Strings(gateIDs)
	mutations := make([]NodeControlMutation, 0, len(gateIDs))
	for _, gateID := range gateIDs {
		state := states[gateID]
		mutations = append(mutations, NodeControlMutation{
			Kind: NodeControlMutationGateRevision, NodeID: NodeID(gateID),
			ExpectedRevision: state.control.Revision, ExpectExisting: state.control.NodeID != "",
			GateRevisions: state.counters, At: state.at,
		})
	}
	return mutations
}

func gateRouteAdvancesRevision(action gate.RouteAction) bool {
	return action == gate.RouteRevise || action == gate.RouteNextGeneration
}

func routeCausesGeneration(action gate.RouteAction) bool {
	// Escalation and halt verdicts remain durable route causes for history; only
	// routeActionForCauses selects the subset that can produce a succession plan.
	switch action {
	case gate.RouteRevise, gate.RouteNextGeneration, gate.RouteEscalate, gate.RouteHalt:
		return true
	default:
		return false
	}
}

func (c *gateEvaluationCollector) intents(
	run Run,
	generation int,
) ([]gate.VerdictIntent, *gate.BestUpdateIntent, error) {
	ordered := c.ordered()
	if len(ordered) == 0 {
		return nil, nil, nil
	}
	ranks := make(map[gateEvaluationKey]int)
	for rank, evaluation := range c.routeCauses() {
		ranks[evaluation.key()] = rank
	}
	currentBest := currentBestIntent(run)
	intents := make([]gate.VerdictIntent, 0, len(ordered))
	var bestUpdate *gate.BestUpdateIntent
	for _, evaluation := range ordered {
		var rank *int
		if value, ok := ranks[evaluation.key()]; ok {
			rank = new(value)
		}
		intent, err := gate.NewVerdictIntent(
			evaluation.runtime.ID,
			evaluation.itemIndex,
			evaluation.verdict,
			rank,
		)
		if err != nil {
			return nil, nil, err
		}
		intents = append(intents, intent)
		candidate, err := gate.BestUpdateForVerdict(
			evaluation.runtime,
			evaluation.verdict,
			int64(generation),
			currentBest,
		)
		if err != nil {
			return nil, nil, err
		}
		if candidate != nil {
			if bestUpdate != nil {
				return nil, nil, fmt.Errorf("%w: multiple metric best updates in one generation", ErrValidation)
			}
			bestUpdate = candidate
		}
	}
	return intents, bestUpdate, nil
}

func (e gateEvaluation) key() gateEvaluationKey {
	return gateEvaluationKey{gateID: strings.TrimSpace(e.runtime.ID), itemIndex: e.itemIndex}
}

func currentBestIntent(run Run) *gate.BestUpdateIntent {
	if run.BestGeneration == nil || run.BestScore == nil {
		return nil
	}
	return &gate.BestUpdateIntent{Generation: *run.BestGeneration, Score: *run.BestScore}
}

func routeCauseIDs(evaluations []gateEvaluation) []string {
	ids := make([]string, 0, len(evaluations))
	for _, evaluation := range evaluations {
		ids = append(ids, strings.TrimSpace(evaluation.runtime.ID))
	}
	sort.Strings(ids)
	return ids
}

func routeActionForCauses(evaluations []gateEvaluation) gate.RouteAction {
	action := gate.RouteAction("")
	for _, evaluation := range evaluations {
		switch evaluation.verdict.Route.Action {
		case gate.RouteNextGeneration:
			return gate.RouteNextGeneration
		case gate.RouteRevise:
			action = gate.RouteRevise
		}
	}
	return action
}
