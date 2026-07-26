//go:build !windows

package procutil

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchParentExit(t *testing.T) {
	t.Parallel()
	t.Run("Should call onExit once when the parent pid changes", func(t *testing.T) {
		t.Parallel()
		var reads atomic.Int32
		var calls atomic.Int32
		done := make(chan struct{})
		// First read is the "original" parent; every later read reports a
		// reparent to init, which must trigger exactly one onExit.
		getppid := func() int {
			if reads.Add(1) == 1 {
				return 1000
			}
			return 1
		}
		go watchParentExit(context.Background(), time.Millisecond, getppid, func() {
			calls.Add(1)
			close(done)
		})
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("onExit was not called after the parent pid changed")
		}
		if got := calls.Load(); got != 1 {
			t.Fatalf("onExit call count = %d, want 1", got)
		}
	})
	t.Run("Should return without calling onExit when ctx is canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		var calls atomic.Int32
		done := make(chan struct{})
		go func() {
			watchParentExit(ctx, time.Millisecond, func() int { return 42 }, func() {
				calls.Add(1)
			})
			close(done)
		}()
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("watchParentExit did not return after ctx cancel")
		}
		if got := calls.Load(); got != 0 {
			t.Fatalf("onExit call count = %d, want 0", got)
		}
	})
}
