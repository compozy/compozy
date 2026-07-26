package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/subprocess"
)

func TestPeerCallDispatchesRequestAndResponse(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	leftConn, rightConn := net.Pipe()
	defer func() {
		_ = leftConn.Close()
	}()
	defer func() {
		_ = rightConn.Close()
	}()

	left := NewPeer(leftConn, leftConn)
	right := NewPeer(rightConn, rightConn)

	if err := right.Handle("echo", func(_ context.Context, raw json.RawMessage) (any, error) {
		var params map[string]string
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		return map[string]string{"message": params["message"]}, nil
	}); err != nil {
		t.Fatalf("right.Handle() error = %v", err)
	}

	go func() { _ = left.Serve(ctx) }()
	go func() { _ = right.Serve(ctx) }()

	var result map[string]string
	if err := left.Call(ctx, "echo", map[string]string{"message": "hello"}, &result); err != nil {
		t.Fatalf("left.Call() error = %v", err)
	}
	if got, want := result["message"], "hello"; got != want {
		t.Fatalf("result[message] = %q, want %q", got, want)
	}
}

func TestPeerCallReturnsRPCError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	leftConn, rightConn := net.Pipe()
	defer func() {
		_ = leftConn.Close()
	}()
	defer func() {
		_ = rightConn.Close()
	}()

	left := NewPeer(leftConn, leftConn)
	right := NewPeer(rightConn, rightConn)

	if err := right.Handle("fail", func(context.Context, json.RawMessage) (any, error) {
		return nil, subprocess.NewRPCError(99, "boom", nil)
	}); err != nil {
		t.Fatalf("right.Handle() error = %v", err)
	}

	go func() { _ = left.Serve(ctx) }()
	go func() { _ = right.Serve(ctx) }()

	err := left.Call(ctx, "fail", nil, nil)
	var rpcErr *subprocess.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("left.Call() error = %T, want *subprocess.RPCError", err)
	}
	if got, want := rpcErr.Code, 99; got != want {
		t.Fatalf("rpcErr.Code = %d, want %d", got, want)
	}
}

func TestPeerHandleRejectsInvalidRegistration(t *testing.T) {
	t.Parallel()

	peer := NewPeer(strings.NewReader(""), io.Discard)
	if err := peer.Handle("", func(context.Context, json.RawMessage) (any, error) { return nil, nil }); err == nil {
		t.Fatal("Handle(empty method) error = nil, want non-nil")
	}
	if err := peer.Handle("ok", nil); err == nil {
		t.Fatal("Handle(nil handler) error = nil, want non-nil")
	}
}

func TestPeerCallReturnsContextErrorWhenResponseDoesNotArrive(t *testing.T) {
	t.Parallel()

	parentCtx := t.Context()

	leftConn, rightConn := net.Pipe()
	defer func() {
		_ = leftConn.Close()
	}()
	defer func() {
		_ = rightConn.Close()
	}()

	left := NewPeer(leftConn, leftConn)
	right := NewPeer(rightConn, rightConn)

	if err := right.Handle("slow", func(context.Context, json.RawMessage) (any, error) {
		time.Sleep(50 * time.Millisecond)
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatalf("right.Handle() error = %v", err)
	}

	go func() { _ = left.Serve(parentCtx) }()
	go func() { _ = right.Serve(parentCtx) }()

	ctx, callCancel := context.WithTimeout(parentCtx, 5*time.Millisecond)
	defer callCancel()

	err := left.Call(ctx, "slow", nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("left.Call() error = %v, want context deadline exceeded", err)
	}
}

func TestPeerServeReturnsDecodeErrorForMalformedFrame(t *testing.T) {
	t.Parallel()

	peer := NewPeer(strings.NewReader("not-json\n"), io.Discard)
	err := peer.Serve(context.Background())
	if err == nil {
		t.Fatal("Serve(malformed frame) error = nil, want non-nil")
	}
}

func TestPeerCallReturnsErrorForUnmarshalableParams(t *testing.T) {
	t.Parallel()

	peer := NewPeer(strings.NewReader(""), io.Discard)

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
}
