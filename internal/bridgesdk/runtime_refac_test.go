package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestRuntimeRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should not hold runtime lock while initialize callback runs", func(t *testing.T) {
		t.Parallel()

		var runtime *Runtime
		var err error
		runtime, err = NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{
				Name:    "telegram-adapter",
				Version: "1.0.0",
			},
			Check: testCheckHandler,
			Initialize: func(context.Context, *Session) error {
				if got := runtime.Session(); got != nil {
					return errors.New("runtime session visible before initialize commit")
				}
				return nil
			},
			Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
				return session.AckDelivery(request, "remote-1", "")
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}
		runtime.peer = NewPeer(io.Reader(nil), io.Discard)

		raw, err := json.Marshal(testInitializeRequest())
		if err != nil {
			t.Fatalf("json.Marshal(initialize request) error = %v", err)
		}

		done := make(chan error, 1)
		go func() {
			_, handleErr := runtime.handleInitialize(t.Context(), raw)
			done <- handleErr
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("handleInitialize() error = %v", err)
			}
		case <-time.After(runtimeRefacTimeout(t)):
			t.Fatal("handleInitialize() timed out, likely held runtime lock across callback")
		}
		if runtime.Session() == nil {
			t.Fatal("runtime.Session() after initialize = nil, want committed session")
		}
	})

	t.Run("Should retry shutdown handler after a failed first attempt", func(t *testing.T) {
		t.Parallel()

		shutdownCalls := 0
		shutdownFailure := errors.New("provider shutdown failed")
		runtime, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{
				Name:    "telegram-adapter",
				Version: "1.0.0",
			},
			Check: testCheckHandler,
			Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
				return session.AckDelivery(request, "remote-1", "")
			},
			Shutdown: func(context.Context, *Session, subprocess.ShutdownRequest) error {
				shutdownCalls++
				if shutdownCalls == 1 {
					return shutdownFailure
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}
		runtime.session = &Session{}

		_, err = runtime.handleShutdown(t.Context(), json.RawMessage(`{"reason":"test"}`))
		if !errors.Is(err, shutdownFailure) {
			t.Fatalf("first handleShutdown() error = %v, want %v", err, shutdownFailure)
		}

		response, err := runtime.handleShutdown(t.Context(), json.RawMessage(`{"reason":"retry"}`))
		if err != nil {
			t.Fatalf("second handleShutdown() error = %v", err)
		}
		if got, want := shutdownCalls, 2; got != want {
			t.Fatalf("shutdownCalls = %d, want %d", got, want)
		}
		shutdown, ok := response.(subprocess.ShutdownResponse)
		if !ok {
			t.Fatalf("response = %T, want subprocess.ShutdownResponse", response)
		}
		if !shutdown.Acknowledged {
			t.Fatal("shutdown.Acknowledged = false, want true")
		}
	})

	t.Run(
		"Should reject concurrent shutdown while one call is running and stay idempotent after success",
		func(t *testing.T) {
			t.Parallel()

			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() {
				releaseOnce.Do(func() {
					close(release)
				})
			})

			shutdownCalls := 0
			runtime, err := NewRuntime(RuntimeConfig{
				ExtensionInfo: subprocess.InitializeExtensionInfo{
					Name:    "telegram-adapter",
					Version: "1.0.0",
				},
				Check: testCheckHandler,
				Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
					return session.AckDelivery(request, "remote-1", "")
				},
				Shutdown: func(ctx context.Context, _ *Session, _ subprocess.ShutdownRequest) error {
					shutdownCalls++
					close(entered)
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				},
			})
			if err != nil {
				t.Fatalf("NewRuntime() error = %v", err)
			}
			runtime.session = &Session{}

			firstDone := make(chan error, 1)
			go func() {
				_, handleErr := runtime.handleShutdown(t.Context(), json.RawMessage(`{"reason":"first"}`))
				firstDone <- handleErr
			}()

			select {
			case <-entered:
			case <-time.After(runtimeRefacTimeout(t)):
				t.Fatal("first handleShutdown() did not enter Shutdown callback")
			}

			_, err = runtime.handleShutdown(t.Context(), json.RawMessage(`{"reason":"second"}`))
			var rpcErr *subprocess.RPCError
			if !errors.As(err, &rpcErr) {
				t.Fatalf("second handleShutdown() error = %v, want *subprocess.RPCError", err)
			}
			if rpcErr.Code != bridgeSDKRPCCodeShutdownRunning {
				t.Fatalf("second handleShutdown() code = %d, want %d", rpcErr.Code, bridgeSDKRPCCodeShutdownRunning)
			}
			if rpcErr.Message != "Shutdown running" {
				t.Fatalf("second handleShutdown() message = %q, want %q", rpcErr.Message, "Shutdown running")
			}
			if got, want := shutdownCalls, 1; got != want {
				t.Fatalf("shutdownCalls during concurrent shutdown = %d, want %d", got, want)
			}
			_, err = runtime.handleHealthCheck(t.Context(), nil)
			if !errors.As(err, &rpcErr) {
				t.Fatalf("handleHealthCheck() during shutdown error = %v, want *subprocess.RPCError", err)
			}
			if rpcErr.Code != bridgeSDKRPCCodeShutdownRunning {
				t.Fatalf(
					"handleHealthCheck() during shutdown code = %d, want %d",
					rpcErr.Code,
					bridgeSDKRPCCodeShutdownRunning,
				)
			}

			releaseOnce.Do(func() {
				close(release)
			})

			select {
			case err := <-firstDone:
				if err != nil {
					t.Fatalf("first handleShutdown() error = %v", err)
				}
			case <-time.After(runtimeRefacTimeout(t)):
				t.Fatal("first handleShutdown() did not finish after release")
			}

			response, err := runtime.handleShutdown(t.Context(), json.RawMessage(`{"reason":"third"}`))
			if err != nil {
				t.Fatalf("third handleShutdown() error = %v", err)
			}
			if got, want := shutdownCalls, 1; got != want {
				t.Fatalf("shutdownCalls after successful shutdown = %d, want %d", got, want)
			}
			shutdown, ok := response.(subprocess.ShutdownResponse)
			if !ok {
				t.Fatalf("response = %T, want subprocess.ShutdownResponse", response)
			}
			if !shutdown.Acknowledged {
				t.Fatal("shutdown.Acknowledged = false, want true")
			}
		},
	)

	t.Run("Should not publish a session after initialize context cancellation", func(t *testing.T) {
		t.Parallel()

		var cancel context.CancelFunc
		runtime, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{
				Name:    "telegram-adapter",
				Version: "1.0.0",
			},
			Check: testCheckHandler,
			Initialize: func(context.Context, *Session) error {
				cancel()
				return nil
			},
			Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
				return session.AckDelivery(request, "remote-1", "")
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}
		runtime.peer = NewPeer(io.Reader(nil), io.Discard)

		raw, err := json.Marshal(testInitializeRequest())
		if err != nil {
			t.Fatalf("json.Marshal(initialize request) error = %v", err)
		}

		ctx, runtimeCancel := context.WithCancel(t.Context())
		cancel = runtimeCancel
		t.Cleanup(cancel)

		_, err = runtime.handleInitialize(ctx, raw)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handleInitialize() error = %v, want %v", err, context.Canceled)
		}
		if runtime.Session() != nil {
			t.Fatal("runtime.Session() after canceled initialize != nil, want nil")
		}
	})

	t.Run("Should decode whitespace null params as empty object", func(t *testing.T) {
		t.Parallel()

		var target map[string]any
		if err := decodeParams(json.RawMessage(" \n null \t "), &target); err != nil {
			t.Fatalf("decodeParams(whitespace null) error = %v", err)
		}
		if target == nil {
			t.Fatal("target = nil, want decoded empty object")
		}
		if got := len(target); got != 0 {
			t.Fatalf("len(target) = %d, want empty decoded object", got)
		}
	})
}

func runtimeRefacTimeout(t *testing.T) time.Duration {
	t.Helper()

	const (
		fallback = 2 * time.Second
		minimum  = time.Second
		maximum  = 5 * time.Second
	)

	deadline, ok := t.Deadline()
	if !ok {
		return fallback
	}

	timeout := time.Until(deadline) / 10
	if timeout < minimum {
		return minimum
	}
	if timeout > maximum {
		return maximum
	}
	return timeout
}
