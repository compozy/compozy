package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Initialize logger for tests
	logger.InitForTests()
}

func TestRegisterService_Ensure(t *testing.T) {
	t.Run("Should register MCP successfully when not already registered", func(t *testing.T) {
		// Create mock proxy server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/admin/mcps" && r.Method == "POST" {
				w.WriteHeader(http.StatusCreated)
			}
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:        "test-mcp",
			URL:       "http://example.com/mcp",
			Transport: "sse",
		}

		err := service.Ensure(context.Background(), &config)
		assert.NoError(t, err)
		assert.True(t, service.IsRegistered("test-mcp"))
	})

	t.Run("Should skip registration when MCP already registered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("Should not make any HTTP calls for already registered MCP")
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		// Pre-register the MCP
		service.regs["test-mcp"] = true

		config := Config{
			ID:        "test-mcp",
			URL:       "http://example.com/mcp",
			Transport: "sse",
		}

		err := service.Ensure(context.Background(), &config)
		assert.NoError(t, err)
		assert.True(t, service.IsRegistered("test-mcp"))
	})

	t.Run("Should return error when proxy registration fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:        "test-mcp",
			URL:       "http://example.com/mcp",
			Transport: "sse",
		}

		err := service.Ensure(context.Background(), &config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to register MCP with proxy")
		assert.False(t, service.IsRegistered("test-mcp"))
	})
}

func TestRegisterService_Deregister(t *testing.T) {
	t.Run("Should deregister MCP successfully when registered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" && r.URL.Path == "/admin/mcps/test-mcp" {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		// Pre-register the MCP
		service.regs["test-mcp"] = true

		err := service.Deregister(context.Background(), "test-mcp")
		assert.NoError(t, err)
		assert.False(t, service.IsRegistered("test-mcp"))
	})

	t.Run("Should skip deregistration when MCP not registered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("Should not make any HTTP calls for unregistered MCP")
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		err := service.Deregister(context.Background(), "test-mcp")
		assert.NoError(t, err)
		assert.False(t, service.IsRegistered("test-mcp"))
	})
}

func TestRegisterService_EnsureMultiple(t *testing.T) {
	t.Run("Should register multiple MCPs successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" && r.URL.Path == "/admin/mcps" {
				w.WriteHeader(http.StatusCreated)
			}
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		configs := []Config{
			{ID: "mcp-1", URL: "http://example.com/mcp1", Transport: "sse"},
			{ID: "mcp-2", URL: "http://example.com/mcp2", Transport: "streamable-http"},
		}

		err := service.EnsureMultiple(context.Background(), configs)
		assert.NoError(t, err)

		// Verify both MCPs are registered
		assert.True(t, service.IsRegistered("mcp-1"))
		assert.True(t, service.IsRegistered("mcp-2"))

		// Verify we have correct count
		registered := service.ListRegistered()
		assert.Len(t, registered, 2)
	})

	t.Run("Should handle empty MCP list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("Should not make any HTTP calls for empty MCP list")
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		err := service.EnsureMultiple(context.Background(), []Config{})
		assert.NoError(t, err)
	})
}

func TestRegisterService_Shutdown(t *testing.T) {
	t.Run("Should deregister all MCPs during shutdown", func(t *testing.T) {
		deregisteredMCPs := make(map[string]bool)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				mcpID := r.URL.Path[len("/admin/mcps/"):]
				deregisteredMCPs[mcpID] = true
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		// Pre-register some MCPs
		service.regs["mcp-1"] = true
		service.regs["mcp-2"] = true

		err := service.Shutdown(context.Background())
		assert.NoError(t, err)

		// Verify all MCPs were deregistered
		assert.True(t, deregisteredMCPs["mcp-1"])
		assert.True(t, deregisteredMCPs["mcp-2"])

		// Verify registry is cleared
		assert.Len(t, service.ListRegistered(), 0)
	})
}

func TestRegisterService_HealthCheck(t *testing.T) {
	t.Run("Should pass health check when proxy is healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		err := service.HealthCheck(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Should fail health check when proxy is unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)

		err := service.HealthCheck(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "proxy health check failed")
	})
}

func TestRegisterService_ConvertToDefinition(t *testing.T) {
	t.Run("Should convert valid remote MCP config to proxy definition", func(t *testing.T) {
		client := NewProxyClient("http://localhost:7077", "", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:        "test-mcp",
			URL:       "http://example.com/mcp",
			Transport: "sse",
			Env:       map[string]string{"API_KEY": "secret"},
		}

		def, err := service.convertToDefinition(&config)
		require.NoError(t, err)

		assert.Equal(t, "test-mcp", def.Name)
		assert.Equal(t, "http://example.com/mcp", def.URL)
		assert.Equal(t, mcpproxy.TransportSSE, def.Transport)
		assert.Equal(t, "secret", def.Env["API_KEY"])
	})

	t.Run("Should convert valid stdio MCP config to proxy definition", func(t *testing.T) {
		client := NewProxyClient("http://localhost:7077", "", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:        "test-stdio-mcp",
			Command:   "node server.js --port 3000",
			Transport: "stdio",
			Env:       map[string]string{"NODE_ENV": "production"},
		}

		def, err := service.convertToDefinition(&config)
		require.NoError(t, err)

		assert.Equal(t, "test-stdio-mcp", def.Name)
		assert.Equal(t, "node", def.Command)
		assert.Equal(t, []string{"server.js", "--port", "3000"}, def.Args)
		assert.Equal(t, mcpproxy.TransportStdio, def.Transport)
		assert.Equal(t, "production", def.Env["NODE_ENV"])
	})

	t.Run("Should return error when neither URL nor Command is provided", func(t *testing.T) {
		client := NewProxyClient("http://localhost:7077", "", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:        "test-mcp",
			Transport: "stdio",
			// Neither URL nor Command provided
		}

		_, err := service.convertToDefinition(&config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MCP configuration must specify either URL (for remote) or Command (for stdio)")
	})

	t.Run("Should return error for missing required fields", func(t *testing.T) {
		client := NewProxyClient("http://localhost:7077", "", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:  "test-mcp",
			URL: "http://example.com/mcp",
			// Missing Transport
		}

		_, err := service.convertToDefinition(&config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MCP transport is required")
	})
}
