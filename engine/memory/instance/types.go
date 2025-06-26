package instance

import (
	"time"
)

// Config represents the runtime configuration for a memory instance
type Config struct {
	// Resource configuration
	Resource any // Will be *core.Resource when fully migrated

	// TTL configurations
	AppendTTL time.Duration
	ClearTTL  time.Duration
	FlushTTL  time.Duration

	// Operational settings
	EnableMetrics bool
	EnableTracing bool

	// Flush settings
	DisableFlush      bool
	FlushThreshold    float64 // Percentage of capacity that triggers flush
	FlushBatchSize    int     // Number of messages to flush at once
	FlushRetryLimit   int     // Maximum retries for flush operations
	FlushRetryBackoff time.Duration
}

// State represents the current state of a memory instance
type State struct {
	MessageCount int
	TokenCount   int
	LastAppend   time.Time
	LastFlush    time.Time
	FlushPending bool
	Healthy      bool
	Errors       []error
}

// AsyncOperationLogger provides structured logging for async operations
type AsyncOperationLogger struct {
	_ string // instanceID - Will be used when logger is migrated
	_ any    // logger - Will be properly typed when logger is migrated
}

// LogAppend logs an append operation
func (l *AsyncOperationLogger) LogAppend(_ any, _ int) {
	// Implementation will be added when logger is migrated
}

// LogFlush logs a flush operation
func (l *AsyncOperationLogger) LogFlush(_, _ int) {
	// Implementation will be added when logger is migrated
}

// LogError logs an error
func (l *AsyncOperationLogger) LogError(_ string, _ error) {
	// Implementation will be added when logger is migrated
}
