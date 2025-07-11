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

// Config represents the autoload configuration for automatically discovering and loading resources.
//
// **AutoLoad** enables automatic discovery and registration of resources (agents, tools, memory)
// without explicitly listing them in the workflow configuration. This feature:
//   - **Reduces boilerplate** by eliminating manual resource registration
//   - **Improves maintainability** by automatically detecting new resources
//   - **Ensures consistency** across complex projects with many resources
//   - **Supports hot-reloading** for development workflows (when watch is enabled)
//
// Resources are discovered using glob patterns and loaded based on their file structure
// and naming conventions.
//
// ## Basic AutoLoad Configuration
//
//	autoload:
//	  enabled: true
//	  include:
//	    - "agents/*.yaml"
//	    - "tools/*.yaml"
//
// ## Advanced AutoLoad with Exclusions
//
//	autoload:
//	  enabled: true
//	  strict: false         # Allow missing resources
//	  watch_enabled: true   # Enable file watching for hot reload
//	  include:
//	    - "agents/**/*.yaml"
//	    - "memory/**/*.yaml"
//	    - "tools/**/*.yaml"
//	  exclude:
//	    - "**/*-test.yaml"  # Exclude test files
//	    - "**/*-draft.yaml" # Exclude draft resources
//
// ## Directory Structure Example
//
//	project/
//	├── agents/
//	│   ├── researcher.yaml    # Auto-discovered
//	│   ├── writer.yaml        # Auto-discovered
//	│   └── draft-agent.yaml   # Excluded by pattern
//	├── tools/
//	│   └── calculator.yaml    # Auto-discovered
//	└── memory/
//	    └── conversation.yaml  # Auto-discovered
type Config struct {
	// Enabled determines whether autoload functionality is active.
	//
	// When `true`, Compozy will automatically discover and load resources
	// matching the patterns specified in `include`. When `false`, all resources
	// must be explicitly defined in workflow configurations.
	//
	// **Default**: `false` (explicit resource definition required)
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`

	// Strict controls error handling when resources cannot be loaded.
	//
	// When `true` (default), any failure to load a discovered resource
	// will cause the workflow to fail immediately. When `false`, load
	// errors are logged but workflow execution continues.
	//
	// **Use cases for non-strict mode**:
	//   - Development environments with partial resources
	//   - Graceful degradation in production
	//   - Migration periods when refactoring resources
	//
	// **Default**: `true` (fail on any load error)
	Strict bool `json:"strict" yaml:"strict" mapstructure:"strict"`

	// Include specifies glob patterns for discovering resources.
	//
	// **Common patterns**:
	//   - `"agents/*.yaml"` - Direct children only
	//   - `"agents/**/*.yaml"` - All nested files
	//   - `"tools/*-tool.yaml"` - Files ending with "-tool"
	//   - `"memory/*/config.yaml"` - Config files in subdirectories
	//
	// **Required when**: `enabled` is `true`
	Include []string `json:"include" yaml:"include" mapstructure:"include" validate:"required_if=Enabled true,dive,required"`

	// Exclude specifies patterns for files to ignore during discovery.
	//
	// **Common exclusion patterns**:
	//   - `"**/*-test.yaml"` - Test fixtures
	//   - `"**/*-draft.yaml"` - Work in progress
	//   - `"**/archive/**"` - Archived resources
	//   - `"**/.backup/**"` - Backup directories
	Exclude []string `json:"exclude,omitempty" yaml:"exclude,omitempty" mapstructure:"exclude,omitempty"`

	// WatchEnabled activates file system monitoring for hot reloading.
	//
	// When `true`, Compozy monitors included directories for changes
	// and automatically reloads modified resources without restarting.
	// This is particularly useful during development.
	//
	// **Note**: Watch mode may impact performance in directories with
	// many files. Use specific include patterns to limit scope.
	//
	// **Default**: `false`
	WatchEnabled bool `json:"watch_enabled,omitempty" yaml:"watch_enabled,omitempty" mapstructure:"watch_enabled,omitempty"`

	defaultsApplied bool // Internal flag to ensure SetDefaults is idempotent
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
	if c.defaultsApplied {
		return
	}
	// Strict mode defaults to true only when autoload is disabled
	if !c.Enabled {
		c.Strict = true
	}
	c.defaultsApplied = true
}

// GetAllExcludes returns the combined list of default and user-defined excludes
func (c *Config) GetAllExcludes() []string {
	out := make([]string, 0, len(DefaultExcludes)+len(c.Exclude))
	out = append(out, DefaultExcludes...)
	return append(out, c.Exclude...)
}

// MergeDefaults merges default values into the configuration
func (c *Config) MergeDefaults() {
	c.SetDefaults()
}
