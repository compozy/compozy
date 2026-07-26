package mcp

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPeerInfoContextHelpers(t *testing.T) {
	t.Parallel()

	peer := PeerInfo{Supported: true, PID: 10, UID: 20, GID: 30, ExecutablePath: "/bin/agh"}
	ctx := ContextWithPeerInfo(context.Background(), peer, nil)
	got, err := PeerInfoFromContext(ctx)
	if err != nil {
		t.Fatalf("PeerInfoFromContext() error = %v", err)
	}
	if got != peer {
		t.Fatalf("PeerInfoFromContext() = %#v, want %#v", got, peer)
	}

	wantErr := errors.New("peer unavailable")
	ctx = ContextWithPeerInfo(context.Background(), PeerInfo{}, wantErr)
	if _, err := PeerInfoFromContext(ctx); !errors.Is(err, wantErr) {
		t.Fatalf("PeerInfoFromContext(error ctx) error = %v, want %v", err, wantErr)
	}

	if _, err := PeerInfoFromContext(context.Background()); !errors.Is(err, ErrPeerCredentialsUnsupported) {
		t.Fatalf("PeerInfoFromContext(empty ctx) error = %v, want ErrPeerCredentialsUnsupported", err)
	}
}

func TestPeerInfoFromConnFailsClosedForUnsupportedConnections(t *testing.T) {
	t.Parallel()

	left, right := net.Pipe()
	defer func() { _ = left.Close() }()
	defer func() { _ = right.Close() }()

	if _, err := PeerInfoFromConn(left); !errors.Is(err, ErrPeerCredentialsUnsupported) {
		t.Fatalf("PeerInfoFromConn(net.Pipe) error = %v, want ErrPeerCredentialsUnsupported", err)
	}
	if _, err := PeerInfoFromConn(nil); err == nil {
		t.Fatal("PeerInfoFromConn(nil) error = nil, want error")
	}
}

func TestPeerInfoFromUnixConnIdentifiesLocalPeer(t *testing.T) {
	t.Parallel()

	socketDir, err := os.MkdirTemp("/tmp", "agh-peer-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socketPath := filepath.Join(socketDir, "p.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		t.Fatalf("ListenUnix() error = %v", err)
	}
	defer func() { _ = listener.Close() }()

	type peerResult struct {
		peer PeerInfo
		err  error
	}
	resultCh := make(chan peerResult, 1)
	go func() {
		conn, acceptErr := listener.AcceptUnix()
		if acceptErr != nil {
			resultCh <- peerResult{err: acceptErr}
			return
		}
		defer func() { _ = conn.Close() }()
		peer, peerErr := PeerInfoFromConn(conn)
		resultCh <- peerResult{peer: peer, err: peerErr}
	}()

	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		t.Fatalf("DialUnix() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case result := <-resultCh:
		if errors.Is(result.err, ErrPeerCredentialsUnsupported) {
			t.Skipf("peer credential inspection unsupported on this platform: %v", result.err)
		}
		if result.err != nil {
			t.Fatalf("PeerInfoFromConn(unix) error = %v", result.err)
		}
		if !result.peer.Supported || result.peer.PID <= 0 || result.peer.UID != os.Getuid() {
			t.Fatalf("peer info = %#v, want supported current user peer", result.peer)
		}
		if result.peer.ExecutablePath == "" {
			t.Fatalf("peer executable path is empty: %#v", result.peer)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for accepted Unix peer")
	}
}
