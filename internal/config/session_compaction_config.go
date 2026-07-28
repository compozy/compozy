package config

import "time"

// DefaultSessionCompactionConfig returns pressure and retry guards for persisted-context compaction.
func DefaultSessionCompactionConfig() SessionCompactionConfig {
	return SessionCompactionConfig{
		Enabled:            true,
		PressureThreshold:  0.85,
		MaxAttemptsPerTurn: 1,
		FailureCooldown:    10 * time.Minute,
	}
}
