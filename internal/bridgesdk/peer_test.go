package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/subprocess"
)

func TestPeerCallDispatchesRequestAndResponse(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch a request and its typed response", func(t *testing.T) {
		t.Parallel()

		leftConn, rightConn := net.Pipe()
		left := mustNewPeer(t, leftConn, leftConn)
		right := mustNewPeer(t, rightConn, rightConn)

		if err := right.Handle("echo", func(_ context.Context, raw json.RawMessage) (any, error) {
			var params map[string]string
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, err
			}
			return map[string]string{"message": params["message"]}, nil
		}); err != nil {
			t.Fatalf("right.Handle() error = %v", err)
		}

		ctx := startPeerPair(t, left, right)
		var result map[string]string
		if err := left.Call(ctx, "echo", map[string]string{"message": "hello"}, &result); err != nil {
			t.Fatalf("left.Call() error = %v", err)
		}
		if got, want := result["message"], "hello"; got != want {
			t.Fatalf("result[message] = %q, want %q", got, want)
		}
	})
}

func TestPeerCallReturnsRPCError(t *testing.T) {
	t.Parallel()

	t.Run("Should return the remote structured RPC error", func(t *testing.T) {
		t.Parallel()

		leftConn, rightConn := net.Pipe()
		left := mustNewPeer(t, leftConn, leftConn)
		right := mustNewPeer(t, rightConn, rightConn)

		if err := right.Handle("fail", func(context.Context, json.RawMessage) (any, error) {
			return nil, subprocess.NewRPCError(99, "boom", nil)
		}); err != nil {
			t.Fatalf("right.Handle() error = %v", err)
		}

		ctx := startPeerPair(t, left, right)
		err := left.Call(ctx, "fail", nil, nil)

		rpcErr, rpcErrMatched := errors.AsType[*subprocess.RPCError](err)
		if !rpcErrMatched {
			t.Fatalf("left.Call() error = %T, want *subprocess.RPCError", err)
		}
		if got, want := rpcErr.Code, 99; got != want {
			t.Fatalf("rpcErr.Code = %d, want %d", got, want)
		}
	})
}

func TestPeerHandleRejectsInvalidRegistration(t *testing.T) {
	t.Parallel()

	t.Run("Should reject empty methods and nil handlers", func(t *testing.T) {
		t.Parallel()

		peer := mustNewPeer(t, io.NopCloser(strings.NewReader("")), io.Discard)
		if err := peer.Handle("", func(context.Context, json.RawMessage) (any, error) { return nil, nil }); err == nil {
			t.Fatal("Handle(empty method) error = nil, want non-nil")
		}
		if err := peer.Handle("ok", nil); err == nil {
			t.Fatal("Handle(nil handler) error = nil, want non-nil")
		}
	})
}

func TestPeerCallReturnsContextErrorWhenResponseDoesNotArrive(t *testing.T) {
	t.Parallel()

	t.Run("Should return call cancellation while the remote handler is still running", func(t *testing.T) {
		t.Parallel()

		leftConn, rightConn := net.Pipe()
		left := mustNewPeer(t, leftConn, leftConn)
		right := mustNewPeer(t, rightConn, rightConn)
		handlerStarted := make(chan struct{})
		handlerRelease := make(chan struct{})
		var releaseOnce sync.Once
		releaseHandler := func() {
			releaseOnce.Do(func() { close(handlerRelease) })
		}

		if err := right.Handle("slow", func(context.Context, json.RawMessage) (any, error) {
			close(handlerStarted)
			<-handlerRelease
			return map[string]bool{"ok": true}, nil
		}); err != nil {
			t.Fatalf("right.Handle() error = %v", err)
		}

		parentCtx := startPeerPair(t, left, right)
		t.Cleanup(releaseHandler)
		ctx, callCancel := context.WithCancel(parentCtx)
		t.Cleanup(callCancel)
		callDone := make(chan error, 1)
		go func() {
			callDone <- left.Call(ctx, "slow", nil, nil)
		}()

		<-handlerStarted
		callCancel()
		err := <-callDone
		releaseHandler()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("left.Call() error = %v, want context canceled", err)
		}
	})
}

func TestPeerServeReturnsDecodeErrorForMalformedFrame(t *testing.T) {
	t.Parallel()

	t.Run("Should return a decode error for a malformed frame", func(t *testing.T) {
		t.Parallel()

		peer := mustNewPeer(t, io.NopCloser(strings.NewReader("not-json\n")), io.Discard)
		err := peer.Serve(t.Context())
		if err == nil {
			t.Fatal("Serve(malformed frame) error = nil, want non-nil")
		}
	})
}

func TestPeerCallReturnsErrorForUnmarshalableParams(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unmarshalable params without publishing a pending call", func(t *testing.T) {
		t.Parallel()

		peer := mustNewPeer(t, io.NopCloser(strings.NewReader("")), io.Discard)
		type badParams struct {
			Callback func()
		}

		var (
			err      error
			panicked any
		)
		func() {
			defer func() {
				panicked = recover()
			}()
			err = peer.Call(t.Context(), "bad", badParams{
				Callback: func() {},
			}, nil)
		}()

		if panicked != nil {
			t.Fatalf("Call() panicked: %v", panicked)
		}
		if err == nil {
			t.Fatal("Call() error = nil, want non-nil")
		}
		if len(peer.pending) != 0 {
			t.Fatalf("len(peer.pending) = %d, want 0", len(peer.pending))
		}
	})
}

func TestPeerLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid IO without panicking", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			run  func() error
		}{
			{
				name: "Should reject nil input",
				run: func() error {
					_, err := NewPeer(nil, io.Discard)
					return err
				},
			},
			{
				name: "Should reject input without cancellation ownership",
				run: func() error {
					_, err := NewPeer(strings.NewReader(""), io.Discard)
					return err
				},
			},
			{
				name: "Should reject typed nil input",
				run: func() error {
					var input *peerBlockingReadCloser
					_, err := NewPeer(input, io.Discard)
					return err
				},
			},
			{
				name: "Should reject nil output",
				run: func() error {
					_, err := NewPeer(io.NopCloser(strings.NewReader("")), nil)
					return err
				},
			},
			{
				name: "Should reject typed nil output",
				run: func() error {
					var output *strings.Builder
					_, err := NewPeer(io.NopCloser(strings.NewReader("")), output)
					return err
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				var (
					err      error
					panicked any
				)
				func() {
					defer func() {
						panicked = recover()
					}()
					err = test.run()
				}()

				if panicked != nil {
					t.Fatalf("invalid peer I/O panicked: %v", panicked)
				}
				if err == nil {
					t.Fatal("invalid peer I/O error = nil, want non-nil")
				}
			})
		}
	})

	t.Run("Should close the owned input and join Serve when canceled", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			reader := newPeerBlockingReadCloser()
			peer := mustNewPeer(t, reader, io.Discard)
			ctx, cancel := context.WithCancel(t.Context())
			serveDone := make(chan error, 1)
			go func() {
				serveDone <- peer.Serve(ctx)
			}()

			<-reader.readStarted
			synctest.Wait()
			cancel()
			synctest.Wait()

			closedByPeer := channelClosed(reader.closeCalled)
			if !closedByPeer {
				if err := reader.Close(); err != nil {
					t.Fatalf("reader.Close() cleanup error = %v", err)
				}
				synctest.Wait()
			}
			serveErr := <-serveDone

			if !closedByPeer {
				t.Error("Serve cancellation did not close its owned input")
			}
			if !errors.Is(serveErr, context.Canceled) {
				t.Errorf("Serve() error = %v, want context canceled", serveErr)
			}
			if got, want := reader.closeCalls.Load(), int64(1); got != want {
				t.Errorf("input Close() calls = %d, want %d", got, want)
			}
		})
	})

	t.Run("Should reject a second Serve owner", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			reader := newPeerBlockingReadCloser()
			peer := mustNewPeer(t, reader, io.Discard)
			ctx, cancel := context.WithCancel(t.Context())
			serveDone := make(chan error, 1)
			go func() {
				serveDone <- peer.Serve(ctx)
			}()

			<-reader.readStarted
			if err := peer.Serve(t.Context()); !errors.Is(err, errPeerAlreadyServing) {
				t.Errorf("second Serve() error = %v, want %v", err, errPeerAlreadyServing)
			}

			cancel()
			synctest.Wait()
			if err := <-serveDone; !errors.Is(err, context.Canceled) {
				t.Errorf("first Serve() error = %v, want context canceled", err)
			}
		})
	})

	t.Run("Should publish one terminal cause to admitted and subsequent calls", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			readErr := errors.New("peer read sentinel")
			closeErr := errors.New("peer close sentinel")
			reader := newPeerTerminalReadCloser(readErr, closeErr)
			writer := newPeerSignalWriter()
			peer := mustNewPeer(t, reader, writer)
			serveDone := make(chan error, 1)
			go func() {
				serveDone <- peer.Serve(t.Context())
			}()
			<-reader.readStarted

			admittedCallDone := make(chan error, 1)
			go func() {
				admittedCallDone <- peer.Call(t.Context(), "during-serve", nil, nil)
			}()
			<-writer.writeCalled
			reader.release()
			synctest.Wait()

			serveErr := <-serveDone
			admittedCallErr := <-admittedCallDone
			subsequentCallErr := peer.Call(t.Context(), "after-terminal", nil, nil)

			if !errors.Is(serveErr, readErr) || !errors.Is(serveErr, closeErr) {
				t.Errorf("Serve() error = %v, want read and close sentinels", serveErr)
			}
			if !errors.Is(admittedCallErr, readErr) || !errors.Is(admittedCallErr, closeErr) {
				t.Errorf("admitted Call() error = %v, want read and close sentinels", admittedCallErr)
			}
			if !errors.Is(subsequentCallErr, readErr) || !errors.Is(subsequentCallErr, closeErr) {
				t.Errorf("subsequent Call() error = %v, want read and close sentinels", subsequentCallErr)
			}
			if got, want := reader.closeCalls.Load(), int64(1); got != want {
				t.Errorf("input Close() calls = %d, want %d", got, want)
			}
		})
	})

	t.Run("Should make an outbound write failure terminal", func(t *testing.T) {
		t.Parallel()

		writeErr := errors.New("peer write sentinel")
		closeErr := errors.New("peer close after write sentinel")
		reader := newPeerTerminalReadCloser(io.EOF, closeErr)
		writer := &peerErrorWriter{err: writeErr}
		peer := mustNewPeer(t, reader, writer)

		firstCallErr := peer.Call(t.Context(), "first", nil, nil)
		subsequentCallErr := peer.Call(t.Context(), "subsequent", nil, nil)

		if !errors.Is(firstCallErr, writeErr) || !errors.Is(firstCallErr, closeErr) {
			t.Errorf("first Call() error = %v, want write and close sentinels", firstCallErr)
		}
		if !errors.Is(subsequentCallErr, writeErr) || !errors.Is(subsequentCallErr, closeErr) {
			t.Errorf("subsequent Call() error = %v, want write and close sentinels", subsequentCallErr)
		}
		if got, want := writer.calls.Load(), int64(1); got != want {
			t.Errorf("output Write() calls = %d, want %d", got, want)
		}
		if got, want := reader.closeCalls.Load(), int64(1); got != want {
			t.Errorf("input Close() calls = %d, want %d", got, want)
		}
	})

	t.Run("Should join outbound write terminalization before Serve returns", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			writeErr := errors.New("concurrent peer write sentinel")
			reader := newPeerBlockingReadCloser()
			writer := newPeerBlockingErrorWriter(writeErr)
			peer := mustNewPeer(t, reader, writer)
			serveDone := make(chan error, 1)
			go func() {
				serveDone <- peer.Serve(t.Context())
			}()
			<-reader.readStarted

			callDone := make(chan error, 1)
			go func() {
				callDone <- peer.Call(t.Context(), "write-failure", nil, nil)
			}()
			<-writer.writeStarted
			peer.stateMu.Lock()
			pendingCount := len(peer.pending)
			var pendingCall chan rpcResult
			for _, responseCh := range peer.pending {
				pendingCall = responseCh
			}
			peer.stateMu.Unlock()
			if pendingCount != 1 || pendingCall == nil {
				t.Fatalf("pending outbound Calls = %d, want exactly one admitted Call", pendingCount)
			}
			writer.release()

			serveErr := <-serveDone
			select {
			case _, open := <-pendingCall:
				if open {
					t.Fatal("pending Call received an unexpected response")
				}
			default:
				t.Fatal("Serve() returned before the pending Call was unblocked")
			}
			callErr := <-callDone
			if !errors.Is(serveErr, writeErr) {
				t.Errorf("Serve() error = %v, want write sentinel", serveErr)
			}
			if !errors.Is(callErr, writeErr) {
				t.Errorf("Call() error = %v, want write sentinel", callErr)
			}
			if got, want := reader.closeCalls.Load(), int64(1); got != want {
				t.Errorf("input Close() calls = %d, want %d", got, want)
			}
		})
	})
}

const peerPanicHelperEnv = "COMPOZY_TEST_BRIDGESDK_PEER_PANIC_HELPER"

func TestPeerContainsHandlerPanicAtRPCBoundary(t *testing.T) {
	if os.Getenv(peerPanicHelperEnv) == "1" {
		runPeerPanicHelper(t)
		return
	}
	t.Parallel()

	t.Run("Should contain a handler panic and keep serving later requests", func(t *testing.T) {
		t.Parallel()

		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPeerContainsHandlerPanicAtRPCBoundary$")
		command.Env = append(os.Environ(), peerPanicHelperEnv+"=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("panic-boundary helper error = %v\n%s", err, output)
		}
	})
}

func runPeerPanicHelper(t *testing.T) {
	t.Helper()

	leftConn, rightConn := net.Pipe()
	t.Cleanup(func() {
		if err := leftConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("leftConn.Close() error = %v", err)
		}
		if err := rightConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("rightConn.Close() error = %v", err)
		}
	})

	left := mustNewPeer(t, leftConn, leftConn)
	right := mustNewPeer(t, rightConn, rightConn)
	if err := right.Handle("panic", func(context.Context, json.RawMessage) (any, error) {
		panic("provider handler sentinel")
	}); err != nil {
		t.Fatalf("right.Handle(panic) error = %v", err)
	}
	if err := right.Handle("echo", func(context.Context, json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatalf("right.Handle(echo) error = %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	leftDone := make(chan error, 1)
	rightDone := make(chan error, 1)
	go func() {
		leftDone <- left.Serve(ctx)
	}()
	go func() {
		rightDone <- right.Serve(ctx)
	}()

	err := left.Call(ctx, "panic", nil, nil)
	rpcErr, ok := errors.AsType[*subprocess.RPCError](err)
	if !ok {
		t.Fatalf("left.Call(panic) error = %T %v, want *subprocess.RPCError", err, err)
	}
	if got, want := rpcErr.Code, bridgeSDKRPCCodeInternal; got != want {
		t.Fatalf("panic RPC code = %d, want %d", got, want)
	}
	if got, want := rpcErr.Message, "Internal error"; got != want {
		t.Fatalf("panic RPC message = %q, want %q", got, want)
	}
	if strings.Contains(string(rpcErr.Data), "provider handler sentinel") {
		t.Fatalf("panic RPC data exposed handler panic value: %s", rpcErr.Data)
	}

	var echoed map[string]bool
	if err := left.Call(ctx, "echo", nil, &echoed); err != nil {
		t.Fatalf("left.Call(echo) error = %v", err)
	}
	if !echoed["ok"] {
		t.Fatal("left.Call(echo) result ok = false, want true")
	}

	cancel()
	assertPeerServeJoined(t, "left", <-leftDone)
	assertPeerServeJoined(t, "right", <-rightDone)
}

type peerBlockingReadCloser struct {
	readStarted chan struct{}
	closeCalled chan struct{}
	released    chan struct{}
	readOnce    sync.Once
	closeOnce   sync.Once
	closeCalls  atomic.Int64
}

func newPeerBlockingReadCloser() *peerBlockingReadCloser {
	return &peerBlockingReadCloser{
		readStarted: make(chan struct{}),
		closeCalled: make(chan struct{}),
		released:    make(chan struct{}),
	}
}

func (r *peerBlockingReadCloser) Read([]byte) (int, error) {
	r.readOnce.Do(func() { close(r.readStarted) })
	<-r.released
	return 0, io.ErrClosedPipe
}

func (r *peerBlockingReadCloser) Close() error {
	r.closeCalls.Add(1)
	r.closeOnce.Do(func() {
		close(r.closeCalled)
		close(r.released)
	})
	return nil
}

type peerTerminalReadCloser struct {
	readStarted chan struct{}
	releaseRead chan struct{}
	readErr     error
	closeErr    error
	readOnce    sync.Once
	releaseOnce sync.Once
	closeCalls  atomic.Int64
}

func newPeerTerminalReadCloser(readErr, closeErr error) *peerTerminalReadCloser {
	return &peerTerminalReadCloser{
		readStarted: make(chan struct{}),
		releaseRead: make(chan struct{}),
		readErr:     readErr,
		closeErr:    closeErr,
	}
}

func (r *peerTerminalReadCloser) Read([]byte) (int, error) {
	r.readOnce.Do(func() { close(r.readStarted) })
	<-r.releaseRead
	return 0, r.readErr
}

func (r *peerTerminalReadCloser) Close() error {
	r.closeCalls.Add(1)
	r.release()
	return r.closeErr
}

func (r *peerTerminalReadCloser) release() {
	r.releaseOnce.Do(func() { close(r.releaseRead) })
}

type peerSignalWriter struct {
	writeCalled chan struct{}
	writeOnce   sync.Once
}

func newPeerSignalWriter() *peerSignalWriter {
	return &peerSignalWriter{writeCalled: make(chan struct{})}
}

func (w *peerSignalWriter) Write(payload []byte) (int, error) {
	w.writeOnce.Do(func() { close(w.writeCalled) })
	return len(payload), nil
}

type peerErrorWriter struct {
	err   error
	calls atomic.Int64
}

func (w *peerErrorWriter) Write([]byte) (int, error) {
	w.calls.Add(1)
	return 0, w.err
}

type peerBlockingErrorWriter struct {
	err          error
	writeStarted chan struct{}
	releaseWrite chan struct{}
	startOnce    sync.Once
	releaseOnce  sync.Once
}

func newPeerBlockingErrorWriter(err error) *peerBlockingErrorWriter {
	return &peerBlockingErrorWriter{
		err:          err,
		writeStarted: make(chan struct{}),
		releaseWrite: make(chan struct{}),
	}
}

func (w *peerBlockingErrorWriter) Write([]byte) (int, error) {
	w.startOnce.Do(func() { close(w.writeStarted) })
	<-w.releaseWrite
	return 0, w.err
}

func (w *peerBlockingErrorWriter) release() {
	w.releaseOnce.Do(func() { close(w.releaseWrite) })
}

func channelClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

type peerTestFataler interface {
	Helper()
	Fatalf(format string, args ...any)
}

func mustNewPeer(t peerTestFataler, stdin io.Reader, stdout io.Writer) *Peer {
	t.Helper()
	peer, err := NewPeer(stdin, stdout)
	if err != nil {
		t.Fatalf("NewPeer() error = %v", err)
	}
	return peer
}

func startPeerPair(t *testing.T, left, right *Peer) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	leftDone := make(chan error, 1)
	rightDone := make(chan error, 1)
	go func() {
		leftDone <- left.Serve(ctx)
	}()
	go func() {
		rightDone <- right.Serve(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		assertPeerServeJoined(t, "left", <-leftDone)
		assertPeerServeJoined(t, "right", <-rightDone)
	})
	return ctx
}

func assertPeerServeJoined(t *testing.T, name string, err error) {
	t.Helper()
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("%s Serve() cleanup error = %v", name, err)
	}
}
