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
		t.Cleanup(func() {
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Errorf("Context() before cleanup = %v, want context.Canceled", ctx.Err())
			}
		})
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

	t.Run("Should detect a port already reachable through localhost", func(t *testing.T) {
		t.Parallel()

		listener, err := (&net.ListenConfig{}).Listen(Context(t), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Listen(localhost) error = %v", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		reachable, err := tcpPortReachable(Context(t), port)
		if err != nil {
			t.Fatalf("tcpPortReachable(%d) error = %v", port, err)
		}
		if !reachable {
			t.Fatalf("tcpPortReachable(%d) = false, want true", port)
		}
		if err := listener.Close(); err != nil {
			t.Fatalf("listener.Close() error = %v", err)
		}
	})

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
					t.Context(),
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

// recordingTB captures wait failures so the timeout paths can be asserted
// without failing the enclosing test.
type recordingTB struct {
	testing.TB
	failures []string
}

func (*recordingTB) Helper() {}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func TestScaled(t *testing.T) {
	t.Parallel()

	t.Run("Should multiply the budget by the race-aware time scale", func(t *testing.T) {
		t.Parallel()

		budget := 2 * time.Second
		if got, want := Scaled(&recordingTB{}, budget), budget*time.Duration(TimeScale()); got != want {
			t.Fatalf("Scaled(%v) = %v, want %v", budget, got, want)
		}
	})
}

func TestEventually(t *testing.T) {
	t.Parallel()

	t.Run("Should return once the condition holds", func(t *testing.T) {
		t.Parallel()

		var calls int
		tb := &recordingTB{}
		Eventually(tb, "third poll", time.Minute, func() bool {
			calls++
			return calls == 3
		})
		if len(tb.failures) != 0 || calls != 3 {
			t.Fatalf("Eventually() failures = %v calls = %d, want none after 3 calls", tb.failures, calls)
		}
	})

	t.Run("Should fail with the label once the budget elapses", func(t *testing.T) {
		t.Parallel()

		tb := &recordingTB{}
		Eventually(tb, "never", 20*time.Millisecond, func() bool { return false })
		if want := []string{"timed out waiting for never"}; !EqualStringSlices(tb.failures, want) {
			t.Fatalf("Eventually() failures = %v, want %v", tb.failures, want)
		}
	})
}

func TestReceive(t *testing.T) {
	t.Parallel()

	t.Run("Should return the delivered value", func(t *testing.T) {
		t.Parallel()

		ch := make(chan int, 1)
		ch <- 7
		tb := &recordingTB{}
		if got := Receive(tb, "value", time.Minute, ch); got != 7 || len(tb.failures) != 0 {
			t.Fatalf("Receive() = %d failures = %v, want 7 and none", got, tb.failures)
		}
	})

	t.Run("Should observe a closed done channel", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		close(done)
		tb := &recordingTB{}
		Receive(tb, "done", time.Minute, done)
		if len(tb.failures) != 0 {
			t.Fatalf("Receive(closed) failures = %v, want none", tb.failures)
		}
	})

	t.Run("Should fail with the label when nothing arrives", func(t *testing.T) {
		t.Parallel()

		tb := &recordingTB{}
		Receive(tb, "silence", 20*time.Millisecond, make(chan int))
		if want := []string{"timed out waiting for silence"}; !EqualStringSlices(tb.failures, want) {
			t.Fatalf("Receive() failures = %v, want %v", tb.failures, want)
		}
	})
}
