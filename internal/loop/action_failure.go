package loop

import (
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

const (
	actionFailureKind      = "action_failure"
	actionFailureCodeLimit = 256
	actionFailureTextLimit = 2 * 1024
)

// ActionFailure is the durable operator-safe failure payload stored for a Loop action.
type ActionFailure struct {
	Kind     string `json:"kind"`
	Code     string `json:"code"`
	Cause    string `json:"cause"`
	Recovery string `json:"recovery"`
	Target   string `json:"target,omitempty"`
}

// SafeActionFailureProvider lets a domain error publish durable operator-safe
// cause and recovery detail without exposing its internal error text.
type SafeActionFailureProvider interface {
	error
	SafeActionFailure() ActionFailure
}

type safeActionFailureError struct {
	err     error
	failure ActionFailure
}

func (e *safeActionFailureError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *safeActionFailureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *safeActionFailureError) SafeActionFailure() ActionFailure {
	if e == nil {
		return ActionFailure{}
	}
	return e.failure
}

func newSafeActionFailureError(err error, failure ActionFailure) error {
	return NewSafeActionFailureError(err, failure)
}

// NewSafeActionFailureError wraps an error with a SafeActionFailure payload.
func NewSafeActionFailureError(err error, failure ActionFailure) error {
	return &safeActionFailureError{err: err, failure: failure}
}

// NewActionFailure constructs a normalized action failure payload.
func NewActionFailure(code string, cause string, recovery string) ActionFailure {
	return ActionFailure{
		Kind:     actionFailureKind,
		Code:     diagnostics.RedactAndBound(code, actionFailureCodeLimit),
		Cause:    diagnostics.RedactAndBound(cause, actionFailureTextLimit),
		Recovery: diagnostics.RedactAndBound(recovery, actionFailureTextLimit),
	}
}

// ActionFailureOutputRefFromMetadata extracts a valid failure payload from task-run metadata.
func ActionFailureOutputRefFromMetadata(raw json.RawMessage) (string, bool) {
	failure, ok := ActionFailureFromMetadata(raw)
	if !ok {
		return "", false
	}
	return ActionFailureOutputRef(failure)
}

// ActionFailureFromMetadata extracts a validated operator-safe failure from task-run metadata.
func ActionFailureFromMetadata(raw json.RawMessage) (ActionFailure, bool) {
	if len(raw) == 0 {
		return ActionFailure{}, false
	}
	var envelope struct {
		Failure *ActionFailure `json:"failure"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ActionFailure{}, false
	}
	if envelope.Failure == nil {
		return ActionFailure{}, false
	}
	failure := *envelope.Failure
	failure.Kind = strings.TrimSpace(failure.Kind)
	failure.Code = strings.TrimSpace(failure.Code)
	failure.Cause = strings.TrimSpace(failure.Cause)
	failure.Recovery = strings.TrimSpace(failure.Recovery)
	failure.Target = diagnostics.RedactAndBound(strings.TrimSpace(failure.Target), actionFailureCodeLimit)
	if failure.Kind != actionFailureKind || failure.Code == "" || failure.Cause == "" {
		return ActionFailure{}, false
	}
	return failure, true
}

// ActionFailureOutputRef encodes a validated failure for a generation output cell.
func ActionFailureOutputRef(failure ActionFailure) (string, bool) {
	failure.Kind = strings.TrimSpace(failure.Kind)
	failure.Code = strings.TrimSpace(failure.Code)
	failure.Cause = strings.TrimSpace(failure.Cause)
	failure.Recovery = strings.TrimSpace(failure.Recovery)
	failure.Target = diagnostics.RedactAndBound(strings.TrimSpace(failure.Target), actionFailureCodeLimit)
	if failure.Kind != actionFailureKind || failure.Code == "" || failure.Cause == "" {
		return "", false
	}
	payload, err := json.Marshal(failure)
	if err != nil {
		return "", false
	}
	return string(payload), true
}
