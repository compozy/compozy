package mcpproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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
	assert.NotNil(t, server.Router)
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
	server.Router.ServeHTTP(rr, req)

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
	server.Router.ServeHTTP(rr, req)

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

	// Channel to capture server error
	serverErr := make(chan error, 1)

	// Start server in background
	go func() {
		err := server.Start(ctx)
		serverErr <- err
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for server to shutdown and check error
	select {
	case err := <-serverErr:
		// When context is canceled, we expect a "context canceled" error
		// This is normal behavior for graceful shutdown
		if err != nil && err.Error() != "context canceled" {
			t.Errorf("Expected 'context canceled' error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Server shutdown timed out")
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies []string
		remoteAddr     string
		headers        map[string]string
		expectedIP     string
	}{
		{
			name:           "Direct connection without headers",
			trustedProxies: []string{},
			remoteAddr:     "192.168.1.1:8080",
			headers:        map[string]string{},
			expectedIP:     "192.168.1.1",
		},
		{
			name:           "Direct connection with spoofed X-Forwarded-For (no trusted proxies)",
			trustedProxies: []string{},
			remoteAddr:     "192.168.1.1:8080",
			headers:        map[string]string{"X-Forwarded-For": "10.0.0.1"},
			expectedIP:     "192.168.1.1", // Should ignore header
		},
		{
			name:           "Direct connection with spoofed X-Real-IP (no trusted proxies)",
			trustedProxies: []string{},
			remoteAddr:     "192.168.1.1:8080",
			headers:        map[string]string{"X-Real-IP": "10.0.0.1"},
			expectedIP:     "192.168.1.1", // Should ignore header
		},
		{
			name:           "Trusted proxy with X-Forwarded-For",
			trustedProxies: []string{"10.0.0.100"},
			remoteAddr:     "10.0.0.100:8080",
			headers:        map[string]string{"X-Forwarded-For": "203.0.113.1"},
			expectedIP:     "203.0.113.1", // Should trust header
		},
		{
			name:           "Trusted proxy with X-Real-IP",
			trustedProxies: []string{"10.0.0.100"},
			remoteAddr:     "10.0.0.100:8080",
			headers:        map[string]string{"X-Real-IP": "203.0.113.1"},
			expectedIP:     "203.0.113.1", // Should trust header
		},
		{
			name:           "Trusted proxy CIDR with X-Forwarded-For",
			trustedProxies: []string{"10.0.0.0/24"},
			remoteAddr:     "10.0.0.50:8080",
			headers:        map[string]string{"X-Forwarded-For": "203.0.113.1, 10.0.0.50"},
			expectedIP:     "203.0.113.1", // Should trust header and get first IP
		},
		{
			name:           "Untrusted proxy with X-Forwarded-For",
			trustedProxies: []string{"10.0.0.100"},
			remoteAddr:     "192.168.1.1:8080",
			headers:        map[string]string{"X-Forwarded-For": "203.0.113.1"},
			expectedIP:     "192.168.1.1", // Should ignore header from untrusted source
		},
		{
			name:           "Trusted proxy with both headers (X-Forwarded-For takes precedence)",
			trustedProxies: []string{"10.0.0.100"},
			remoteAddr:     "10.0.0.100:8080",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
				"X-Real-IP":       "203.0.113.2",
			},
			expectedIP: "203.0.113.1", // X-Forwarded-For should take precedence
		},
		{
			name:           "Trusted proxy with empty X-Forwarded-For falls back to X-Real-IP",
			trustedProxies: []string{"10.0.0.100"},
			remoteAddr:     "10.0.0.100:8080",
			headers: map[string]string{
				"X-Forwarded-For": "",
				"X-Real-IP":       "203.0.113.2",
			},
			expectedIP: "203.0.113.2", // Should fall back to X-Real-IP
		},
		{
			name:           "RemoteAddr without port",
			trustedProxies: []string{},
			remoteAddr:     "192.168.1.1",
			headers:        map[string]string{},
			expectedIP:     "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				TrustedProxies: tt.trustedProxies,
			}
			server := &Server{config: config}

			// Create a mock gin context
			req := httptest.NewRequest("GET", "/test", http.NoBody)
			req.RemoteAddr = tt.remoteAddr

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			rr := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rr)
			c.Request = req

			// Test the function
			result := server.getClientIP(c)
			assert.Equal(t, tt.expectedIP, result)
		})
	}
}

func TestIsTrustedProxy(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies []string
		clientIP       string
		expected       bool
	}{
		{
			name:           "Empty trusted proxies list",
			trustedProxies: []string{},
			clientIP:       "10.0.0.1",
			expected:       false,
		},
		{
			name:           "Exact IP match",
			trustedProxies: []string{"10.0.0.1", "192.168.1.1"},
			clientIP:       "10.0.0.1",
			expected:       true,
		},
		{
			name:           "IP not in list",
			trustedProxies: []string{"10.0.0.1", "192.168.1.1"},
			clientIP:       "203.0.113.1",
			expected:       false,
		},
		{
			name:           "CIDR match",
			trustedProxies: []string{"10.0.0.0/24"},
			clientIP:       "10.0.0.50",
			expected:       true,
		},
		{
			name:           "CIDR no match",
			trustedProxies: []string{"10.0.0.0/24"},
			clientIP:       "10.0.1.50",
			expected:       false,
		},
		{
			name:           "Mixed IP and CIDR",
			trustedProxies: []string{"192.168.1.1", "10.0.0.0/16"},
			clientIP:       "10.0.5.10",
			expected:       true,
		},
		{
			name:           "Invalid IP",
			trustedProxies: []string{"10.0.0.1"},
			clientIP:       "invalid-ip",
			expected:       false,
		},
		{
			name:           "Invalid CIDR in config",
			trustedProxies: []string{"invalid-cidr/24"},
			clientIP:       "10.0.0.1",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				TrustedProxies: tt.trustedProxies,
			}
			server := &Server{config: config}

			result := server.isTrustedProxy(tt.clientIP)
			assert.Equal(t, tt.expected, result)
		})
	}
}
