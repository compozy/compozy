package runtime

import (
	"os"
	"time"

	appconfig "github.com/compozy/compozy/pkg/config"
)

// Config holds configuration for the RuntimeManager
type Config struct {
	BackoffInitialInterval time.Duration
	BackoffMaxInterval     time.Duration
	BackoffMaxElapsedTime  time.Duration
	WorkerFilePerm         os.FileMode
	ToolExecutionTimeout   time.Duration
	// Runtime selection fields
	RuntimeType    string   // "bun" or "node"
	EntrypointPath string   // Path to entrypoint file
	BunPermissions []string // Bun-specific permissions
	NodeOptions    []string // Node.js-specific options
	// Application config integration fields
	Environment string // Deployment environment (development, staging, production)
	// Memory management
	MaxMemoryMB          int // Maximum memory limit in MB for the runtime process (0 = no limit)
	MaxStderrCaptureSize int // Maximum size of stderr buffer to capture (default 1MB)
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		BackoffInitialInterval: 100 * time.Millisecond,
		BackoffMaxInterval:     5 * time.Second,
		BackoffMaxElapsedTime:  30 * time.Second,
		WorkerFilePerm:         0600, // Secure file permissions
		ToolExecutionTimeout:   60 * time.Second,
		RuntimeType:            RuntimeTypeBun, // Default to Bun runtime
		BunPermissions: []string{
			"--allow-read", // Minimal permissions - allow read only by default
		},
		Environment:          "development",   // Default environment
		MaxMemoryMB:          2048,            // Default 2GB memory limit
		MaxStderrCaptureSize: 1 * 1024 * 1024, // Default 1MB stderr buffer
	}
}

func TestConfig() *Config {
	return &Config{
		BackoffInitialInterval: 10 * time.Millisecond,
		BackoffMaxInterval:     100 * time.Millisecond,
		BackoffMaxElapsedTime:  1 * time.Second, // Much shorter for tests
		WorkerFilePerm:         0600,            // Secure permissions for tests
		ToolExecutionTimeout:   5 * time.Second, // Shorter timeout for tests
		RuntimeType:            RuntimeTypeBun,  // Default to Bun for tests
		BunPermissions: []string{
			"--allow-read",
		},
		Environment: "testing", // Test environment
		MaxMemoryMB: 512,       // Lower memory limit for tests
	}
}

// FromAppConfig creates a runtime Config from the application's RuntimeConfig.
//
// This method consolidates configuration by converting from the centralized
// pkg/config.RuntimeConfig to the runtime-specific Config structure, applying
// appropriate defaults and mappings.
//
// **Mapping Strategy:**
//   - Direct field mappings where names/types match
//   - Default values applied for runtime-specific settings
//   - Advanced runtime features use sensible production defaults
//
// **Example Usage:**
//
//	appConfig := &config.RuntimeConfig{
//	  Environment: "production",
//	  ToolExecutionTimeout: 30*time.Second,
//	}
//	runtimeConfig := FromAppConfig(appConfig)
func FromAppConfig(appConfig *appconfig.RuntimeConfig) *Config {
	if appConfig == nil {
		return DefaultConfig()
	}
	config := DefaultConfig()
	if appConfig.Environment != "" {
		config.Environment = appConfig.Environment
	}
	if appConfig.ToolExecutionTimeout > 0 {
		config.ToolExecutionTimeout = appConfig.ToolExecutionTimeout
	}
	if appConfig.RuntimeType != "" {
		config.RuntimeType = appConfig.RuntimeType
	}
	if appConfig.EntrypointPath != "" {
		config.EntrypointPath = appConfig.EntrypointPath
	}
	if len(appConfig.BunPermissions) > 0 {
		config.BunPermissions = appConfig.BunPermissions
	}
	return config
}
