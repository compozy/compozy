package loop

import (
	"context"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

const contractStopWhenConditionKey = "contract.stop_when"

type contractStopWhenEvaluation struct {
	stop        bool
	present     bool
	terminal    *task.CoordinatorTerminal
	diagnostics []PredicateDiagnostic
}

func evaluateContractStopWhen(
	_ context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	outputs []GenerationOutput,
	history GenerationHistory,
) (contractStopWhenEvaluation, error) {
	if resolved == nil {
		return contractStopWhenEvaluation{}, nil
	}
	condition := resolved.Conditions[contractStopWhenConditionKey]
	if condition == nil {
		return contractStopWhenEvaluation{}, nil
	}
	namespace, err := runtimeNamespaceWithHistory(
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		outputs,
		history,
		dsl.NodeID(""),
		0,
	)
	if err != nil {
		return contractStopWhenEvaluation{present: true}, err
	}
	evaluated, err := evaluatePredicate(
		contractStopWhenConditionKey,
		condition,
		namespace,
		PredicateContinuation,
		resolved.Definition.Contract.StopWhen.OnEvalError,
	)
	if err != nil {
		return contractStopWhenEvaluation{present: true}, err
	}
	result := contractStopWhenEvaluation{
		stop: evaluated.Value, present: true, diagnostics: evaluated.Diagnostics,
	}
	if evaluated.Disposition == nil {
		return result, nil
	}
	if evaluated.Disposition.Policy == PredicateErrorExit {
		result.stop = true
		return result, nil
	}
	result.terminal = predicateFailureTerminal(evaluated.Disposition.Diagnostic)
	return result, nil
}
