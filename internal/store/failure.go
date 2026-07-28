package store

import (
	"fmt"
	"strings"
)

// FailureKind classifies ACP/session lifecycle failures at the source and keeps
// them transport-stable across storage, API, SSE, and CLI surfaces.
type FailureKind string

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

// SessionFailure is the durable, redacted diagnostic summary attached to a
// session terminal state and projected through public read paths.
type SessionFailure struct {
	Kind            FailureKind `json:"kind"`
	Summary         string      `json:"summary,omitempty"`
	CrashBundlePath string      `json:"crash_bundle_path,omitempty"`
}

// Normalize returns a trimmed copy of f.
func (f SessionFailure) Normalize() SessionFailure {
	return SessionFailure{
		Kind:            FailureKind(strings.TrimSpace(string(f.Kind))),
		Summary:         strings.TrimSpace(f.Summary),
		CrashBundlePath: strings.TrimSpace(f.CrashBundlePath),
	}
}

// IsZero reports whether the failure carries no diagnostic fields.
func (f SessionFailure) IsZero() bool {
	normalized := f.Normalize()
	return normalized.Kind == "" && normalized.Summary == "" && normalized.CrashBundlePath == ""
}

// Validate checks that non-empty failure records use known kinds.
func (f SessionFailure) Validate() error {
	normalized := f.Normalize()
	if normalized.Kind == "" {
		if normalized.Summary != "" || normalized.CrashBundlePath != "" {
			return fmt.Errorf("store: session failure kind is required")
		}
		return nil
	}
	if !ValidFailureKind(normalized.Kind) {
		return fmt.Errorf("store: invalid session failure kind %q", normalized.Kind)
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
