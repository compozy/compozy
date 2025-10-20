package config

import (
	"context"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
)

// ContextKey is an alias used for storing values in context
type ContextKey string

const (
	// ManagerCtxKey is the context key used to store the *Manager instance
	ManagerCtxKey ContextKey = "config_manager"
)

// ContextWithManager stores the configuration manager in the context
func ContextWithManager(ctx context.Context, m *Manager) context.Context {
	return context.WithValue(ctx, ManagerCtxKey, m)
}

var defaultManager *Manager
var defaultManagerOnce sync.Once

// ManagerFromContext retrieves the configuration manager from the context.
// If none is found, it falls back to a lazily-initialized default manager
// that loads defaults and environment variables. This mirrors the logger
// package behavior and ensures components have a usable configuration in
// edge cases where the manager was not explicitly attached to context.
func ManagerFromContext(ctx context.Context) *Manager {
	if ctx != nil {
		if m, ok := ctx.Value(ManagerCtxKey).(*Manager); ok && m != nil {
			return m
		}
	}
	return getDefaultManager(ctx)
}

// FromContext returns the active configuration (*Config) for the provided context.
func FromContext(ctx context.Context) *Config {
	m := ManagerFromContext(ctx)
	if m == nil {
		return nil
	}
	return m.Get()
}

// getDefaultManager returns a singleton default manager initialized with
// built-in defaults and environment overrides. YAML/CLI sources are not
// applied here; callers that need them must construct a Manager and attach
// it to the context explicitly.
func getDefaultManager(ctx context.Context) *Manager {
	defaultManagerOnce.Do(func() {
		m := NewManager(ctx, NewService())
		if _, err := m.Load(ctx, NewDefaultProvider(), NewEnvProvider()); err != nil {
			log := logger.FromContext(ctx)
			log.Warn("failed to load default configuration, using fallback defaults", "error", err)
		}
		defaultManager = m
	})
	return defaultManager
}
