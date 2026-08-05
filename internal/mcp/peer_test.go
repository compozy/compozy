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
	t.Run("Should require a caller-owned context", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("ContextWithPeerInfo(nil) did not panic")
			}
		}()
		//nolint:staticcheck // This subtest proves the explicit nil-context rejection contract.
		ContextWithPeerInfo(nil, PeerInfo{}, nil)
	})

	t.Run("Should preserve peer information and fail closed when absent", func(t *testing.T) {
		t.Parallel()

		peer := PeerInfo{Supported: true, PID: 10, UID: 20, GID: 30, ExecutablePath: "/bin/compozy"}
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
	})
}

func TestPeerInfoFromConnFailsClosedForUnsupportedConnections(t *testing.T) {
	t.Run("Should reject unsupported and nil connections", func(t *testing.T) {
		t.Parallel()

		left, right := net.Pipe()
		t.Cleanup(func() {
			if err := left.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("left.Close() error = %v", err)
			}
		})
		t.Cleanup(func() {
			if err := right.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("right.Close() error = %v", err)
			}
		})

		if _, err := PeerInfoFromConn(left); !errors.Is(err, ErrPeerCredentialsUnsupported) {
			t.Fatalf("PeerInfoFromConn(net.Pipe) error = %v, want ErrPeerCredentialsUnsupported", err)
		}
		if _, err := PeerInfoFromConn(nil); err == nil {
			t.Fatal("PeerInfoFromConn(nil) error = nil, want error")
		}
	})
}

func TestPeerInfoFromUnixConnIdentifiesLocalPeer(t *testing.T) {
	t.Run("Should identify the local peer over a contextual Unix connection", func(t *testing.T) {
		t.Parallel()

		socketDir, err := os.MkdirTemp("/tmp", "compozy-peer-")
		if err != nil {
			t.Fatalf("MkdirTemp() error = %v", err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(socketDir); err != nil {
				t.Errorf("RemoveAll(%q) error = %v", socketDir, err)
			}
		})
		socketPath := filepath.Join(socketDir, "p.sock")
		listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
		if err != nil {
			t.Fatalf("ListenUnix() error = %v", err)
		}
		t.Cleanup(func() {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("listener.Close() error = %v", err)
			}
		})

		type peerResult struct {
			peer     PeerInfo
			err      error
			closeErr error
		}
		resultCh := make(chan peerResult, 1)
		go func() {
			conn, acceptErr := listener.AcceptUnix()
			if acceptErr != nil {
				resultCh <- peerResult{err: acceptErr}
				return
			}
			peer, peerErr := PeerInfoFromConn(conn)
			resultCh <- peerResult{peer: peer, err: peerErr, closeErr: conn.Close()}
		}()

		client, err := (&net.Dialer{}).DialUnix(
			t.Context(),
			"unix",
			nil,
			&net.UnixAddr{Name: socketPath, Net: "unix"},
		)
		if err != nil {
			t.Fatalf("DialUnix() error = %v", err)
		}
		t.Cleanup(func() {
			if err := client.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("client.Close() error = %v", err)
			}
		})

		select {
		case result := <-resultCh:
			if result.closeErr != nil && !errors.Is(result.closeErr, net.ErrClosed) {
				t.Fatalf("accepted connection Close() error = %v", result.closeErr)
			}
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
	})
}
