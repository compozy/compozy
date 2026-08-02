package compozysdk

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadyCallbackLifecycleDrain(t *testing.T) {
	t.Parallel()

	t.Run("Should exclude callback errors accepted after the deadline decision", func(t *testing.T) {
		t.Parallel()

		lifecycle := newReadyCallbackLifecycle()
		started := make(chan struct{})
		release := make(chan struct{})
		released := false
		t.Cleanup(func() {
			if !released {
				close(release)
			}
		})
		lateErr := errors.New("late callback failure")
		lifecycle.start(func(context.Context) error {
			close(started)
			<-release
			return lateErr
		})
		waitForReadyLifecycleSignal(t, started, "ready callback did not start")

		lifecycle.stop()
		deadlineCtx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
		defer cancel()
		result := lifecycle.drain(deadlineCtx)
		if !errors.Is(result.waitErr, context.DeadlineExceeded) {
			t.Fatalf("drain wait error = %v, want context.DeadlineExceeded", result.waitErr)
		}
		if result.callbackErr != nil {
			t.Fatalf("drain callback error = %v, want nil before late completion", result.callbackErr)
		}

		close(release)
		released = true
		waitForReadyLifecycleSignal(t, lifecycle.done, "ready callback did not complete")
		if result.callbackErr != nil {
			t.Fatalf("drain callback error changed to %v after decision", result.callbackErr)
		}
	})

	t.Run("Should prefer observable completion when deadline and done are ready", func(t *testing.T) {
		t.Parallel()

		lifecycle := newReadyCallbackLifecycle()
		callbackErr := errors.New("callback failure")
		lifecycle.start(func(context.Context) error {
			return callbackErr
		})
		lifecycle.stop()
		waitForReadyLifecycleSignal(t, lifecycle.done, "ready callback did not complete")

		deadlineCtx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
		defer cancel()
		result := lifecycle.drain(deadlineCtx)
		if result.waitErr != nil {
			t.Fatalf("drain wait error = %v, want completed lifecycle", result.waitErr)
		}
		if !errors.Is(result.callbackErr, callbackErr) {
			t.Fatalf("drain callback error = %v, want %v", result.callbackErr, callbackErr)
		}
	})
}

func waitForReadyLifecycleSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal(failure)
	}
}
