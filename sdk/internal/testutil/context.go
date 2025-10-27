package testutil

import (
	"context"
	"testing"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// NewTestContext returns a context derived from t.Context() with test logger and configuration manager attached.
func NewTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx := t.Context()
	ctx = WithTestLogger(t, ctx)
	ctx = WithTestConfig(t, ctx)
	return ctx
}

// WithTestLogger returns a copy of ctx containing a logger configured for tests.
func WithTestLogger(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	log := logger.NewForTests()
	return logger.ContextWithLogger(ctx, log)
}

// WithTestConfig returns a copy of ctx containing a configuration manager loaded with defaults suitable for tests.
func WithTestConfig(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	manager := config.NewManager(ctx, config.NewService())
	if _, err := manager.Load(ctx, config.NewDefaultProvider()); err != nil {
		t.Fatalf("failed to load test configuration: %v", err)
	}
	t.Cleanup(func() {
		_ = manager.Close(context.WithoutCancel(ctx))
	})
	return config.ContextWithManager(ctx, manager)
}
