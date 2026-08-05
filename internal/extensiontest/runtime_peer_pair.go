package extensiontest

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/bridgesdk"
)

const defaultRuntimePeerCleanupTimeout = 2 * time.Second

// RuntimePeerPairConfig defines one provider runtime loop connected to a host peer.
type RuntimePeerPairConfig struct {
	ServeRuntime func(io.Reader, io.Writer) error
	Stop         func()
	Shutdown     func(context.Context) error
	Wait         func(context.Context) error
	Timeout      time.Duration
}

// NewRuntimePeerPair starts and owns a provider runtime/host peer connection.
func NewRuntimePeerPair(t testing.TB, cfg RuntimePeerPairConfig) (*bridgesdk.Peer, func()) {
	t.Helper()
	if cfg.ServeRuntime == nil {
		t.Fatal("extensiontest: runtime serve function is required")
	}

	hostConn, runtimeConn := net.Pipe()
	hostPeer, err := bridgesdk.NewPeer(hostConn, hostConn)
	if err != nil {
		closeRuntimePeerConnection(t, "host", hostConn)
		closeRuntimePeerConnection(t, "runtime", runtimeConn)
		t.Fatalf("extensiontest: bridgesdk.NewPeer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errorsCh := make(chan error, 2)
	go func() { errorsCh <- cfg.ServeRuntime(runtimeConn, runtimeConn) }()
	go func() { errorsCh <- hostPeer.Serve(ctx) }()

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultRuntimePeerCleanupTimeout
	}
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			cancel()
			if cfg.Stop != nil {
				cfg.Stop()
			}
			if cfg.Shutdown != nil {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
				if shutdownErr := cfg.Shutdown(shutdownCtx); shutdownErr != nil {
					t.Errorf("extensiontest: runtime shutdown error = %v", shutdownErr)
				}
				shutdownCancel()
			}
			closeRuntimePeerConnection(t, "host", hostConn)
			closeRuntimePeerConnection(t, "runtime", runtimeConn)
			joinRuntimePeerServeLoops(t, errorsCh, timeout)
			if cfg.Wait != nil {
				waitCtx, waitCancel := context.WithTimeout(context.Background(), timeout)
				if waitErr := cfg.Wait(waitCtx); waitErr != nil {
					t.Errorf("extensiontest: provider lifecycle wait error = %v", waitErr)
				}
				waitCancel()
			}
		})
	}
	return hostPeer, cleanup
}

func closeRuntimePeerConnection(t testing.TB, name string, connection net.Conn) {
	t.Helper()
	if connection == nil {
		return
	}
	if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Errorf("extensiontest: %s connection close error = %v", name, err)
	}
}

func joinRuntimePeerServeLoops(t testing.TB, errorsCh <-chan error, timeout time.Duration) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for range 2 {
		select {
		case err := <-errorsCh:
			if runtimePeerServeErrorExpected(err) {
				continue
			}
			t.Errorf("extensiontest: runtime peer serve error = %v", err)
		case <-timer.C:
			t.Errorf("extensiontest: runtime peer serve cleanup exceeded %s", timeout)
			return
		}
	}
}

func runtimePeerServeErrorExpected(err error) bool {
	return err == nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, io.ErrClosedPipe)
}
