package store

import (
	"fmt"
	"strings"
)

// FailureKind classifies ACP/session lifecycle failures at the source and keeps
// them transport-stable across storage, API, SSE, and CLI surfaces.
type FailureKind string

// FailureReasonCode identifies a stable machine-readable cause within a failure kind.
type FailureReasonCode string

const (
	FailureStartup      FailureKind = "startup_failure"
	FailureHandshake    FailureKind = "handshake_failure"
	FailureLoad         FailureKind = "load_session_failure"
	FailureProtocol     FailureKind = "protocol_failure"
	FailurePrompt       FailureKind = "prompt_failure"
	FailureCanceled     FailureKind = "cancellation"
	FailurePermission   FailureKind = "permission_failure"
	FailureProviderAuth FailureKind = "provider_auth_failure"
	FailureProcess      FailureKind = "process_exit"
	FailureTransport    FailureKind = "transport_failure"
	FailureTimeout      FailureKind = "timeout"
	FailureUnknown      FailureKind = "unknown_failure"
)

const (
	FailureReasonPromptStreamIncomplete FailureReasonCode = "prompt_stream_incomplete"
)

// ValidFailureKind reports whether kind is a supported failure enum member.
func ValidFailureKind(kind FailureKind) bool {
	switch kind {
	case FailureStartup,
		FailureHandshake,
		FailureLoad,
		FailureProtocol,
		FailurePrompt,
		FailureCanceled,
		FailurePermission,
		FailureProviderAuth,
		FailureProcess,
		FailureTransport,
		FailureTimeout,
		FailureUnknown:
		return true
	default:
		return false
	}
}

// ValidFailureReasonCode reports whether code is a supported failure reason.
func ValidFailureReasonCode(code FailureReasonCode) bool {
	return code == FailureReasonPromptStreamIncomplete
}

// SessionFailure is the durable, redacted diagnostic summary attached to a
// session terminal state and projected through public read paths.
type SessionFailure struct {
	Kind            FailureKind       `json:"kind"`
	ReasonCode      FailureReasonCode `json:"reason_code,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	CrashBundlePath string            `json:"crash_bundle_path,omitempty"`
}

// Normalize returns a trimmed copy of f.
func (f SessionFailure) Normalize() SessionFailure {
	return SessionFailure{
		Kind:            FailureKind(strings.TrimSpace(string(f.Kind))),
		ReasonCode:      FailureReasonCode(strings.TrimSpace(string(f.ReasonCode))),
		Summary:         strings.TrimSpace(f.Summary),
		CrashBundlePath: strings.TrimSpace(f.CrashBundlePath),
	}
}

// IsZero reports whether the failure carries no diagnostic fields.
func (f SessionFailure) IsZero() bool {
	normalized := f.Normalize()
	return normalized.Kind == "" && normalized.ReasonCode == "" && normalized.Summary == "" &&
		normalized.CrashBundlePath == ""
}

// Validate checks that non-empty failure records use known kinds.
func (f SessionFailure) Validate() error {
	normalized := f.Normalize()
	if normalized.Kind == "" {
		if normalized.ReasonCode != "" || normalized.Summary != "" || normalized.CrashBundlePath != "" {
			return fmt.Errorf("store: session failure kind is required")
		}
		return nil
	}
	if !ValidFailureKind(normalized.Kind) {
		return fmt.Errorf("store: invalid session failure kind %q", normalized.Kind)
	}
	if normalized.ReasonCode != "" && !ValidFailureReasonCode(normalized.ReasonCode) {
		return fmt.Errorf("store: invalid session failure reason code %q", normalized.ReasonCode)
	}
	if normalized.ReasonCode == FailureReasonPromptStreamIncomplete && normalized.Kind != FailureTransport {
		return fmt.Errorf(
			"store: session failure reason code %q requires kind %q",
			normalized.ReasonCode,
			FailureTransport,
		)
	}
	return nil
}

// CloneSessionFailure returns a deep copy of a session failure pointer.
func CloneSessionFailure(failure *SessionFailure) *SessionFailure {
	if failure == nil {
		return nil
	}
	clone := failure.Normalize()
	return &clone
}
