package remotetest

import (
	"context"
	"time"
)

// Fixture is the lifetime and failure reporter shared by Go tests and the harness executable.
type Fixture interface {
	Context() context.Context
	Helper()
	Cleanup(func())
	TempDir() string
	Failed() bool
	Logf(string, ...any)
	Errorf(string, ...any)
	Fatal(...any)
	Fatalf(string, ...any)
}

func Await(t Fixture, label string, budget time.Duration, check func(context.Context) bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), budget)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for !check(ctx) {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s: %v", label, ctx.Err())
		case <-ticker.C:
		}
	}
}
