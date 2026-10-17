package config

import (
	"fmt"
	"time"
)

// SessionMessagingConfig bounds agent-to-agent messages sent with compozy__session_prompt.
type SessionMessagingConfig struct {
	// RateLimitMessages is the most messages one sender session may admit into one target per window.
	// -1 disables the budget; 0 means the default.
	RateLimitMessages int `toml:"rate_limit_messages,omitempty"`
	// RateLimitWindow is the sliding window the per sender→target budget is counted over.
	RateLimitWindow time.Duration `toml:"rate_limit_window,omitempty"`
	// ReplyWatchTTL releases an armed reply watch with an expired outcome when no reply arrives in time,
	// so a forgotten watch stops keeping its sender awake. An explicit zero never expires a watch.
	ReplyWatchTTL time.Duration `toml:"reply_watch_ttl"`
	// QueuePromoteAfter promotes a queued message from another session into the live turn as injected
	// steering once the target has been in the same turn this long and supports injection. Zero disables it.
	QueuePromoteAfter time.Duration `toml:"queue_promote_after"`
}

const (
	minSessionMessagingRateLimit  = 1
	maxSessionMessagingRateLimit  = 10000
	minSessionMessagingRateWindow = time.Second
	maxSessionMessagingRateWindow = 24 * time.Hour
	// SessionMessagingRateLimitDisabled turns the per sender→target message budget off.
	SessionMessagingRateLimitDisabled = -1
	minSessionReplyWatchTTL           = time.Minute
	maxSessionReplyWatchTTL           = 30 * 24 * time.Hour
	minSessionQueuePromoteAfter       = 10 * time.Second
	maxSessionQueuePromoteAfter       = 24 * time.Hour
)

// DefaultSessionMessagingConfig returns the default agent messaging guards.
func DefaultSessionMessagingConfig() SessionMessagingConfig {
	return SessionMessagingConfig{
		RateLimitMessages: 30,
		RateLimitWindow:   10 * time.Minute,
		ReplyWatchTTL:     24 * time.Hour,
		QueuePromoteAfter: 2 * time.Minute,
	}
}

// Normalize returns a config copy with implicit defaults applied. ReplyWatchTTL and QueuePromoteAfter
// keep an explicit zero because zero disables them.
func (c SessionMessagingConfig) Normalize() SessionMessagingConfig {
	defaults := DefaultSessionMessagingConfig()
	if c.RateLimitMessages == 0 {
		c.RateLimitMessages = defaults.RateLimitMessages
	}
	if c.RateLimitWindow == 0 {
		c.RateLimitWindow = defaults.RateLimitWindow
	}
	return c
}

// Validate ensures the messaging guards stay inside their supported bounds.
func (c SessionMessagingConfig) Validate() error {
	normalized := c.Normalize()
	if normalized.RateLimitMessages != SessionMessagingRateLimitDisabled &&
		(normalized.RateLimitMessages < minSessionMessagingRateLimit ||
			normalized.RateLimitMessages > maxSessionMessagingRateLimit) {
		return fmt.Errorf(
			"session.messaging.rate_limit_messages must be -1 (disabled) or between %d and %d: %d",
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
	if normalized.ReplyWatchTTL != 0 &&
		(normalized.ReplyWatchTTL < minSessionReplyWatchTTL || normalized.ReplyWatchTTL > maxSessionReplyWatchTTL) {
		return fmt.Errorf(
			"session.messaging.reply_watch_ttl must be 0 (never expire) or between %s and %s: %s",
			minSessionReplyWatchTTL,
			maxSessionReplyWatchTTL,
			normalized.ReplyWatchTTL,
		)
	}
	if normalized.QueuePromoteAfter != 0 && (normalized.QueuePromoteAfter < minSessionQueuePromoteAfter ||
		normalized.QueuePromoteAfter > maxSessionQueuePromoteAfter) {
		return fmt.Errorf(
			"session.messaging.queue_promote_after must be 0 or between %s and %s: %s",
			minSessionQueuePromoteAfter,
			maxSessionQueuePromoteAfter,
			normalized.QueuePromoteAfter,
		)
	}
	return nil
}

type sessionMessagingOverlay struct {
	RateLimitMessages *int           `toml:"rate_limit_messages"`
	RateLimitWindow   *time.Duration `toml:"rate_limit_window"`
	ReplyWatchTTL     *time.Duration `toml:"reply_watch_ttl"`
	QueuePromoteAfter *time.Duration `toml:"queue_promote_after"`
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
	if o.QueuePromoteAfter != nil {
		dst.QueuePromoteAfter = *o.QueuePromoteAfter
	}
}
