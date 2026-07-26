package loop

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/task"
)

const definitionOfDoneGateID = "definition_of_done"

func definitionOfDoneTerminal(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	topology controlTopology,
	evaluator gate.GateEvaluator,
	decisions GateDecisionReader,
	outputs []GenerationOutput,
) (*task.CoordinatorTerminal, error) {
	terminal := &task.CoordinatorTerminal{
		Status: string(StatusDone),
		Cause:  string(TransitionCauseContract),
	}
	if evaluator == nil || resolved == nil || len(resolved.Definition.Contract.Verification) == 0 {
		return terminal, nil
	}
	namespace, err := definitionOfDoneNamespace(run, generation, resolved, topology, outputs)
	if err != nil {
		return nil, err
	}
	runtimeGate, empty, err := runtimeDefinitionOfDoneGate(resolved, effective, namespace)
	if err != nil {
		return nil, err
	}
	if empty {
		return approvedDefinitionOfDoneTerminal(terminal)
	}
	humanDecisions, err := loadGateDecisions(ctx, decisions, run, generation, dsl.NodeID(runtimeGate.ID))
	if err != nil {
		return nil, err
	}
	verdict, err := evaluator.Evaluate(ctx, runtimeGate, runtimeGateInput(
		run,
		generation,
		resolved,
		effective,
		namespace,
		gate.PlacementDefinitionOfDone,
		humanDecisions,
	))
	if err != nil {
		return nil, err
	}
	return terminalFromDefinitionOfDoneVerdict(terminal, verdict)
}

func definitionOfDoneNamespace(
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	outputs []GenerationOutput,
) (map[string]any, error) {
	return runtimeNamespace(run, generation, resolved.Definition.Graph, topology, outputs, "", 0)
}

func runtimeDefinitionOfDoneGate(
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	namespace map[string]any,
) (gate.Gate, bool, error) {
	runtimeGate := gate.GateFromContract(
		definitionOfDoneGateID,
		resolved.Definition.Contract,
		effective.GateMaxRevisions,
	)
	for index := range runtimeGate.Criteria {
		criterion, err := renderGateCriterion("", runtimeGate.Criteria[index], namespace)
		if err != nil {
			return gate.Gate{}, false, err
		}
		runtimeGate.Criteria[index] = criterion
	}
	return applyEffectiveGateConfig(runtimeGate, effective)
}

func approvedDefinitionOfDoneTerminal(
	terminal *task.CoordinatorTerminal,
) (*task.CoordinatorTerminal, error) {
	ref, err := gateApprovedOutputRef()
	if err != nil {
		return nil, err
	}
	terminal.Details = []byte(ref)
	return terminal, nil
}

func terminalFromDefinitionOfDoneVerdict(
	terminal *task.CoordinatorTerminal,
	verdict gate.Verdict,
) (*task.CoordinatorTerminal, error) {
	ref, err := gateVerdictOutputRef(verdict)
	if err != nil {
		return nil, err
	}
	switch verdict.Route.Action {
	case gate.RouteDone:
		terminal.Details = []byte(ref)
		return terminal, nil
	case gate.RouteEscalate, gate.RouteHalt:
		return gateRouteTerminal(definitionOfDoneGateID, verdict.Route, ref), nil
	case gate.RouteNextGeneration:
		return &task.CoordinatorTerminal{
			Status:     string(StatusFailed),
			Cause:      string(TransitionCauseContract),
			ReasonCode: verdict.Route.ReasonCode,
			Details:    []byte(ref),
		}, nil
	default:
		return nil, fmt.Errorf(
			"%w: definition-of-done gate returned unsupported route action %q",
			ErrValidation,
			verdict.Route.Action,
		)
	}
}
