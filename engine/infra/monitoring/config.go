package monitoring

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/compozy/compozy/pkg/logger"
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
	// Validate path doesn't conflict with API routes
	if strings.HasPrefix(c.Path, "/api/") {
		return fmt.Errorf("monitoring path cannot be under /api/")
	}
	// Path should not contain query parameters
	if strings.ContainsRune(c.Path, '?') {
		return fmt.Errorf("monitoring path cannot contain query parameters")
	}
	return nil
}

// LoadWithEnv creates a monitoring config with environment variable precedence
// Environment variables take precedence over the provided config values
func LoadWithEnv(yamlConfig *Config) *Config {
	// Start with defaults if no config provided
	config := DefaultConfig()
	// Apply YAML config if provided
	if yamlConfig != nil {
		config.Enabled = yamlConfig.Enabled
		if yamlConfig.Path != "" {
			config.Path = yamlConfig.Path
		}
	}
	// Environment variable takes precedence for Enabled flag
	if envEnabled := os.Getenv("MONITORING_ENABLED"); envEnabled != "" {
		enabled, err := strconv.ParseBool(envEnabled)
		if err != nil {
			logger.Error("Invalid MONITORING_ENABLED value", "value", envEnabled, "error", err)
		} else {
			config.Enabled = enabled
		}
	}
	// Environment variable takes precedence for Path
	if envPath := os.Getenv("MONITORING_PATH"); envPath != "" {
		config.Path = envPath
	}
	return config
}
