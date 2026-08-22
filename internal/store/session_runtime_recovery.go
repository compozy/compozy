package store

import (
	"fmt"
	"strings"
	"time"
)

// SessionRuntimeRecovery is the durable projection of an automatic runtime recovery attempt.
type SessionRuntimeRecovery struct {
	Attempt       int        `json:"attempt"`
	MaxAttempts   int        `json:"max_attempts"`
	Generation    int64      `json:"generation"`
	StartedAt     time.Time  `json:"started_at"`
	LastAttemptAt time.Time  `json:"last_attempt_at"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
}

// CloneSessionRuntimeRecovery returns an ownership-safe recovery projection.
func CloneSessionRuntimeRecovery(recovery *SessionRuntimeRecovery) *SessionRuntimeRecovery {
	if recovery == nil {
		return nil
	}
	cloned := *recovery
	if recovery.NextAttemptAt != nil {
		next := recovery.NextAttemptAt.UTC()
		cloned.NextAttemptAt = &next
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
