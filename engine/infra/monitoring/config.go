package monitoring

import (
	"fmt"
)

// Config holds configuration for monitoring service
type Config struct {
	Enabled bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled" env:"MONITORING_ENABLED"`
	Path    string `json:"path"    yaml:"path"    mapstructure:"path"    env:"MONITORING_PATH"`
}

// DefaultConfig returns default monitoring configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled: false,
		Path:    "/metrics",
	}
}

// Validate validates the monitoring configuration
func (c *Config) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("monitoring path cannot be empty")
	}
	if c.Path[0] != '/' {
		return fmt.Errorf("monitoring path must start with '/': got %s", c.Path)
	}
	return nil
}
