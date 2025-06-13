package autoload

import (
	"fmt"
)

// DefaultExcludes contains patterns for common temporary/backup files that should be ignored
var DefaultExcludes = []string{
	"**/.#*",   // Emacs lock files
	"**/*~",    // Backup files
	"**/*.bak", // Backup files
	"**/*.swp", // Vim swap files
	"**/*.tmp", // Temporary files
	"**/._*",   // macOS resource forks
}

// Config represents the autoload configuration in compozy.yaml
type Config struct {
	Enabled      bool     `json:"enabled"                 yaml:"enabled"                 mapstructure:"enabled"`
	Strict       bool     `json:"strict"                  yaml:"strict"                  mapstructure:"strict"`
	Include      []string `json:"include"                 yaml:"include"                 mapstructure:"include"                 validate:"required_if=Enabled true,dive,required"`
	Exclude      []string `json:"exclude,omitempty"       yaml:"exclude,omitempty"       mapstructure:"exclude,omitempty"`
	WatchEnabled bool     `json:"watch_enabled,omitempty" yaml:"watch_enabled,omitempty" mapstructure:"watch_enabled,omitempty"`
}

// NewConfig creates a new AutoLoadConfig with defaults
func NewConfig() *Config {
	return &Config{
		Enabled: false,
		Strict:  true,
		Include: []string{},
		Exclude: []string{},
	}
}

// Validate validates the autoload configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	// Validate that includes are provided when enabled
	if len(c.Include) == 0 {
		return fmt.Errorf("autoload.include patterns are required when autoload is enabled")
	}
	// Validate include patterns
	for _, pattern := range c.Include {
		if pattern == "" {
			return fmt.Errorf("empty include pattern is not allowed")
		}
	}
	// Validate exclude patterns
	for _, pattern := range c.Exclude {
		if pattern == "" {
			return fmt.Errorf("empty exclude pattern is not allowed")
		}
	}
	return nil
}

// SetDefaults sets default values for the configuration
func (c *Config) SetDefaults() {
	// Strict mode defaults to true if not explicitly set
	if !c.Enabled {
		c.Strict = true
	}
}

// GetAllExcludes returns the combined list of default and user-defined excludes
func (c *Config) GetAllExcludes() []string {
	return append(DefaultExcludes, c.Exclude...)
}

// MergeDefaults merges default values into the configuration
func (c *Config) MergeDefaults() {
	c.SetDefaults()
}
