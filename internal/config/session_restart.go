package config

import (
	"fmt"
	"time"
)

type SessionRestartConfig struct {
	AutoContinue bool          `toml:"auto_continue"`
	MaxAttempts  int           `toml:"max_attempts"`
	Window       time.Duration `toml:"window"`
}

func DefaultSessionRestartConfig() SessionRestartConfig {
	return SessionRestartConfig{AutoContinue: true, MaxAttempts: 3, Window: 5 * time.Minute}
}

func (c SessionRestartConfig) Validate() error {
	if c.MaxAttempts < 1 || c.MaxAttempts > 10 {
		return fmt.Errorf("session.restart.max_attempts must be between 1 and 10: %d", c.MaxAttempts)
	}
	if c.Window < time.Second || c.Window > time.Hour {
		return fmt.Errorf("session.restart.window must be between 1s and 1h: %s", c.Window)
	}
	return nil
}

type sessionRestartOverlay struct {
	AutoContinue *bool          `toml:"auto_continue"`
	MaxAttempts  *int           `toml:"max_attempts"`
	Window       *time.Duration `toml:"window"`
}

func (o sessionRestartOverlay) Apply(dst *SessionRestartConfig) {
	if o.AutoContinue != nil {
		dst.AutoContinue = *o.AutoContinue
	}
	if o.MaxAttempts != nil {
		dst.MaxAttempts = *o.MaxAttempts
	}
	if o.Window != nil {
		dst.Window = *o.Window
	}
}
