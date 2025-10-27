package testutil

import (
	"context"
	"testing"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// NewTestContext returns a context derived from tb.Context() with test logger and configuration manager attached.
func NewTestContext(tb testing.TB) context.Context {
	tb.Helper()
	ctx := contextFromTB(tb)
	ctx = WithTestLogger(tb, ctx)
	ctx = WithTestConfig(tb, ctx)
	return ctx
}

// WithTestLogger returns a copy of ctx containing a logger configured for tests.
func WithTestLogger(tb testing.TB, ctx context.Context) context.Context {
	tb.Helper()
	log := logger.NewForTests()
	return logger.ContextWithLogger(ctx, log)
}

// WithTestConfig returns a copy of ctx containing a configuration manager loaded with defaults suitable for tests.
func WithTestConfig(tb testing.TB, ctx context.Context) context.Context {
	tb.Helper()
	manager := config.NewManager(ctx, config.NewService())
	if _, err := manager.Load(ctx, config.NewDefaultProvider()); err != nil {
		tb.Fatalf("failed to load test configuration: %v", err)
	}
	cleanup, ok := tb.(interface{ Cleanup(func()) })
	if !ok {
		tb.Fatalf("testing object does not support Cleanup")
	}
	cleanup.Cleanup(func() {
		_ = manager.Close(context.WithoutCancel(ctx))
	})
	return config.ContextWithManager(ctx, manager)
}

// NewBenchmarkContext returns a context derived from b.Context() with logger and configuration attached.
func NewBenchmarkContext(b *testing.B) context.Context {
	b.Helper()
	return NewTestContext(b)
}

func contextFromTB(tb testing.TB) context.Context {
	provider, ok := tb.(interface{ Context() context.Context })
	if !ok {
		tb.Fatalf("testing object does not expose Context()")
	}
	return provider.Context()
}
