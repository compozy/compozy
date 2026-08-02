package loop

import (
	"fmt"
	"strings"
)

// PredicateKind distinguishes loop-exit predicates from routing predicates.
type PredicateKind string

const (
	// PredicateContinuation fails open so a broken continuation cannot hang a run.
	PredicateContinuation PredicateKind = "continuation"
	// PredicateRouting fails closed as a routable authoring failure.
	PredicateRouting PredicateKind = "routing"
)

// PredicateErrorPolicy is the explicit response to an evaluation failure.
type PredicateErrorPolicy string

const (
	// PredicateErrorExit exits the current loop by policy.
	PredicateErrorExit PredicateErrorPolicy = "exit"
	// PredicateErrorRoute produces a routable failure.
	PredicateErrorRoute PredicateErrorPolicy = "route"
)

// PredicateFailureDisposition is the pure result of applying predicate policy.
type PredicateFailureDisposition struct {
	Policy     PredicateErrorPolicy
	ExitReason string
	Failure    *ClassifiedFailure
	Diagnostic PredicateDiagnostic
}

// PredicateDiagnostic is the durable diagnostic payload later consumers append.
type PredicateDiagnostic struct {
	Predicate string
	Cause     string
	Outcome   PredicateErrorPolicy
}

// ApplyPredicateFailurePolicy maps a broken predicate to its deterministic policy result.
func ApplyPredicateFailurePolicy(
	predicate string,
	kind PredicateKind,
	override *PredicateErrorPolicy,
	cause error,
) (PredicateFailureDisposition, error) {
	name := strings.TrimSpace(predicate)
	if name == "" {
		return PredicateFailureDisposition{}, fmt.Errorf("%w: predicate name is required", ErrValidation)
	}
	if cause == nil {
		return PredicateFailureDisposition{}, fmt.Errorf("%w: predicate failure cause is required", ErrValidation)
	}
	policy, err := resolvePredicateErrorPolicy(kind, override)
	if err != nil {
		return PredicateFailureDisposition{}, err
	}
	diagnostic := PredicateDiagnostic{Predicate: name, Cause: cause.Error(), Outcome: policy}
	result := PredicateFailureDisposition{Policy: policy, Diagnostic: diagnostic}
	if policy == PredicateErrorExit {
		result.ExitReason = fmt.Sprintf("predicate %s failed: %s", name, cause)
		return result, nil
	}
	classified := ClassifyNodeFailure(FailureEvidence{
		Authoring: true,
		Code:      "predicate_evaluation_failed",
		Cause:     fmt.Sprintf("predicate %s failed: %s", name, cause),
	})
	result.Failure = &classified
	return result, nil
}

func resolvePredicateErrorPolicy(
	kind PredicateKind,
	override *PredicateErrorPolicy,
) (PredicateErrorPolicy, error) {
	if override != nil {
		switch *override {
		case PredicateErrorExit, PredicateErrorRoute:
			return *override, nil
		default:
			return "", fmt.Errorf("%w: predicate on_eval_error policy is invalid: %q", ErrValidation, *override)
		}
	}
	switch kind {
	case PredicateContinuation:
		return PredicateErrorExit, nil
	case PredicateRouting:
		return PredicateErrorRoute, nil
	default:
		return "", fmt.Errorf("%w: predicate kind is invalid: %q", ErrValidation, kind)
	}
}
