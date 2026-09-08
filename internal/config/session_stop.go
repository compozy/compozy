package config

import (
	"fmt"
	"time"
)

func DefaultSessionStopConfig() SessionStopConfig {
	return SessionStopConfig{CooperativeGrace: 10 * time.Second}
}

func (c SessionStopConfig) Validate() error {
	if c.CooperativeGrace <= 0 {
		return fmt.Errorf("session.stop.cooperative_grace must be positive: %s", c.CooperativeGrace)
	}
	return nil
}
