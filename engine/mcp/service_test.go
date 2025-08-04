package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterService_Ensure(t *testing.T) {
	t.Run("Should register MCP successfully when not already registered", func(t *testing.T) {
		// Create test server
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
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to register MCP with proxy")
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
		err := service.Deregister(context.Background(), "test-mcp")
		assert.NoError(t, err)
	})
	t.Run("Should handle deregistration when MCP not registered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)
		err := service.Deregister(context.Background(), "test-mcp")
		assert.NoError(t, err)
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
		var deregisteredMCPs []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && r.URL.Path == "/admin/mcps" {
				// Return list of MCPs
				response := struct {
					MCPs []Definition `json:"mcps"`
				}{
					MCPs: []Definition{
						{Name: "mcp-1", Transport: "sse"},
						{Name: "mcp-2", Transport: "sse"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			} else if r.Method == "DELETE" {
				mcpID := r.URL.Path[len("/admin/mcps/"):]
				deregisteredMCPs = append(deregisteredMCPs, mcpID)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()
		client := NewProxyClient(server.URL, "", 5*time.Second)
		service := NewRegisterService(client)
		err := service.Shutdown(context.Background())
		assert.NoError(t, err)
		// Verify both MCPs were deregistered
		assert.Contains(t, deregisteredMCPs, "mcp-1")
		assert.Contains(t, deregisteredMCPs, "mcp-2")
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
		require.Error(t, err)
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
		require.Error(t, err)
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
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MCP transport is required")
	})
}

func TestParseCommand(t *testing.T) {
	t.Run("Should parse simple command without arguments", func(t *testing.T) {
		parts, err := parseCommand("node")
		assert.NoError(t, err)
		assert.Equal(t, []string{"node"}, parts)
	})

	t.Run("Should parse command with simple arguments", func(t *testing.T) {
		parts, err := parseCommand("node server.js --port 3000")
		assert.NoError(t, err)
		assert.Equal(t, []string{"node", "server.js", "--port", "3000"}, parts)
	})

	t.Run("Should parse command with quoted arguments containing spaces", func(t *testing.T) {
		parts, err := parseCommand(`python server.py --config "config file.json" --env production`)
		assert.NoError(t, err)
		assert.Equal(t, []string{"python", "server.py", "--config", "config file.json", "--env", "production"}, parts)
	})

	t.Run("Should parse command with single quoted arguments", func(t *testing.T) {
		parts, err := parseCommand(`node server.js --path '/tmp/data with spaces'`)
		assert.NoError(t, err)
		assert.Equal(t, []string{"node", "server.js", "--path", "/tmp/data with spaces"}, parts)
	})

	t.Run("Should parse command with escaped quotes", func(t *testing.T) {
		parts, err := parseCommand(`echo "He said \"hello\""`)
		assert.NoError(t, err)
		assert.Equal(t, []string{"echo", `He said "hello"`}, parts)
	})

	t.Run("Should return error for empty command", func(t *testing.T) {
		_, err := parseCommand("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command cannot be empty")
	})

	t.Run("Should return error for command with newlines", func(t *testing.T) {
		_, err := parseCommand("node\nserver.js")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command cannot contain newlines")
	})

	t.Run("Should return error for command starting with dash", func(t *testing.T) {
		_, err := parseCommand("--help")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command name cannot start with dash")
	})

	t.Run("Should return error for malformed quotes", func(t *testing.T) {
		_, err := parseCommand(`node server.js "unclosed quote`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse command")
	})
}
