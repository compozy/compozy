package ratelimit

import (
	"fmt"
	"time"

	"github.com/ulule/limiter/v3"
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
		GlobalRate: RateConfig{
			Limit:    100,
			Period:   1 * time.Minute,
			Disabled: false,
		},
		APIKeyRate: RateConfig{
			Limit:    100,
			Period:   1 * time.Minute,
			Disabled: false,
		},
		RouteRates: map[string]RateConfig{
			"/api/v0/memory": {
				Limit:    200, // More reasonable for memory operations
				Period:   1 * time.Minute,
				Disabled: false,
			},
			"/api/v0/workflow": {
				Limit:    100,
				Period:   1 * time.Minute,
				Disabled: false,
			},
			"/api/v0/task": {
				Limit:    100,
				Period:   1 * time.Minute,
				Disabled: false,
			},
			// Auth endpoints - stricter limits to prevent brute force attacks
			"/api/v0/auth": {
				Limit:    20, // Stricter limit for auth operations
				Period:   1 * time.Minute,
				Disabled: false,
			},
			"/api/v0/users": {
				Limit:    30, // Moderate limit for user management (admin only)
				Period:   1 * time.Minute,
				Disabled: false,
			},
		},
		// RedisAddr is the address of the Redis server. If empty, an in-memory store is used.
		RedisAddr:           "",
		RedisPassword:       "",
		RedisDB:             0,
		Prefix:              "compozy:ratelimit:",
		MaxRetry:            3,
		HealthCheckInterval: 30 * time.Second,
		DisableHeaders:      false,
		ExcludedPaths: []string{
			"/health",
			"/metrics",
			"/swagger",
			"/api/v0/health",
		},
		ExcludedIPs: []string{},
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
	if c.GlobalRate.Limit <= 0 {
		return fmt.Errorf("global rate limit must be positive")
	}
	if c.APIKeyRate.Limit <= 0 {
		return fmt.Errorf("API key rate limit must be positive")
	}
	for route, rate := range c.RouteRates {
		if rate.Limit <= 0 {
			return fmt.Errorf("route rate limit for %s must be positive", route)
		}
	}
	return nil
}
