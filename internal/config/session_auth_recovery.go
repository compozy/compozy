package config

import (
	"fmt"
	"time"
)

// SessionAuthRecoveryConfig bounds how long a turn waits for a provider sign-in to return.
type SessionAuthRecoveryConfig struct {
	Enabled      bool          `toml:"enabled"`
	Window       time.Duration `toml:"window"`
	InitialDelay time.Duration `toml:"initial_delay"`
	MaxDelay     time.Duration `toml:"max_delay"`
}

// DefaultSessionAuthRecoveryConfig returns the default provider sign-in wait.
func DefaultSessionAuthRecoveryConfig() SessionAuthRecoveryConfig {
	return SessionAuthRecoveryConfig{
		Enabled:      true,
		Window:       10 * time.Minute,
		InitialDelay: 15 * time.Second,
		MaxDelay:     time.Minute,
	}
}

// Validate ensures the retry schedule is bounded and yields at least one attempt.
func (c SessionAuthRecoveryConfig) Validate() error {
	if c.InitialDelay < time.Second || c.InitialDelay > 10*time.Minute {
		return fmt.Errorf("session.auth_recovery.initial_delay must be between 1s and 10m: %s", c.InitialDelay)
	}
	if c.MaxDelay < c.InitialDelay || c.MaxDelay > 10*time.Minute {
		return fmt.Errorf(
			"session.auth_recovery.max_delay must be between initial_delay (%s) and 10m: %s",
			c.InitialDelay, c.MaxDelay,
		)
	}
	if c.Window < c.InitialDelay || c.Window > time.Hour {
		return fmt.Errorf(
			"session.auth_recovery.window must be between initial_delay (%s) and 1h: %s",
			c.InitialDelay, c.Window,
		)
	}
	return nil
}

// Delays returns the wait before each retry: doubling from initial_delay, capped at
// max_delay, for as many attempts as fit inside window. Disabled yields no attempts.
func (c SessionAuthRecoveryConfig) Delays() []time.Duration {
	if !c.Enabled || c.InitialDelay <= 0 || c.MaxDelay <= 0 {
		return nil
	}
	var delays []time.Duration
	var elapsed time.Duration
	delay := c.InitialDelay
	for elapsed+delay <= c.Window {
		delays = append(delays, delay)
		elapsed += delay
		delay = min(delay*2, c.MaxDelay)
	}
	return delays
}

type sessionAuthRecoveryOverlay struct {
	Enabled      *bool          `toml:"enabled"`
	Window       *time.Duration `toml:"window"`
	InitialDelay *time.Duration `toml:"initial_delay"`
	MaxDelay     *time.Duration `toml:"max_delay"`
}

func (o sessionAuthRecoveryOverlay) Apply(dst *SessionAuthRecoveryConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.Window != nil {
		dst.Window = *o.Window
	}
	if o.InitialDelay != nil {
		dst.InitialDelay = *o.InitialDelay
	}
	if o.MaxDelay != nil {
		dst.MaxDelay = *o.MaxDelay
	}
}
