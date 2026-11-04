package config

import (
	"fmt"
	"strings"
	"time"
)

type HeartbeatSourcesConfig struct {
	ScriptsEnabled     bool          `toml:"scripts_enabled"`
	Timeout            time.Duration `toml:"timeout"`
	MaxConcurrentPolls int           `toml:"max_concurrent_polls"`
	MaxPerAgent        int           `toml:"max_per_agent"`
	MaxItems           int           `toml:"max_items"`
	MaxOutputBytes     int           `toml:"max_output_bytes"`
	MaxStateBytes      int           `toml:"max_state_bytes"`
	FailureThreshold   int           `toml:"failure_threshold"`
	EnvAllow           []string      `toml:"env_allow"`
}

func DefaultHeartbeatSourcesConfig() HeartbeatSourcesConfig {
	return HeartbeatSourcesConfig{
		ScriptsEnabled:     true,
		Timeout:            30 * time.Second,
		MaxConcurrentPolls: 4,
		MaxPerAgent:        8,
		MaxItems:           20,
		MaxOutputBytes:     262144,
		MaxStateBytes:      16384,
		FailureThreshold:   3,
	}
}
func (c HeartbeatSourcesConfig) Validate() error {
	if c.Timeout < time.Second || c.Timeout > 5*time.Minute {
		return fmt.Errorf("agents.heartbeat.sources.timeout must be 1s to 5m")
	}
	for _, v := range []struct {
		name            string
		value, min, max int
	}{
		{"max_concurrent_polls", c.MaxConcurrentPolls, 1, 32},
		{"max_per_agent", c.MaxPerAgent, 1, 32},
		{"max_items", c.MaxItems, 1, 100},
		{"max_output_bytes", c.MaxOutputBytes, 4096, 4 << 20},
		{"max_state_bytes", c.MaxStateBytes, 1024, 256 << 10},
		{"failure_threshold", c.FailureThreshold, 1, 20},
	} {
		if v.value < v.min || v.value > v.max {
			return fmt.Errorf("agents.heartbeat.sources.%s must be %d to %d", v.name, v.min, v.max)
		}
	}
	for _, name := range c.EnvAllow {
		if name == "" || strings.HasPrefix(name, "COMPOZY_") || strings.ContainsAny(name, "=\x00") {
			return fmt.Errorf("agents.heartbeat.sources.env_allow contains an invalid environment name")
		}
	}
	return nil
}

type heartbeatSourcesOverlay struct {
	ScriptsEnabled     *bool          `toml:"scripts_enabled"`
	Timeout            *time.Duration `toml:"timeout"`
	MaxConcurrentPolls *int           `toml:"max_concurrent_polls"`
	MaxPerAgent        *int           `toml:"max_per_agent"`
	MaxItems           *int           `toml:"max_items"`
	MaxOutputBytes     *int           `toml:"max_output_bytes"`
	MaxStateBytes      *int           `toml:"max_state_bytes"`
	FailureThreshold   *int           `toml:"failure_threshold"`
	EnvAllow           []string       `toml:"env_allow"`
}

func (o heartbeatSourcesOverlay) Apply(dst *HeartbeatSourcesConfig) {
	if o.ScriptsEnabled != nil {
		dst.ScriptsEnabled = *o.ScriptsEnabled
	}
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
	if o.MaxConcurrentPolls != nil {
		dst.MaxConcurrentPolls = *o.MaxConcurrentPolls
	}
	if o.MaxPerAgent != nil {
		dst.MaxPerAgent = *o.MaxPerAgent
	}
	if o.MaxItems != nil {
		dst.MaxItems = *o.MaxItems
	}
	if o.MaxOutputBytes != nil {
		dst.MaxOutputBytes = *o.MaxOutputBytes
	}
	if o.MaxStateBytes != nil {
		dst.MaxStateBytes = *o.MaxStateBytes
	}
	if o.FailureThreshold != nil {
		dst.FailureThreshold = *o.FailureThreshold
	}
	if o.EnvAllow != nil {
		dst.EnvAllow = o.EnvAllow
	}
}

func (o heartbeatSourcesOverlay) present() bool {
	return o.ScriptsEnabled != nil || o.Timeout != nil || o.MaxConcurrentPolls != nil || o.MaxPerAgent != nil ||
		o.MaxItems != nil ||
		o.MaxOutputBytes != nil ||
		o.MaxStateBytes != nil ||
		o.FailureThreshold != nil ||
		o.EnvAllow != nil
}

func (c HeartbeatSourcesConfig) IsZero() bool {
	return c.Timeout == 0 && c.MaxPerAgent == 0 && c.MaxConcurrentPolls == 0 && c.MaxItems == 0 &&
		c.MaxOutputBytes == 0 &&
		c.MaxStateBytes == 0 &&
		c.FailureThreshold == 0 &&
		len(c.EnvAllow) == 0 &&
		!c.ScriptsEnabled
}
