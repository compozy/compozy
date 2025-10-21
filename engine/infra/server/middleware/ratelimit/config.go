package ratelimit

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/ulule/limiter/v3"
)

const (
	defaultGlobalRateLimit     = 100
	defaultAPIKeyRateLimit     = 100
	defaultMemoryRateLimit     = 200
	defaultHooksRateLimit      = 60
	defaultWorkflowRateLimit   = 100
	defaultTaskRateLimit       = 100
	defaultAuthRateLimit       = 20
	defaultUsersRateLimit      = 30
	defaultRatePeriod          = time.Minute
	defaultMaxRetry            = 3
	defaultHealthCheckInterval = 30 * time.Second
)

// Config represents rate limiting configuration
type Config struct {
	// Global rate limit settings
	GlobalRate RateConfig `yaml:"global_rate"`

	// Per-key rate limit for API keys
	APIKeyRate RateConfig `yaml:"api_key_rate"`

	// Per-route rate limits
	RouteRates map[string]RateConfig `yaml:"route_rates"`

	// Redis configuration
	RedisAddr     string `yaml:"redis_addr"`
	RedisPassword string `yaml:"redis_password"`
	RedisDB       int    `yaml:"redis_db"`

	// Options
	Prefix              string        `yaml:"prefix"`
	MaxRetry            int           `yaml:"max_retry"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`

	// Header configuration
	DisableHeaders bool `yaml:"disable_headers"`

	// Exclude patterns
	ExcludedPaths []string `yaml:"excluded_paths"`

	// Excluded IPs
	ExcludedIPs []string `yaml:"excluded_ips"`

	// Fail-open strategy
	FailOpen bool `yaml:"fail_open"`
}

// RateConfig represents a single rate limit configuration
type RateConfig struct {
	Period   time.Duration `yaml:"period"`
	Limit    int64         `yaml:"limit"`
	Disabled bool          `yaml:"disabled,omitempty"`
}

// DefaultConfig returns default rate limiting configuration
func DefaultConfig() *Config {
	return &Config{
		GlobalRate:          defaultGlobalRate(),
		APIKeyRate:          defaultAPIKeyRate(),
		RouteRates:          defaultRouteRates(),
		RedisAddr:           "",
		RedisPassword:       "",
		RedisDB:             0,
		Prefix:              "compozy:ratelimit:",
		MaxRetry:            defaultMaxRetry,
		HealthCheckInterval: defaultHealthCheckInterval,
		DisableHeaders:      false,
		ExcludedPaths:       defaultExcludedPaths(),
		ExcludedIPs:         nil,
		FailOpen:            true,
	}
}

// defaultGlobalRate returns the default global rate limiter configuration.
func defaultGlobalRate() RateConfig {
	return RateConfig{
		Limit:    defaultGlobalRateLimit,
		Period:   defaultRatePeriod,
		Disabled: false,
	}
}

// defaultAPIKeyRate returns the default API key rate limiter configuration.
func defaultAPIKeyRate() RateConfig {
	return RateConfig{
		Limit:    defaultAPIKeyRateLimit,
		Period:   defaultRatePeriod,
		Disabled: false,
	}
}

// defaultRouteRates returns default per-route rate limit settings.
func defaultRouteRates() map[string]RateConfig {
	return map[string]RateConfig{
		routes.Base() + "/memory": {
			Limit:    defaultMemoryRateLimit,
			Period:   defaultRatePeriod,
			Disabled: false,
		},
		routes.Hooks(): {
			Limit:    defaultHooksRateLimit,
			Period:   defaultRatePeriod,
			Disabled: false,
		},
		routes.Base() + "/workflow": {
			Limit:    defaultWorkflowRateLimit,
			Period:   defaultRatePeriod,
			Disabled: false,
		},
		routes.Base() + "/task": {
			Limit:    defaultTaskRateLimit,
			Period:   defaultRatePeriod,
			Disabled: false,
		},
		routes.Base() + "/auth": {
			Limit:    defaultAuthRateLimit,
			Period:   defaultRatePeriod,
			Disabled: false,
		},
		routes.Base() + "/users": {
			Limit:    defaultUsersRateLimit,
			Period:   defaultRatePeriod,
			Disabled: false,
		},
	}
}

// defaultExcludedPaths returns paths excluded from rate limiting.
func defaultExcludedPaths() []string {
	return []string{
		"/health",
		routes.HealthVersioned(),
		"/healthz",
		"/readyz",
		"/mcp-proxy/health",
		"/metrics",
		"/swagger",
	}
}

// ToLimiterRate converts RateConfig to limiter.Rate
func (rc RateConfig) ToLimiterRate() limiter.Rate {
	return limiter.Rate{
		Period: rc.Period,
		Limit:  rc.Limit,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if !c.GlobalRate.Disabled && (c.GlobalRate.Limit <= 0 || c.GlobalRate.Period <= 0) {
		return fmt.Errorf("global rate limit must be positive with non-zero period")
	}
	if !c.APIKeyRate.Disabled && (c.APIKeyRate.Limit <= 0 || c.APIKeyRate.Period <= 0) {
		return fmt.Errorf("API key rate limit must be positive with non-zero period")
	}
	for route, rate := range c.RouteRates {
		if rate.Disabled {
			continue
		}
		if rate.Limit <= 0 || rate.Period <= 0 {
			return fmt.Errorf("route rate limit for %s must be positive with non-zero period", route)
		}
	}
	return nil
}
