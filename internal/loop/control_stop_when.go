package loop

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
)

const contractStopWhenConditionKey = "contract.stop_when"

func evaluateContractStopWhen(
	_ context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	outputs []GenerationOutput,
) (bool, bool, error) {
	if resolved == nil {
		return false, false, nil
	}
	condition := resolved.Conditions[contractStopWhenConditionKey]
	if condition == nil {
		return false, false, nil
	}
	namespace, err := runtimeNamespace(
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		outputs,
		dsl.NodeID(""),
		0,
	)
	if err != nil {
		return false, true, err
	}
	value, _, err := condition.Program.Eval(namespace)
	if err != nil {
		return false, true, fmt.Errorf("loop: evaluate contract.stop_when: %w", err)
	}
	result, ok := value.Value().(bool)
	if !ok {
		return false, true, fmt.Errorf(
			"%w: contract.stop_when returned %T",
			ErrValidation,
			value.Value(),
		)
	}
	return result, true, nil
}
