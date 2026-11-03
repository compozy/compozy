package store

import (
	"fmt"
	"strings"
	"time"
)

// SessionRuntimeRecoveryReason names the failure an automatic recovery is waiting out.
type SessionRuntimeRecoveryReason string

const (
	// SessionRuntimeRecoveryRuntimeExit replaces a provider runtime that exited or disconnected mid-turn.
	SessionRuntimeRecoveryRuntimeExit SessionRuntimeRecoveryReason = "runtime_exit"
	// SessionRuntimeRecoveryProviderAuth waits for a provider sign-in that went missing mid-turn.
	SessionRuntimeRecoveryProviderAuth SessionRuntimeRecoveryReason = "provider_auth"
)

// SessionRuntimeRecoveryReasonValues returns the public recovery reason vocabulary.
func SessionRuntimeRecoveryReasonValues() []string {
	return []string{string(SessionRuntimeRecoveryRuntimeExit), string(SessionRuntimeRecoveryProviderAuth)}
}

// SessionRuntimeRecovery is the durable projection of an automatic runtime recovery attempt.
type SessionRuntimeRecovery struct {
	Reason        SessionRuntimeRecoveryReason `json:"reason,omitempty"`
	Attempt       int                          `json:"attempt"`
	MaxAttempts   int                          `json:"max_attempts"`
	Generation    int64                        `json:"generation"`
	StartedAt     time.Time                    `json:"started_at"`
	LastAttemptAt time.Time                    `json:"last_attempt_at"`
	NextAttemptAt *time.Time                   `json:"next_attempt_at,omitempty"`
	LastError     string                       `json:"last_error,omitempty"`
}

// CloneSessionRuntimeRecovery returns an ownership-safe recovery projection.
func CloneSessionRuntimeRecovery(recovery *SessionRuntimeRecovery) *SessionRuntimeRecovery {
	if recovery == nil {
		return nil
	}
	cloned := *recovery
	if recovery.NextAttemptAt != nil {
		cloned.NextAttemptAt = new(recovery.NextAttemptAt.UTC())
	}
	return &cloned
}

func validateSessionRuntimeRecovery(
	status SessionRuntimeStatus,
	generation int64,
	recovery *SessionRuntimeRecovery,
) error {
	if generation < 0 {
		return fmt.Errorf("store: session runtime generation must be non-negative")
	}
	if recovery == nil {
		return nil
	}
	if status != SessionRuntimeRecovering {
		return fmt.Errorf("store: runtime recovery requires recovering status")
	}
	if recovery.Attempt < 1 || recovery.MaxAttempts < recovery.Attempt {
		return fmt.Errorf("store: invalid runtime recovery attempt %d of %d", recovery.Attempt, recovery.MaxAttempts)
	}
	if recovery.Generation <= generation {
		return fmt.Errorf("store: runtime recovery generation must follow the active generation")
	}
	if recovery.StartedAt.IsZero() || recovery.LastAttemptAt.IsZero() {
		return fmt.Errorf("store: runtime recovery timestamps are required")
	}
	switch recovery.Reason {
	case "", SessionRuntimeRecoveryRuntimeExit, SessionRuntimeRecoveryProviderAuth:
	default:
		return fmt.Errorf("store: unknown runtime recovery reason %q", recovery.Reason)
	}
	if strings.TrimSpace(recovery.LastError) != recovery.LastError {
		return fmt.Errorf("store: runtime recovery last error must be normalized")
	}
	return nil
}

// ValidateSessionRuntimeRecovery validates one durable recovery projection.
func ValidateSessionRuntimeRecovery(
	status SessionRuntimeStatus,
	generation int64,
	recovery *SessionRuntimeRecovery,
) error {
	return validateSessionRuntimeRecovery(status, generation, recovery)
}
