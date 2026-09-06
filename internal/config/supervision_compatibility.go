package config

import (
	"fmt"
	"log/slog"
	"time"
)

// QuietPolicy is the normalized silence policy consumed by the session manager.
// Zero durations disable their respective action. StopGrace starts at the warning;
// StopAfterQuiet is an absolute silence threshold.
type QuietPolicy struct {
	WarningAfter   time.Duration
	StopGrace      time.Duration
	StopAfterQuiet time.Duration
}

// QuietPolicy resolves the current keys and the N-1 loader's independent timers.
func (c SessionSupervisionConfig) QuietPolicy() QuietPolicy {
	if c.compatibilityQuietPolicy != nil {
		return *c.compatibilityQuietPolicy
	}
	if c.QuietAfter == 0 {
		return QuietPolicy{}
	}
	return QuietPolicy{WarningAfter: c.QuietAfter, StopGrace: c.StopGrace}
}

// Translate the released absolute inactivity thresholds at the loader boundary.
// Any current timing key opts into current zero semantics; its value wins over
// its translated alias. Remove this one-generation decoder in v0.5.0.
func applySupervisionCompatibility(overlay sessionSupervisionOverlay, dst *SessionSupervisionConfig) {
	if overlay.InactivityWarningAfter != nil || overlay.InactivityTimeout != nil {
		quiet, cutoff := 15*time.Minute, 30*time.Minute
		if previous := dst.compatibilityQuietPolicy; previous != nil {
			quiet, cutoff = previous.WarningAfter, previous.StopAfterQuiet
		}
		if overlay.InactivityWarningAfter != nil {
			quiet = *overlay.InactivityWarningAfter
		}
		if overlay.InactivityTimeout != nil {
			cutoff = *overlay.InactivityTimeout
		}
		dst.QuietAfter = quiet
		dst.StopGrace = max(cutoff-quiet, 0)
		dst.compatibilityQuietPolicy = &QuietPolicy{WarningAfter: quiet, StopAfterQuiet: cutoff}
		slog.Warn(
			"session.supervision.inactivity_warning_after and inactivity_timeout are deprecated; "+
				"use quiet_after and stop_grace before v0.5.0",
			"quiet_after",
			dst.QuietAfter,
			"stop_grace",
			dst.StopGrace,
		)
	}
	if overlay.QuietAfter != nil || overlay.StopGrace != nil {
		dst.compatibilityQuietPolicy = nil
	}
}

func (c SessionSupervisionConfig) validateQuietCompatibility() error {
	policy := c.compatibilityQuietPolicy
	if policy == nil {
		return nil
	}
	if policy.WarningAfter < 0 || policy.StopAfterQuiet < 0 {
		return fmt.Errorf("session.supervision legacy inactivity thresholds must be zero or positive")
	}
	if policy.WarningAfter > 0 && policy.StopAfterQuiet > 0 && policy.WarningAfter > policy.StopAfterQuiet {
		return fmt.Errorf("session.supervision.inactivity_warning_after must not exceed inactivity_timeout")
	}
	return nil
}

func (c SessionSupervisionConfig) quietCompatibilityValues() (time.Duration, time.Duration) {
	policy := c.QuietPolicy()
	if policy.StopAfterQuiet > 0 {
		return policy.WarningAfter, policy.StopAfterQuiet
	}
	if policy.WarningAfter > 0 && policy.StopGrace > 0 {
		return policy.WarningAfter, policy.WarningAfter + policy.StopGrace
	}
	return policy.WarningAfter, 0
}

// Keep the released read shape in the same config boundary as its decoder.
func addQuietCompatibilityView(cfg *Config, values map[string]any) {
	sessions, ok := values["session"].(map[string]any)
	if !ok {
		return
	}
	supervision, ok := sessions["supervision"].(map[string]any)
	if !ok {
		return
	}
	warning, cutoff := cfg.Session.Supervision.quietCompatibilityValues()
	supervision["inactivity_warning_after"] = warning.String()
	supervision["inactivity_timeout"] = cutoff.String()
}
