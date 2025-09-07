package mcpproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	t.Run("Should initialize server with proper configuration and dependencies", func(t *testing.T) {
		config := &Config{
			Port:            "6001",
			Host:            "localhost",
			ShutdownTimeout: 5 * time.Second,
		}

		server := newTestServer(config)

		assert.NotNil(t, server)
		assert.Equal(t, config, server.config)
		assert.NotNil(t, server.Router)
		assert.NotNil(t, server.httpServer)
	})
}

func TestHealthzEndpoint(t *testing.T) {
	t.Run("Should return healthy status with timestamp and version", func(t *testing.T) {
		config := DefaultConfig()
		server := newTestServer(config)

		req, err := http.NewRequestWithContext(context.Background(), "GET", "/healthz", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		server.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "healthy")
		assert.Contains(t, rr.Body.String(), "timestamp")
		assert.Contains(t, rr.Body.String(), "version")
	})
}

// IP allowlist unit tests removed along with feature.

func TestPingEndpoint(t *testing.T) {
	t.Run("Should respond with pong to ping request", func(t *testing.T) {
		config := DefaultConfig()
		server := newTestServer(config)

		req, err := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/ping", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		server.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "pong")
	})
}

func TestServerShutdown(t *testing.T) {
	t.Run("Should shutdown gracefully on context cancellation", func(t *testing.T) {
		config := &Config{
			Port:            "0",
			Host:            "localhost",
			ShutdownTimeout: 1 * time.Second,
		}

		server := newTestServer(config)

		ctx, cancel := context.WithCancel(context.Background())

		serverErr := make(chan error, 1)

		go func() {
			err := server.Start(ctx)
			serverErr <- err
		}()

		time.Sleep(100 * time.Millisecond)

		cancel()

		select {
		case err := <-serverErr:
			if err != nil && err.Error() != "context canceled" {
				t.Errorf("Expected 'context canceled' error, got: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Server shutdown timed out")
		}
	})
}

// IP/proxy tests removed with feature deprecation.

// Loopback host helper removed with allowlist feature.

// Startup security validation removed along with allowlist feature.
