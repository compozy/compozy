package testutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestContextIsCanceledOnCleanup(t *testing.T) {
	t.Parallel()

	var ctx context.Context
	done := make(chan struct{})

	t.Run("Should cancel the context during cleanup", func(t *testing.T) {
		ctx = Context(t)
		go func() {
			<-ctx.Done()
			close(done)
		}()
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Context() was not canceled after cleanup")
	}

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("Context() err = %v, want context.Canceled", ctx.Err())
	}
}

func TestContextCreatedDuringCleanupRemainsUsable(t *testing.T) {
	t.Parallel()

	errCh := make(chan error, 1)

	t.Run("Should create a usable context during cleanup", func(t *testing.T) {
		t.Cleanup(func() {
			ctx := Context(t)
			if err := ctx.Err(); err != nil {
				errCh <- fmt.Errorf("Context() err during cleanup = %v, want nil", err)
				return
			}
			errCh <- nil
		})
	})

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup callback did not report result")
	}
}

func TestEqualStringSlices(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		left  []string
		right []string
		want  bool
	}{
		{name: "Should treat nil slices as equal", want: true},
		{name: "Should treat equal values as equal", left: []string{"a", "b"}, right: []string{"a", "b"}, want: true},
		{name: "Should reject different lengths", left: []string{"a"}, right: []string{"a", "b"}, want: false},
		{name: "Should reject different values", left: []string{"a", "b"}, right: []string{"a", "c"}, want: false},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := EqualStringSlices(tt.left, tt.right); got != tt.want {
				t.Fatalf("EqualStringSlices(%v, %v) = %v, want %v", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestFreeTCPPort(t *testing.T) {
	t.Parallel()

	t.Run("Should return a bindable positive port", func(t *testing.T) {
		t.Parallel()

		port := FreeTCPPort(t)
		if port <= 0 {
			t.Fatalf("FreeTCPPort() = %d, want positive port", port)
		}

		ln, err := (&net.ListenConfig{}).Listen(Context(t), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			t.Fatalf("Listen(reused port %d) error = %v", port, err)
		}
		if err := ln.Close(); err != nil {
			t.Fatalf("ln.Close() error = %v", err)
		}
	})

	t.Run("Should avoid collisions while returned ports are rebound concurrently", func(t *testing.T) {
		t.Parallel()

		const workers = 32
		type allocation struct {
			port     int
			listener net.Listener
			err      error
		}

		start := make(chan struct{})
		results := make(chan allocation, workers)
		var ready sync.WaitGroup
		ready.Add(workers)

		for range workers {
			go func() {
				ready.Done()
				<-start

				port := FreeTCPPort(t)
				listener, err := (&net.ListenConfig{}).Listen(
					context.Background(),
					"tcp",
					fmt.Sprintf("127.0.0.1:%d", port),
				)
				results <- allocation{port: port, listener: listener, err: err}
			}()
		}

		ready.Wait()
		close(start)

		listeners := make([]net.Listener, 0, workers)
		ports := make(map[int]struct{}, workers)
		for range workers {
			select {
			case result := <-results:
				if result.err != nil {
					t.Fatalf("Listen(rebound port %d) error = %v", result.port, result.err)
				}
				if _, ok := ports[result.port]; ok {
					t.Fatalf("FreeTCPPort() returned duplicate port %d", result.port)
				}
				ports[result.port] = struct{}{}
				listeners = append(listeners, result.listener)
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for concurrent FreeTCPPort allocations")
			}
		}

		for _, listener := range listeners {
			if err := listener.Close(); err != nil {
				t.Fatalf("listener.Close() error = %v", err)
			}
		}
	})
}
