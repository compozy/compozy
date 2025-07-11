package monitoring

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config defines the monitoring and observability configuration for Compozy.
//
// **Monitoring** provides insights into workflow execution, performance metrics,
// and system health. When enabled, Compozy exposes a metrics endpoint compatible
// with Prometheus and other monitoring systems.
//
// Key features:
//   - **Performance metrics**: Track workflow execution times, task durations, and throughput
//   - **Resource utilization**: Monitor memory usage, goroutines, and connection pools
//   - **Error tracking**: Count and categorize failures across workflows and tasks
//   - **Business metrics**: Custom counters for domain-specific events
//
// ## Basic Monitoring Configuration
//
//	monitoring:
//	  enabled: true
//
// ## Custom Metrics Path
//
//	monitoring:
//	  enabled: true
//	  path: /internal/metrics  # Custom endpoint
//
// ## Environment Variable Override
//
// Monitoring can be configured via environment variables:
//
//	export MONITORING_ENABLED=true
//	export MONITORING_PATH=/metrics
//
// Environment variables take precedence over YAML configuration.
type Config struct {
	// Enabled activates the monitoring endpoint and metric collection.
	//
	// When `true`, Compozy will:
	//   - Start collecting internal metrics (CPU, memory, goroutines)
	//   - Track workflow and task execution metrics
	//   - Expose metrics at the configured path
	//   - Enable custom metric registration
	//
	// **Default**: `false` (no metrics collection)
	//
	// **Environment variable**: `MONITORING_ENABLED`
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled" env:"MONITORING_ENABLED"`

	// Path specifies the HTTP endpoint for exposing metrics.
	//
	// This endpoint returns metrics in Prometheus exposition format,
	// compatible with:
	//   - Prometheus scraping
	//   - Grafana agent
	//   - VictoriaMetrics
	//   - Other Prometheus-compatible systems
	//
	// **Requirements**:
	//   - Must start with `/`
	//   - Cannot conflict with API routes (`/api/*`)
	//   - Should not contain query parameters
	//
	// **Common paths**:
	//   - `/metrics` - Standard Prometheus convention
	//   - `/internal/metrics` - For internal-only access
	//   - `/monitoring/metrics` - Namespaced approach
	//
	// **Default**: `/metrics`
	//
	// **Environment variable**: `MONITORING_PATH`
	Path string `json:"path" yaml:"path" mapstructure:"path" env:"MONITORING_PATH"`
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
func LoadWithEnv(_ context.Context, yamlConfig *Config) (*Config, error) {
	config := DefaultConfig()
	if yamlConfig != nil {
		config.Enabled = yamlConfig.Enabled
		if yamlConfig.Path != "" {
			config.Path = yamlConfig.Path
		}
	}
	// Environment variable takes precedence for Enabled flag
	if envEnabled := os.Getenv("MONITORING_ENABLED"); envEnabled != "" {
		enabled, err := strconv.ParseBool(envEnabled)
		if err == nil {
			config.Enabled = enabled
		}
	}
	// Environment variable takes precedence for Path
	if envPath := os.Getenv("MONITORING_PATH"); envPath != "" {
		config.Path = envPath
	}
	// Validate configuration before returning
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid monitoring configuration: %w", err)
	}
	return config, nil
}
