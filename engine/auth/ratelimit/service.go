package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/time/rate"
)

// Config holds rate limiting configuration
type Config struct {
	// Default requests per second per API key when no custom limit is provided
	DefaultRequestsPerSecond int
	// Default burst capacity per API key
	DefaultBurstCapacity int
	// Cleanup interval for unused limiters
	CleanupInterval time.Duration
	// Limiter expiry time (how long to keep unused limiters)
	LimiterExpiry time.Duration
}

// DefaultConfig returns default rate limiting configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultRequestsPerSecond: 100,
		DefaultBurstCapacity:     20,
		CleanupInterval:          time.Hour,
		LimiterExpiry:            24 * time.Hour,
	}
}

// limiterEntry holds a rate limiter and its last access time
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// Service provides rate limiting functionality
type Service struct {
	mu       sync.RWMutex
	limiters map[string]*limiterEntry
	config   *Config
	done     chan struct{}
}

// NewService creates a new rate limiting service
func NewService(config *Config) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	s := &Service{
		limiters: make(map[string]*limiterEntry),
		config:   config,
		done:     make(chan struct{}),
	}
	// Start cleanup goroutine
	go s.cleanupLoop()
	return s
}

// Stop stops the rate limiting service
func (s *Service) Stop() {
	close(s.done)
}

// CheckRateLimit checks if a request is allowed for the given API key
func (s *Service) CheckRateLimit(_ context.Context, apiKeyID core.ID, customLimit *int) error {
	limiter := s.getLimiter(apiKeyID.String(), customLimit)
	if !limiter.Allow() {
		return core.NewError(nil, "RATE_LIMIT_EXCEEDED", map[string]any{
			"api_key_id": apiKeyID,
			"limit_rps":  limiter.Limit(),
			"burst":      limiter.Burst(),
		})
	}
	return nil
}

// CheckRateLimitWithN checks if n requests are allowed for the given API key
func (s *Service) CheckRateLimitWithN(_ context.Context, apiKeyID core.ID, n int, customLimit *int) error {
	limiter := s.getLimiter(apiKeyID.String(), customLimit)
	if !limiter.AllowN(time.Now(), n) {
		return core.NewError(nil, "RATE_LIMIT_EXCEEDED", map[string]any{
			"api_key_id": apiKeyID,
			"limit_rps":  limiter.Limit(),
			"burst":      limiter.Burst(),
			"requested":  n,
		})
	}
	return nil
}

// getLimiter gets or creates a rate limiter for the given key
func (s *Service) getLimiter(keyID string, customLimit *int) *rate.Limiter {
	s.mu.RLock()
	entry, exists := s.limiters[keyID]
	s.mu.RUnlock()
	if exists {
		// Update last access time
		s.mu.Lock()
		entry.lastAccess = time.Now()
		s.mu.Unlock()
		// If custom limit is provided and different, update the limiter
		if customLimit != nil {
			// Convert from requests per hour to requests per second
			customLimitRPS := float64(*customLimit) / 3600.0
			if entry.limiter.Limit() != rate.Limit(customLimitRPS) {
				s.mu.Lock()
				entry.limiter.SetLimit(rate.Limit(customLimitRPS))
				s.mu.Unlock()
			}
		}
		return entry.limiter
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check pattern
	if entry, exists := s.limiters[keyID]; exists {
		entry.lastAccess = time.Now()
		return entry.limiter
	}
	// Determine rate limit - convert from per-hour to per-second
	limitRPS := float64(s.config.DefaultRequestsPerSecond)
	if customLimit != nil && *customLimit > 0 {
		// Custom limit is in requests per hour, convert to per second
		limitRPS = float64(*customLimit) / 3600.0
	}
	// Create new limiter for this API key
	limiter := rate.NewLimiter(
		rate.Limit(limitRPS),
		s.config.DefaultBurstCapacity,
	)
	s.limiters[keyID] = &limiterEntry{
		limiter:    limiter,
		lastAccess: time.Now(),
	}
	return limiter
}

// cleanupLoop periodically removes unused limiters
func (s *Service) cleanupLoop() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.done:
			return
		}
	}
}

// cleanup removes expired limiters
func (s *Service) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	expired := 0
	for keyID, entry := range s.limiters {
		if now.Sub(entry.lastAccess) > s.config.LimiterExpiry {
			delete(s.limiters, keyID)
			expired++
		}
	}
	if expired > 0 {
		// Note: We don't have a context here, so we use the default logger
		// In production, consider passing context through the cleanup goroutine
		log := logger.FromContext(context.Background())
		log.With("expired_count", expired, "remaining_count", len(s.limiters)).
			Debug("Cleaned up expired rate limiters")
	}
}

// GetStats returns statistics about the rate limiter
func (s *Service) GetStats() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"total_limiters": len(s.limiters),
		"config":         s.config,
	}
}
