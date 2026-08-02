package daemon

import (
	"context"
	"errors"
	"testing"
)

func TestStopSkillsWatcherRespectsShutdownContext(t *testing.T) {
	t.Parallel()

	t.Run("Should return context error when watcher does not stop", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		cancelCalled := false

		err := stopSkillsWatcher(ctx, func() {
			cancelCalled = true
		}, make(chan struct{}))
		if !cancelCalled {
			t.Fatal("stopSkillsWatcher() did not call cancel")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stopSkillsWatcher() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should return nil after watcher exits", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		close(done)

		if err := stopSkillsWatcher(t.Context(), func() {}, done); err != nil {
			t.Fatalf("stopSkillsWatcher() error = %v, want nil", err)
		}
	})

	t.Run("Should reject a missing wait context after requesting cancellation", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		close(done)
		cancelCalled := false

		err := stopSkillsWatcher(nil, func() { //nolint:staticcheck // Verifies the internal nil-context guard.
			cancelCalled = true
		}, done)
		if !cancelCalled {
			t.Fatal("stopSkillsWatcher() did not call cancel")
		}
		if err == nil {
			t.Fatal("stopSkillsWatcher(nil context) error = nil, want non-nil")
		}
	})
}
