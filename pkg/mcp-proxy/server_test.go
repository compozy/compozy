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
	config := &Config{
		Port:            "8080",
		Host:            "localhost",
		ShutdownTimeout: 5 * time.Second,
	}

	server := newTestServer(config)

	assert.NotNil(t, server)
	assert.Equal(t, config, server.config)
	assert.NotNil(t, server.router)
	assert.NotNil(t, server.httpServer)
}

func TestHealthzEndpoint(t *testing.T) {
	config := DefaultConfig()
	server := newTestServer(config)

	// Create a test request
	req, err := http.NewRequestWithContext(context.Background(), "GET", "/healthz", http.NoBody)
	require.NoError(t, err)

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Execute the request
	server.router.ServeHTTP(rr, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "healthy")
	assert.Contains(t, rr.Body.String(), "timestamp")
	assert.Contains(t, rr.Body.String(), "version")
}

func TestPingEndpoint(t *testing.T) {
	config := DefaultConfig()
	server := newTestServer(config)

	// Create a test request
	req, err := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/ping", http.NoBody)
	require.NoError(t, err)

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Execute the request
	server.router.ServeHTTP(rr, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "pong")
}

func TestServerShutdown(t *testing.T) {
	config := &Config{
		Port:            "0", // Use any available port
		Host:            "localhost",
		ShutdownTimeout: 1 * time.Second,
	}

	server := newTestServer(config)

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	go func() {
		err := server.Start(ctx)
		assert.NoError(t, err)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Give server time to shutdown
	time.Sleep(200 * time.Millisecond)
}
