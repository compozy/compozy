package auth

import (
	"time"

	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/worker"
)

// Config holds the complete configuration for the auth module
type Config struct {
	// Organization service configuration
	Organization *org.Config `json:"organization"`

	// Temporal service configuration
	Temporal *worker.TemporalConfig `json:"temporal"`

	// API Key configuration
	APIKey *APIKeyConfig `json:"api_key"`

	// Rate limiting configuration
	RateLimit *RateLimitConfig `json:"rate_limit"`
}

// APIKeyConfig holds configuration for API key management
type APIKeyConfig struct {
	// Argon2 parameters
	Argon2Time    uint32 `json:"argon2_time"`
	Argon2Memory  uint32 `json:"argon2_memory"`
	Argon2Threads uint8  `json:"argon2_threads"`
	Argon2KeyLen  uint32 `json:"argon2_key_len"`

	// Key generation
	KeyLength int    `json:"key_length"`
	KeyPrefix string `json:"key_prefix"`

	// Expiration settings
	DefaultExpirationDays int `json:"default_expiration_days"`
}

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// Default requests per hour
	DefaultRequestsPerHour int `json:"default_requests_per_hour"`

	// Burst capacity
	BurstCapacity int `json:"burst_capacity"`

	// Cleanup interval for unused limiters
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// DefaultConfig returns the default auth configuration
func DefaultConfig() *Config {
	return &Config{
		Organization: org.DefaultConfig(),
		Temporal:     worker.DefaultTemporalConfig(),
		APIKey:       DefaultAPIKeyConfig(),
		RateLimit:    DefaultRateLimitConfig(),
	}
}

// DefaultAPIKeyConfig returns the default API key configuration
func DefaultAPIKeyConfig() *APIKeyConfig {
	return &APIKeyConfig{
		Argon2Time:            1,
		Argon2Memory:          64 * 1024,
		Argon2Threads:         4,
		Argon2KeyLen:          32,
		KeyLength:             16, // 16 bytes = 32 hex chars
		KeyPrefix:             "cmpz_",
		DefaultExpirationDays: 365, // 1 year default
	}
}

// DefaultRateLimitConfig returns the default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		DefaultRequestsPerHour: 3600,      // 1 request per second
		BurstCapacity:          20,        // Allow bursts of 20 requests
		CleanupInterval:        time.Hour, // Cleanup unused limiters hourly
	}
}
