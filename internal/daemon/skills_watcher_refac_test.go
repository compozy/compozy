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
}
