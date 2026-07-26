package loop

import (
	"encoding/json"
	"strings"
)

const actionFailureKind = "action_failure"

// ActionFailure is the durable operator-safe failure payload stored for a Loop action.
type ActionFailure struct {
	Kind     string `json:"kind"`
	Code     string `json:"code"`
	Cause    string `json:"cause"`
	Recovery string `json:"recovery"`
}

// SafeActionFailureProvider lets a domain error publish durable operator-safe
// cause and recovery detail without exposing its internal error text.
type SafeActionFailureProvider interface {
	error
	SafeActionFailure() ActionFailure
}

// NewActionFailure constructs a normalized action failure payload.
func NewActionFailure(code string, cause string, recovery string) ActionFailure {
	return ActionFailure{
		Kind:     actionFailureKind,
		Code:     strings.TrimSpace(code),
		Cause:    strings.TrimSpace(cause),
		Recovery: strings.TrimSpace(recovery),
	}
}

// ActionFailureOutputRefFromMetadata extracts a valid failure payload from task-run metadata.
func ActionFailureOutputRefFromMetadata(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var envelope struct {
		Failure *ActionFailure `json:"failure"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", false
	}
	if envelope.Failure == nil {
		return "", false
	}
	failure := *envelope.Failure
	failure.Kind = strings.TrimSpace(failure.Kind)
	failure.Code = strings.TrimSpace(failure.Code)
	failure.Cause = strings.TrimSpace(failure.Cause)
	failure.Recovery = strings.TrimSpace(failure.Recovery)
	if failure.Kind != actionFailureKind || failure.Code == "" || failure.Cause == "" || failure.Recovery == "" {
		return "", false
	}
	payload, err := json.Marshal(failure)
	if err != nil {
		return "", false
	}
	return string(payload), true
}
