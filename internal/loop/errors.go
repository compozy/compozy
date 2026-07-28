package loop

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrValidation reports invalid loop-domain input.
	ErrValidation = errors.New("loop: validation failed")
	// ErrRunNotFound reports a missing loop_run aggregate.
	ErrRunNotFound = errors.New("loop: run not found")
	// ErrConfigNotFound reports the absence of a per-loop loop_config row.
	ErrConfigNotFound = errors.New("loop: config not found")
	// ErrOutputRefNotFound reports a missing content-addressed loop output payload.
	ErrOutputRefNotFound = errors.New("loop: output ref not found")
	// ErrInvalidTransition reports an illegal status transition.
	ErrInvalidTransition = errors.New("loop: invalid status transition")
	// ErrTransitionConflict reports that the durable compare-and-swap lost a race.
	ErrTransitionConflict = errors.New("loop: transition conflict")
	// ErrConcurrencyConflict reports a same-loop start rejected by policy.
	ErrConcurrencyConflict = errors.New("loop: concurrency conflict")
	// ErrAncestryRejected reports an invalid run-loop ancestry chain.
	ErrAncestryRejected = errors.New("loop: ancestry rejected")
)

// ReasonCode is the deterministic machine-readable failure code surfaced to agents.
type ReasonCode string

const (
	// ReasonCodeActiveRunExists reports a forbid-concurrency start conflict.
	ReasonCodeActiveRunExists ReasonCode = "active_loop_run_exists"
	// ReasonCodeAncestryCycle reports a run-loop target already in its parent chain.
	ReasonCodeAncestryCycle ReasonCode = "run_loop_ancestry_cycle"
	// ReasonCodeAncestryDepthExceeded reports a parent chain deeper than the hard ceiling.
	ReasonCodeAncestryDepthExceeded ReasonCode = "run_loop_ancestry_depth_exceeded"
	// ReasonCodeInvalidStatusTransition reports a rejected status edge.
	ReasonCodeInvalidStatusTransition ReasonCode = "invalid_status_transition"
	// ReasonCodeTerminalRun reports an operation rejected because the run is already terminal.
	ReasonCodeTerminalRun ReasonCode = "terminal_loop_run"
	// ReasonCodeStartKindNotAllowed reports a start surface missing from the loop definition allowlist.
	ReasonCodeStartKindNotAllowed ReasonCode = "start_kind_not_allowed"
	// ReasonCodeStartInputInvalid reports invalid static or mapped start inputs.
	ReasonCodeStartInputInvalid ReasonCode = "start_input_invalid"
	// ReasonCodeStartInputMappingInvalid reports invalid or unresolvable input_mapping entries.
	ReasonCodeStartInputMappingInvalid ReasonCode = "start_input_mapping_invalid"
)

// ReasonError carries a deterministic reason code while preserving errors.Is.
type ReasonError struct {
	Code ReasonCode
	Err  error
	Meta map[string]string
}

func (e *ReasonError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Code)
	}
	if len(e.Meta) == 0 {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	keys := make([]string, 0, len(e.Meta))
	for key := range e.Meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, e.Meta[key]))
	}
	return fmt.Sprintf("%s: %v (%s)", e.Code, e.Err, strings.Join(parts, ", "))
}

// Unwrap returns the wrapped sentinel error.
func (e *ReasonError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func reasonError(code ReasonCode, err error, meta map[string]string) error {
	return &ReasonError{Code: code, Err: err, Meta: meta}
}
