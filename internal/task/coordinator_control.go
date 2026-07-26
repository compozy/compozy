package task

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CoordinatorControlResult is a neutral coordinator-owned completion envelope.
// The owning coordinator validates Payload without introducing a task-domain dependency.
type CoordinatorControlResult struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// RunResult captures the durable JSON result returned by a completed run.
type RunResult struct {
	Value              json.RawMessage           `json:"value,omitempty"`
	TokensUsed         int64                     `json:"tokens_used,omitempty"`
	CoordinatorControl *CoordinatorControlResult `json:"coordinator_control,omitempty"`
}

// StoredValue returns the task_runs.result_json representation while preserving
// the historical raw Value shape for ordinary completions.
func (r RunResult) StoredValue() (json.RawMessage, error) {
	if r.CoordinatorControl == nil {
		return cloneRawJSON(r.Value), nil
	}
	payload, err := json.Marshal(r.CoordinatorControl)
	if err != nil {
		return nil, fmt.Errorf("task: marshal coordinator control: %w", err)
	}
	return payload, nil
}

// Validate reports whether the run result respects the shared result-size guardrail.
func (r RunResult) Validate(path string) error {
	if err := ValidateResultSize(r.Value, nestedPath(path, "value")); err != nil {
		return err
	}
	if hasRawClaimTokenField(r.Value) {
		return fmt.Errorf(
			"%w: %s must not contain raw lease credentials",
			ErrValidation,
			nestedPath(path, "value"),
		)
	}
	if r.TokensUsed < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "tokens_used"),
			r.TokensUsed,
		)
	}
	if r.CoordinatorControl == nil {
		return nil
	}
	if len(r.Value) > 0 {
		return fmt.Errorf(
			"%w: %s and %s are mutually exclusive",
			ErrValidation,
			nestedPath(path, "value"),
			nestedPath(path, "coordinator_control"),
		)
	}
	return r.CoordinatorControl.Validate(nestedPath(path, "coordinator_control"))
}

// Validate reports whether a neutral coordinator-control envelope is bounded and well formed.
func (r CoordinatorControlResult) Validate(path string) error {
	if strings.TrimSpace(r.Kind) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "kind"))
	}
	if len(r.Payload) == 0 {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "payload"))
	}
	if err := ValidateResultSize(r.Payload, nestedPath(path, "payload")); err != nil {
		return err
	}
	if hasRawClaimTokenField(r.Payload) {
		return fmt.Errorf(
			"%w: %s must not contain raw lease credentials",
			ErrValidation,
			nestedPath(path, "payload"),
		)
	}
	return nil
}
