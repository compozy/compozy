package mcpproxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	utils "github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPProxyIntegration tests the complete MCP proxy workflow
func TestMCPProxyIntegration(t *testing.T) {
	// Initialize logger for tests
	utils.InitLogger(t)

	// Create memory storage
	storage := mcpproxy.NewMemoryStorage()

	// Create mock client manager
	clientManager := mcpproxy.NewMockClientManager()

	// Create server
	config := &mcpproxy.Config{
		Port:            "8080",
		Host:            "localhost",
		BaseURL:         "http://localhost:8080",
		ShutdownTimeout: 5 * time.Second,
		AdminTokens:     []string{"test-admin-token"},
		// No IP restrictions for tests
	}

	server := mcpproxy.NewServer(config, storage, clientManager)

	// Test case: Add MCP with stdio transport
	mcpDef := mcpproxy.MCPDefinition{
		Name:        "test-mcp",
		Description: "Test MCP for integration testing",
		Transport:   mcpproxy.TransportStdio,
		Command:     "echo",
		Args:        []string{"hello"},
	}

	// Convert to JSON
	mcpJSON, err := json.Marshal(mcpDef)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/admin/mcps", bytes.NewReader(mcpJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-admin-token")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve the request
	server.Router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "MCP definition added successfully", response["message"])
	assert.Equal(t, "test-mcp", response["name"])

	// Wait for async MCP registration to complete
	require.Eventually(t, func() bool {
		req := httptest.NewRequest(http.MethodGet, "/admin/mcps", http.NoBody)
		req.Header.Set("Authorization", "Bearer test-admin-token")
		w := httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			return false
		}

		var listResponse map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &listResponse); err != nil {
			return false
		}

		mcps, ok := listResponse["mcps"].([]any)
		return ok && len(mcps) == 1
	}, 2*time.Second, 10*time.Millisecond, "MCP should be registered within timeout")

	// Test healthz endpoint
	req = httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	w = httptest.NewRecorder()
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test admin/mcps endpoint
	req = httptest.NewRequest(http.MethodGet, "/admin/mcps", http.NoBody)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w = httptest.NewRecorder()
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var listResponse map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &listResponse)
	require.NoError(t, err)

	mcps, ok := listResponse["mcps"].([]any)
	require.True(t, ok)
	assert.Len(t, mcps, 1)

	// Test metrics endpoint
	req = httptest.NewRequest(http.MethodGet, "/admin/metrics", http.NoBody)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w = httptest.NewRecorder()
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var metricsResponse map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &metricsResponse)
	require.NoError(t, err)
	assert.Contains(t, metricsResponse, "metrics")
}

// TestAdminSecurity tests the admin API security features
func TestAdminSecurity(t *testing.T) {
	// Initialize logger for tests
	utils.InitLogger(t)

	storage := mcpproxy.NewMemoryStorage()
	clientManager := mcpproxy.NewMockClientManager()

	config := &mcpproxy.Config{
		Port:            "8080",
		Host:            "localhost",
		BaseURL:         "http://localhost:8080",
		ShutdownTimeout: 5 * time.Second,
		AdminTokens:     []string{"valid-token"},
		// Don't restrict IPs for testing httptest
	}

	server := mcpproxy.NewServer(config, storage, clientManager)

	// Test unauthorized access (no token)
	req := httptest.NewRequest(http.MethodGet, "/admin/mcps", http.NoBody)
	w := httptest.NewRecorder()
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test invalid token
	req = httptest.NewRequest(http.MethodGet, "/admin/mcps", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w = httptest.NewRecorder()
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test valid token
	req = httptest.NewRequest(http.MethodGet, "/admin/mcps", http.NoBody)
	req.Header.Set("Authorization", "Bearer valid-token")
	w = httptest.NewRecorder()
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// NOTE: Additional tool execution integration tests are available in integration_tool_test.go
// These tests validate the tool API endpoints structure, authentication, and input validation.
