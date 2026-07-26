package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// CoordinatorFailureKind identifies the typed safe coordinator diagnostic envelope.
	CoordinatorFailureKind = "coordinator_failure"

	maxCoordinatorFailureCodeBytes     = 96
	maxCoordinatorFailureCauseBytes    = 512
	maxCoordinatorFailureRecoveryBytes = 512
)

var defaultCoordinatorRunFailure = RunFailure{
	Error: "The Loop coordinator failed before it could settle the run.",
	Metadata: json.RawMessage(
		`{"kind":"coordinator_failure","code":"coordinator_failed","cause":"The Loop coordinator failed before it could settle the run.","recovery":"Inspect daemon logs for the correlated coordinator run, correct the underlying condition, then start a new run."}`,
	),
}

// CoordinatorFailure is the bounded operator-safe diagnostic for coordinator execution failures.
type CoordinatorFailure struct {
	Kind     string `json:"kind"`
	Code     string `json:"code"`
	Cause    string `json:"cause"`
	Recovery string `json:"recovery"`
}

// NewCoordinatorRunFailure builds a validated safe failure for persistence and projection.
func NewCoordinatorRunFailure(code string, cause string, recovery string) (RunFailure, error) {
	failure := CoordinatorFailure{
		Kind:     CoordinatorFailureKind,
		Code:     strings.TrimSpace(code),
		Cause:    strings.TrimSpace(cause),
		Recovery: strings.TrimSpace(recovery),
	}
	if err := failure.validate(); err != nil {
		return RunFailure{}, err
	}
	metadata, err := json.Marshal(failure)
	if err != nil {
		return RunFailure{}, fmt.Errorf("task: marshal coordinator failure: %w", err)
	}
	runFailure, err := normalizeRunFailure(RunFailure{Error: failure.Cause, Metadata: metadata})
	if err != nil {
		return RunFailure{}, fmt.Errorf("task: normalize coordinator failure: %w", err)
	}
	return runFailure, nil
}

// CoordinatorFailureFromRunFailure parses a valid typed coordinator failure.
func CoordinatorFailureFromRunFailure(failure RunFailure) (CoordinatorFailure, bool) {
	normalized, err := normalizeRunFailure(failure)
	if err != nil || len(normalized.Metadata) == 0 {
		return CoordinatorFailure{}, false
	}
	var details CoordinatorFailure
	if err := json.Unmarshal(normalized.Metadata, &details); err != nil {
		return CoordinatorFailure{}, false
	}
	details.Kind = strings.TrimSpace(details.Kind)
	details.Code = strings.TrimSpace(details.Code)
	details.Cause = strings.TrimSpace(details.Cause)
	details.Recovery = strings.TrimSpace(details.Recovery)
	if err := details.validate(); err != nil || normalized.Error != details.Cause {
		return CoordinatorFailure{}, false
	}
	return details, true
}

// CanonicalCoordinatorRunFailure returns a bounded operator-safe coordinator failure.
// Invalid, untyped, or non-canonical metadata is replaced instead of persisted.
func CanonicalCoordinatorRunFailure(failure RunFailure) RunFailure {
	details, ok := CoordinatorFailureFromRunFailure(failure)
	if !ok {
		return cloneDefaultCoordinatorRunFailure()
	}
	canonical, err := NewCoordinatorRunFailure(details.Code, details.Cause, details.Recovery)
	if err != nil {
		return cloneDefaultCoordinatorRunFailure()
	}
	return canonical
}

func coordinatorRunFailure(err error) RunFailure {
	type safeRunFailureProvider interface {
		error
		SafeRunFailure() RunFailure
	}
	if provider, ok := errors.AsType[safeRunFailureProvider](err); ok {
		return CanonicalCoordinatorRunFailure(provider.SafeRunFailure())
	}
	return cloneDefaultCoordinatorRunFailure()
}

func cloneDefaultCoordinatorRunFailure() RunFailure {
	return RunFailure{
		Error:    defaultCoordinatorRunFailure.Error,
		Metadata: cloneRawJSON(defaultCoordinatorRunFailure.Metadata),
	}
}

func (f CoordinatorFailure) validate() error {
	if strings.TrimSpace(f.Kind) != CoordinatorFailureKind {
		return fmt.Errorf("%w: coordinator failure kind must be %q", ErrValidation, CoordinatorFailureKind)
	}
	if err := validateCoordinatorFailureField("code", f.Code, maxCoordinatorFailureCodeBytes); err != nil {
		return err
	}
	if err := validateCoordinatorFailureField("cause", f.Cause, maxCoordinatorFailureCauseBytes); err != nil {
		return err
	}
	return validateCoordinatorFailureField("recovery", f.Recovery, maxCoordinatorFailureRecoveryBytes)
}

func validateCoordinatorFailureField(name string, value string, maxBytes int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%w: coordinator failure %s is required", ErrValidation, name)
	}
	if len([]byte(trimmed)) > maxBytes {
		return fmt.Errorf(
			"%w: coordinator failure %s exceeds %d bytes",
			ErrValidation,
			name,
			maxBytes,
		)
	}
	return nil
}
