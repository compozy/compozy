package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTransport struct {
	base   http.RoundTripper
	called bool
}

func (s *stubTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return s.base.RoundTrip(r)
}
func (s *stubTransport) CloseIdleConnections() { s.called = true }

func TestRegisterService_Ensure(t *testing.T) {
	t.Run("Should register MCP successfully when not already registered", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/admin/mcps" && r.Method == "POST" {
				w.WriteHeader(http.StatusCreated)
			}
		}))
		defer server.Close()
		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
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
		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
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
		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
		service := NewRegisterService(client)
		err := service.Deregister(context.Background(), "test-mcp")
		assert.NoError(t, err)
	})
	t.Run("Should handle deregistration when MCP not registered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
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
		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
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
		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
		service := NewRegisterService(client)
		err := service.EnsureMultiple(context.Background(), []Config{})
		assert.NoError(t, err)
	})
}

func TestRegisterService_Shutdown(t *testing.T) {
	t.Run("Should deregister all MCPs during shutdown", func(t *testing.T) {
		var deregisteredMCPs []string
		var mu sync.Mutex
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
				mu.Lock()
				deregisteredMCPs = append(deregisteredMCPs, mcpID)
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()
		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
		service := NewRegisterService(client)
		err := service.Shutdown(context.Background())
		assert.NoError(t, err)
		// Verify both MCPs were deregistered
		assert.Contains(t, deregisteredMCPs, "mcp-1")
		assert.Contains(t, deregisteredMCPs, "mcp-2")
	})

	t.Run("Should call proxy Close() during shutdown (defer)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == "GET" && r.URL.Path == "/admin/mcps":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"mcps": []Definition{{Name: "m1"}, {Name: "m2"}}})
			case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/admin/mcps/"):
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
		st := &stubTransport{base: http.DefaultTransport}
		client.http.Transport = st
		service := NewRegisterService(client)
		err := service.Shutdown(context.Background())
		assert.NoError(t, err)
		assert.True(t, st.called, "expected CloseIdleConnections to be called via client.Close()")
	})
}

func TestTemplateValidation_StrictModeRules(t *testing.T) {
	t.Run("Should allow simple lookups", func(t *testing.T) {
		err := validateTemplate("{{ .env.API_KEY }}")
		assert.NoError(t, err)
	})
	t.Run("Should reject pipelines", func(t *testing.T) {
		err := validateTemplate("{{ .env.API_KEY | printf \"%s\" }}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pipelines")
	})
	t.Run("Should reject function calls", func(t *testing.T) {
		err := validateTemplate("{{ printf \"%s\" .env.API_KEY }}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "function")
	})
	t.Run("Should reject template inclusions", func(t *testing.T) {
		err := validateTemplate("{{ template \"name\" }}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "inclusions")
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

		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
		service := NewRegisterService(client)

		err := service.HealthCheck(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Should fail health check when proxy is unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewProxyClient(context.Background(), server.URL, 5*time.Second)
		service := NewRegisterService(client)

		err := service.HealthCheck(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "proxy health check failed")
	})
}

func TestRegisterService_ConvertToDefinition(t *testing.T) {
	t.Run("Should convert valid remote MCP config to proxy definition", func(t *testing.T) {
		client := NewProxyClient(context.Background(), "http://localhost:7077", 5*time.Second)
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
		client := NewProxyClient(context.Background(), "http://localhost:7077", 5*time.Second)
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

	t.Run("Should merge explicit args with parsed command args", func(t *testing.T) {
		client := NewProxyClient(context.Background(), "http://localhost:7077", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:        "npx-mcp",
			Command:   "npx",
			Args:      []string{"-y", "mcp-server-fetch"},
			Transport: "stdio",
		}

		def, err := service.convertToDefinition(&config)
		require.NoError(t, err)

		assert.Equal(t, "npx", def.Command)
		assert.Equal(t, []string{"-y", "mcp-server-fetch"}, def.Args)
	})

	t.Run("Should return error when neither URL nor Command is provided", func(t *testing.T) {
		client := NewProxyClient(context.Background(), "http://localhost:7077", 5*time.Second)
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
		client := NewProxyClient(context.Background(), "http://localhost:7077", 5*time.Second)
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

	t.Run("Should return error for URL-based MCP with invalid transport", func(t *testing.T) {
		client := NewProxyClient(context.Background(), "http://localhost:7077", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:        "test-mcp",
			URL:       "http://example.com/mcp",
			Transport: "stdio", // Invalid for URL-based MCP
		}

		_, err := service.convertToDefinition(&config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "remote MCP must use 'sse' or 'streamable-http' transport")
	})

	t.Run("Should return error for command-based MCP with invalid transport", func(t *testing.T) {
		client := NewProxyClient(context.Background(), "http://localhost:7077", 5*time.Second)
		service := NewRegisterService(client)

		config := Config{
			ID:        "test-mcp",
			Command:   "node server.js",
			Transport: "sse", // Invalid for command-based MCP
		}

		_, err := service.convertToDefinition(&config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stdio MCP must use 'stdio' transport")
	})

	t.Run("Should accept valid transport combinations", func(t *testing.T) {
		client := NewProxyClient(context.Background(), "http://localhost:7077", 5*time.Second)
		service := NewRegisterService(client)

		// Test valid URL + SSE
		config1 := Config{
			ID:        "test-mcp-sse",
			URL:       "http://example.com/mcp",
			Transport: "sse",
		}
		_, err := service.convertToDefinition(&config1)
		assert.NoError(t, err)

		// Test valid URL + streamable-http
		config2 := Config{
			ID:        "test-mcp-streamable",
			URL:       "http://example.com/mcp",
			Transport: "streamable-http",
		}
		_, err = service.convertToDefinition(&config2)
		assert.NoError(t, err)

		// Test valid command + stdio
		config3 := Config{
			ID:        "test-mcp-stdio",
			Command:   "node server.js",
			Transport: "stdio",
		}
		_, err = service.convertToDefinition(&config3)
		assert.NoError(t, err)
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
