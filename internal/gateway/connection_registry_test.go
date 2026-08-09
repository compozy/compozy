package gateway

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConnectionRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should cancel exactly the target device without leaking concurrent handles (UT-041)", func(t *testing.T) {
		registry := NewConnectionRegistry()
		var targetCanceled atomic.Int64
		var otherCanceled atomic.Int64
		var wait sync.WaitGroup
		for index := range 64 {
			wait.Go(func() {
				deviceID := "dev_target"
				counter := &targetCanceled
				if index%2 == 1 {
					deviceID = "dev_other"
					counter = &otherCanceled
				}
				handle := registry.Register(deviceID, func() { counter.Add(1) })
				if index%4 == 0 {
					registry.Deregister(deviceID, handle)
				}
			})
		}
		wait.Wait()
		canceled, err := registry.CancelDevice(t.Context(), "dev_target")
		if err != nil {
			t.Fatalf("CancelDevice(target) error = %v", err)
		}
		if int(targetCanceled.Load()) != canceled || otherCanceled.Load() != 0 {
			t.Fatalf(
				"CancelDevice(target) = %d, counters target:%d other:%d",
				canceled,
				targetCanceled.Load(),
				otherCanceled.Load(),
			)
		}
		other, err := registry.CancelDevice(t.Context(), "dev_other")
		if err != nil {
			t.Fatalf("CancelDevice(other) error = %v", err)
		}
		remaining, err := registry.CancelDevice(t.Context(), "dev_target")
		if err != nil {
			t.Fatalf("CancelDevice(after teardown) error = %v", err)
		}
		if int(otherCanceled.Load()) != other || remaining != 0 {
			t.Fatalf("remaining cancellation = other:%d counter:%d", other, otherCanceled.Load())
		}
	})

	t.Run("Should wait for registered stream teardown before revoke returns (UT-040)", func(t *testing.T) {
		registry := NewConnectionRegistry()
		streamCtx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		registry.RegisterWaitable("dev_wait", cancel, done)
		go func() {
			<-streamCtx.Done()
			close(done)
		}()
		if canceled, err := registry.CancelDevice(t.Context(), "dev_wait"); err != nil || canceled != 1 {
			t.Fatalf("CancelDevice(waitable) = (%d, %v)", canceled, err)
		}
		select {
		case <-done:
		default:
			t.Fatal("CancelDevice returned before stream teardown")
		}
	})

	t.Run("Should stop waiting when a stream ignores cancellation", func(t *testing.T) {
		t.Parallel()
		registry := NewConnectionRegistry()
		registry.RegisterWaitable("dev_stuck", func() {}, make(chan struct{}))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		canceled, err := registry.CancelDevice(ctx, "dev_stuck")
		if canceled != 1 || !errors.Is(err, context.Canceled) {
			t.Fatalf("CancelDevice(stuck) = (%d, %v), want one cancellation and context error", canceled, err)
		}
	})
}
