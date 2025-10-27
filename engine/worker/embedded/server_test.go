package embedded

import (
	"context"
	"net"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var nextTestPort uint32 = 54000

func TestNewServer(t *testing.T) {
	t.Run("Should create server with valid config", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		cfg := newTestConfig(t)

		srv, err := NewServer(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, srv)
		assert.Equal(t, cfg.DatabaseFile, srv.config.DatabaseFile)
		assert.Equal(t, cfg.ClusterName, srv.config.ClusterName)
		assert.Equal(t, cfg.Namespace, srv.config.Namespace)
		assert.False(t, srv.started)
		assert.Equal(t, srv.frontendAddr, srv.FrontendAddress())
	})

	t.Run("Should reject invalid config", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		cfg := newTestConfig(t)
		cfg.FrontendPort = -1

		srv, err := NewServer(ctx, cfg)
		require.Error(t, err)
		assert.Nil(t, srv)
	})
}

func TestServerStartStop(t *testing.T) {
	t.Run("Should start server successfully", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		cfg := newTestConfig(t)

		srv := newServerForTest(ctx, t, cfg)
		require.NoError(t, srv.Start(ctx))
		t.Cleanup(func() {
			require.NoError(t, srv.Stop(ctx))
		})

		dialer := &net.Dialer{Timeout: 500 * time.Millisecond}
		conn, err := dialer.DialContext(ctx, "tcp", srv.FrontendAddress())
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})

	t.Run("Should stop server gracefully", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		cfg := newTestConfig(t)

		srv := newServerForTest(ctx, t, cfg)
		require.NoError(t, srv.Start(ctx))

		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(stopCtx))

		dialer := &net.Dialer{Timeout: 200 * time.Millisecond}
		_, err := dialer.DialContext(ctx, "tcp", srv.FrontendAddress())
		require.Error(t, err)
	})

	t.Run("Should timeout if server doesn't start", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		cfg := newTestConfig(t)
		cfg.StartTimeout = time.Nanosecond

		srv := newServerForTest(ctx, t, cfg)
		err := srv.Start(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, "wait for ready")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("Should handle port conflicts", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		listener, port := reservePort(t)
		defer func() { require.NoError(t, listener.Close()) }()

		cfg := newTestConfig(t)
		cfg.FrontendPort = port

		srv, err := NewServer(ctx, cfg)
		assert.Nil(t, srv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port")
		assert.Contains(t, err.Error(), strconv.Itoa(port))
	})

	t.Run("Should wait for ready state", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		cfg := newTestConfig(t)

		srv := newServerForTest(ctx, t, cfg)
		require.NoError(t, srv.Start(ctx))
		t.Cleanup(func() {
			require.NoError(t, srv.Stop(ctx))
		})

		waitCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		require.NoError(t, srv.waitForReady(waitCtx))
	})
}

func newServerForTest(ctx context.Context, t *testing.T, cfg *Config) *Server {
	t.Helper()
	srv, err := NewServer(ctx, cfg)
	require.NoError(t, err)
	return srv
}

func newTestConfig(t *testing.T) *Config {
	t.Helper()
	return &Config{
		DatabaseFile: filepath.Join(t.TempDir(), "temporal.db"),
		FrontendPort: randomPort(t),
		BindIP:       "127.0.0.1",
		Namespace:    "default",
		ClusterName:  "cluster-" + t.Name(),
		EnableUI:     false,
		UIPort:       0,
		LogLevel:     "error",
		StartTimeout: 10 * time.Second,
	}
}

func randomPort(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 512; attempt++ {
		base := atomic.AddUint32(&nextTestPort, 5)
		port := int(54000 + base%5000)
		if port+maxServicePortOffset > maxPort {
			atomic.StoreUint32(&nextTestPort, 0)
			continue
		}
		ports := []int{port, port + 1, port + 2, port + 3}
		if err := ensurePortsAvailable(t.Context(), "127.0.0.1", ports); err != nil {
			continue
		}
		return port
	}
	t.Fatalf("failed to allocate tcp port after multiple attempts")
	return 0
}

func reservePort(t *testing.T) (net.Listener, int) {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().(*net.TCPAddr)
	return listener, addr.Port
}
