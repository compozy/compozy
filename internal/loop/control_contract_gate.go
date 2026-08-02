package loop

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

// definitionOfDoneGateID is reserved by the linter for the synthetic contract gate.
const definitionOfDoneGateID = "contract"

type definitionOfDoneEvaluation struct {
	terminal *task.CoordinatorTerminal
	gate     *definitionOfDoneGateEvaluation
}

type definitionOfDoneGateEvaluation struct {
	runtime gate.Gate
	verdict gate.Verdict
}

func evaluateDefinitionOfDone(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	topology controlTopology,
	evaluator gate.GateEvaluator,
	decisions GateDecisionReader,
	runtimeCatalog WorkspaceRuntimeCatalog,
	outputs []GenerationOutput,
	history GenerationHistory,
) (definitionOfDoneEvaluation, error) {
	terminal := &task.CoordinatorTerminal{
		Status: string(StatusDone),
		Cause:  string(TransitionCauseContract),
	}
	if evaluator == nil || resolved == nil || len(resolved.Definition.Contract.Verification) == 0 {
		return definitionOfDoneEvaluation{terminal: terminal}, nil
	}
	namespace, err := definitionOfDoneNamespace(run, generation, resolved, topology, outputs, history)
	if err != nil {
		return definitionOfDoneEvaluation{}, err
	}
	runtimeGate, empty, err := runtimeDefinitionOfDoneGate(resolved, effective, namespace)
	if err != nil {
		return definitionOfDoneEvaluation{}, err
	}
	if empty {
		approved, approveErr := approvedDefinitionOfDoneTerminal(terminal)
		return definitionOfDoneEvaluation{terminal: approved}, approveErr
	}
	if err := validateJudgeGateRuntimes(
		ctx,
		runtimeCatalog,
		run.WorkspaceID,
		effective.RuntimeDefaults.Judge,
		runtimeGate.Criteria,
	); err != nil {
		return definitionOfDoneEvaluation{}, err
	}
	humanDecisions, err := loadGateDecisions(ctx, decisions, run, generation, dsl.NodeID(runtimeGate.ID))
	if err != nil {
		return definitionOfDoneEvaluation{}, err
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
		return definitionOfDoneEvaluation{}, err
	}
	verdict, err = gate.SanitizeVerdict(verdict)
	if err != nil {
		return definitionOfDoneEvaluation{}, err
	}
	result, err := terminalFromDefinitionOfDoneVerdict(terminal, verdict)
	if err != nil {
		return definitionOfDoneEvaluation{}, err
	}
	return definitionOfDoneEvaluation{
		terminal: result,
		gate:     &definitionOfDoneGateEvaluation{runtime: runtimeGate, verdict: verdict},
	}, nil
}

func definitionOfDoneNamespace(
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	outputs []GenerationOutput,
	history GenerationHistory,
) (map[string]any, error) {
	return runtimeNamespaceWithHistory(
		run, generation, resolved.Definition.Graph, topology, outputs, history, "", 0,
	)
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
		return nil, nil
	default:
		return nil, fmt.Errorf(
			"%w: definition-of-done gate returned unsupported route action %q",
			ErrValidation,
			verdict.Route.Action,
		)
	}
}
