package config

import (
	"fmt"
	"time"
)

// SessionMessagingConfig bounds agent-to-agent messages sent with compozy__session_prompt.
type SessionMessagingConfig struct {
	// RateLimitMessages is the most messages one sender session may admit into one target per window.
	RateLimitMessages int `toml:"rate_limit_messages,omitempty"`
	// RateLimitWindow is the sliding window the per sender→target budget is counted over.
	RateLimitWindow time.Duration `toml:"rate_limit_window,omitempty"`
	// ReplyWatchTTL releases an armed reply watch with an expired outcome when no reply arrives in time.
	ReplyWatchTTL time.Duration `toml:"reply_watch_ttl,omitempty"`
}

const (
	minSessionMessagingRateLimit  = 1
	maxSessionMessagingRateLimit  = 10000
	minSessionMessagingRateWindow = time.Second
	maxSessionMessagingRateWindow = 24 * time.Hour
	minSessionReplyWatchTTL       = time.Minute
	maxSessionReplyWatchTTL       = 30 * 24 * time.Hour
)

// DefaultSessionMessagingConfig returns the default agent messaging guards.
func DefaultSessionMessagingConfig() SessionMessagingConfig {
	return SessionMessagingConfig{
		RateLimitMessages: 30,
		RateLimitWindow:   10 * time.Minute,
		ReplyWatchTTL:     24 * time.Hour,
	}
}

// Normalize returns a config copy with implicit defaults applied.
func (c SessionMessagingConfig) Normalize() SessionMessagingConfig {
	defaults := DefaultSessionMessagingConfig()
	if c.RateLimitMessages == 0 {
		c.RateLimitMessages = defaults.RateLimitMessages
	}
	if c.RateLimitWindow == 0 {
		c.RateLimitWindow = defaults.RateLimitWindow
	}
	if c.ReplyWatchTTL == 0 {
		c.ReplyWatchTTL = defaults.ReplyWatchTTL
	}
	return c
}

// Validate ensures the messaging guards stay inside their supported bounds.
func (c SessionMessagingConfig) Validate() error {
	normalized := c.Normalize()
	if normalized.RateLimitMessages < minSessionMessagingRateLimit ||
		normalized.RateLimitMessages > maxSessionMessagingRateLimit {
		return fmt.Errorf(
			"session.messaging.rate_limit_messages must be between %d and %d: %d",
			minSessionMessagingRateLimit,
			maxSessionMessagingRateLimit,
			normalized.RateLimitMessages,
		)
	}
	if normalized.RateLimitWindow < minSessionMessagingRateWindow ||
		normalized.RateLimitWindow > maxSessionMessagingRateWindow {
		return fmt.Errorf(
			"session.messaging.rate_limit_window must be between %s and %s: %s",
			minSessionMessagingRateWindow,
			maxSessionMessagingRateWindow,
			normalized.RateLimitWindow,
		)
	}
	if normalized.ReplyWatchTTL < minSessionReplyWatchTTL || normalized.ReplyWatchTTL > maxSessionReplyWatchTTL {
		return fmt.Errorf(
			"session.messaging.reply_watch_ttl must be between %s and %s: %s",
			minSessionReplyWatchTTL,
			maxSessionReplyWatchTTL,
			normalized.ReplyWatchTTL,
		)
	}
	return nil
}

type sessionMessagingOverlay struct {
	RateLimitMessages *int           `toml:"rate_limit_messages"`
	RateLimitWindow   *time.Duration `toml:"rate_limit_window"`
	ReplyWatchTTL     *time.Duration `toml:"reply_watch_ttl"`
}

func (o sessionMessagingOverlay) Apply(dst *SessionMessagingConfig) {
	if o.RateLimitMessages != nil {
		dst.RateLimitMessages = *o.RateLimitMessages
	}
	if o.RateLimitWindow != nil {
		dst.RateLimitWindow = *o.RateLimitWindow
	}
	if o.ReplyWatchTTL != nil {
		dst.ReplyWatchTTL = *o.ReplyWatchTTL
	}
}
