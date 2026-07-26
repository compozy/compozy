package clientstate

import (
	"errors"
	"log/slog"
	"path/filepath"
	"time"
)

const (
	// StateDirName is the AGH-home directory that owns client-state data.
	StateDirName = "state"
	// DatabaseName is the client-state bbolt filename.
	DatabaseName = "clientstate.db"
	// SubscriptionBufferSize is the fixed per-subscription event capacity.
	SubscriptionBufferSize = 256

	defaultMaxValueBytes       = 256 * 1024
	defaultMaxKeysPerWorkspace = 512
	defaultOpenTimeout         = time.Second
)

// Limits bounds retained data in the client-state store.
type Limits struct {
	MaxValueBytes       int
	MaxKeysPerWorkspace int
}

// DefaultLimits returns the production client-state limits.
func DefaultLimits() Limits {
	return Limits{
		MaxValueBytes:       defaultMaxValueBytes,
		MaxKeysPerWorkspace: defaultMaxKeysPerWorkspace,
	}
}

func (l Limits) validate() error {
	if l.MaxValueBytes <= 0 {
		return errors.New("clientstate: max value bytes must be positive")
	}
	if l.MaxKeysPerWorkspace <= 0 {
		return errors.New("clientstate: max keys per workspace must be positive")
	}
	return nil
}

// DatabasePath returns the canonical client-state path for an AGH home.
func DatabasePath(homeDir string) string {
	return filepath.Join(homeDir, StateDirName, DatabaseName)
}

type engineOptions struct {
	logger      *slog.Logger
	now         func() time.Time
	openTimeout time.Duration
}

// Option customizes Engine construction.
type Option func(*engineOptions) error

// WithLogger configures structured client-state logging.
func WithLogger(logger *slog.Logger) Option {
	return func(opts *engineOptions) error {
		if logger == nil {
			return errors.New("clientstate: logger is required")
		}
		opts.logger = logger
		return nil
	}
}

// WithClock replaces the wall clock for deterministic tests.
func WithClock(now func() time.Time) Option {
	return func(opts *engineOptions) error {
		if now == nil {
			return errors.New("clientstate: clock is required")
		}
		opts.now = now
		return nil
	}
}

// WithOpenTimeout bounds waiting for another process to release the bbolt file.
func WithOpenTimeout(timeout time.Duration) Option {
	return func(opts *engineOptions) error {
		if timeout <= 0 {
			return errors.New("clientstate: open timeout must be positive")
		}
		opts.openTimeout = timeout
		return nil
	}
}

func resolveEngineOptions(options []Option) (engineOptions, error) {
	resolved := engineOptions{
		logger:      slog.Default(),
		now:         time.Now,
		openTimeout: defaultOpenTimeout,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&resolved); err != nil {
			return engineOptions{}, err
		}
	}
	resolved.logger = resolved.logger.With("component", "clientstate")
	return resolved, nil
}
