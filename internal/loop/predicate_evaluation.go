package loop

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/task"
)

const predicateCostWarningCode = "predicate_cost_warning"

type predicateEvaluation struct {
	Value       bool
	Disposition *PredicateFailureDisposition
	Diagnostics []PredicateDiagnostic
}

type predicateFailureError struct {
	failure ClassifiedFailure
}

func (e *predicateFailureError) Error() string {
	return e.failure.Cause
}

func evaluatePredicate(
	name string,
	condition *refs.Condition,
	variables map[string]any,
	kind PredicateKind,
	override dsl.EvalErrorPolicy,
) (predicateEvaluation, error) {
	if condition == nil {
		return predicateEvaluation{}, fmt.Errorf("%w: compiled predicate %q is missing", ErrValidation, name)
	}
	evaluated, evalErr := condition.Evaluate(variables)
	result := predicateEvaluation{Value: evaluated.Value}
	if evalErr == nil && evaluated.CostWarning {
		result.Diagnostics = append(result.Diagnostics, PredicateDiagnostic{
			Code: predicateCostWarningCode, Predicate: name, Cost: evaluated.Cost,
			CostLimit: condition.CostLimit, Warning: true,
		})
	}
	if evalErr == nil {
		return result, nil
	}
	policy, err := predicatePolicyOverride(override)
	if err != nil {
		return predicateEvaluation{}, err
	}
	disposition, err := ApplyPredicateFailurePolicy(name, kind, policy, evalErr)
	if err != nil {
		return predicateEvaluation{}, err
	}
	disposition.Diagnostic.Cost = evaluated.Cost
	disposition.Diagnostic.CostLimit = condition.CostLimit
	result.Disposition = &disposition
	result.Diagnostics = append(result.Diagnostics, disposition.Diagnostic)
	return result, nil
}

func predicatePolicyOverride(authored dsl.EvalErrorPolicy) (*PredicateErrorPolicy, error) {
	var policy PredicateErrorPolicy
	switch authored {
	case "":
		return nil, nil
	case dsl.EvalErrorFail:
		policy = PredicateErrorRoute
	case dsl.EvalErrorExit:
		policy = PredicateErrorExit
	default:
		return nil, fmt.Errorf("%w: on_eval_error policy is invalid: %q", ErrValidation, authored)
	}
	return &policy, nil
}

func applyPredicateFailureDisposition(
	output GenerationOutput,
	node dsl.Node,
	failure *ClassifiedFailure,
	graph dsl.Graph,
	topology controlTopology,
	outputs *[]GenerationOutput,
) (GenerationOutput, error) {
	if failure == nil {
		return GenerationOutput{}, fmt.Errorf("%w: predicate failure is required", ErrValidation)
	}
	key := generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}
	index, ok := generationOutputIndexMap(*outputs)[key]
	if !ok {
		return GenerationOutput{}, fmt.Errorf(
			"%w: predicate output %s/%d is missing",
			ErrValidation,
			output.NodeID,
			output.ItemIndex,
		)
	}
	output.Status = generationOutputFailed
	(*outputs)[index] = output
	applyNodeFailureDisposition(graph, topology, node, output, *outputs, index, *failure)
	return (*outputs)[index], nil
}

func predicateExitTerminal(diagnostic PredicateDiagnostic) *task.CoordinatorTerminal {
	details, err := json.Marshal(diagnostic)
	if err != nil {
		details = json.RawMessage(predicateEvaluationJSON)
	}
	return &task.CoordinatorTerminal{
		Status: string(StatusDone), Cause: string(TransitionCauseContract),
		ReasonCode: predicateEvaluationFailed, Details: details,
	}
}

func predicateFailureTerminal(diagnostic PredicateDiagnostic) *task.CoordinatorTerminal {
	details, err := json.Marshal(diagnostic)
	if err != nil {
		details = json.RawMessage(predicateEvaluationJSON)
	}
	return &task.CoordinatorTerminal{
		Status: string(StatusFailed), Cause: string(TransitionCauseContract),
		ReasonCode: predicateEvaluationFailed, Details: details,
	}
}
