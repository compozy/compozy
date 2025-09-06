package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Health_Success(t *testing.T) {
	t.Run("Should successfully check health when server responds OK", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/healthz", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		err := client.Health(context.Background())
		assert.NoError(t, err)
	})
}

func TestClient_Health_Failure(t *testing.T) {
	t.Run("Should return error when server responds with error status", func(t *testing.T) {
		// Create test server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Service unavailable"))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		err := client.Health(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "proxy service unhealthy")
	})
}

func TestClient_Register_Success(t *testing.T) {
	t.Run("Should successfully register MCP when server responds with 201", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/admin/mcps", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		def := Definition{
			Name:      "test-mcp",
			URL:       "http://example.com",
			Transport: "sse",
		}

		err := client.Register(context.Background(), &def)
		assert.NoError(t, err)
	})
}

func TestClient_Register_AlreadyExists(t *testing.T) {
	t.Run("Should treat conflict as success when MCP already exists", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte("MCP already exists"))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		def := Definition{
			Name:      "test-mcp",
			URL:       "http://example.com",
			Transport: "sse",
		}

		// Should treat conflict as success (idempotent)
		err := client.Register(context.Background(), &def)
		assert.NoError(t, err)
	})
}

func TestClient_Register_Unauthorized(t *testing.T) {
	t.Run("Should return error when server responds with unauthorized", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		def := Definition{
			Name:      "test-mcp",
			URL:       "http://example.com",
			Transport: "sse",
		}

		err := client.Register(context.Background(), &def)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
	})
}

func TestClient_Deregister_Success(t *testing.T) {
	t.Run("Should successfully deregister MCP when server responds OK", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/admin/mcps/test-mcp", r.URL.Path)
			assert.Equal(t, "DELETE", r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		err := client.Deregister(context.Background(), "test-mcp")
		assert.NoError(t, err)
	})
}

func TestClient_Deregister_NotFound(t *testing.T) {
	t.Run("Should treat not found as success when MCP does not exist", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("MCP not found"))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		// Should treat not found as success (idempotent)
		err := client.Deregister(context.Background(), "nonexistent-mcp")
		assert.NoError(t, err)
	})
}

func TestClient_Deregister_NoContent(t *testing.T) {
	t.Run("Should successfully deregister MCP when server responds with No Content", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/admin/mcps/test-mcp", r.URL.Path)
			assert.Equal(t, "DELETE", r.Method)
			w.WriteHeader(http.StatusNoContent) // 204 response
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		err := client.Deregister(context.Background(), "test-mcp")
		assert.NoError(t, err)
	})
}

func TestClient_ListMCPs_Success(t *testing.T) {
	t.Run("Should successfully list MCPs when server responds with valid JSON", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/admin/mcps", r.URL.Path)
			assert.Equal(t, "GET", r.Method)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"mcps": [
					{
						"name": "test-mcp-1",
						"url": "http://example.com/mcp1",
						"transport": "sse"
					},
					{
						"name": "test-mcp-2",
						"url": "http://example.com/mcp2",
						"transport": "streamable-http"
					}
				]
			}`))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		mcps, err := client.ListMCPs(context.Background())
		require.NoError(t, err)
		assert.Len(t, mcps, 2)
		assert.Equal(t, "test-mcp-1", mcps[0].Name)
		assert.Equal(t, "test-mcp-2", mcps[1].Name)
	})
}

func TestClient_WithInvalidURL(t *testing.T) {
	t.Run("Should return error when using invalid URL", func(t *testing.T) {
		client := NewProxyClient("invalid-url", 5*time.Second)
		defer client.Close()

		err := client.Health(context.Background())
		require.Error(t, err)
		// The "invalid-url" gets treated as a hostname, so we get a DNS lookup error
		assert.Contains(t, err.Error(), "invalid-url")
	})
}

func TestClient_RetryLogic(t *testing.T) {
	t.Run("Should retry request when server initially fails", func(t *testing.T) {
		// Create test server that fails first time, succeeds second time
		var callCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&callCount, 1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		err := client.Health(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount)) // Should have retried once
	})
}

func TestClient_ListTools_Success(t *testing.T) {
	t.Run("Should successfully list tools when server responds with valid data", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/admin/tools", r.URL.Path)
			assert.Equal(t, "GET", r.Method)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"tools": [
					{
						"name": "search-tool",
						"description": "Search for information",
						"inputSchema": {
							"type": "object",
							"properties": {
								"query": {"type": "string"}
							}
						},
						"mcpName": "search-mcp"
					},
					{
						"name": "weather-tool",
						"description": "Get weather information",
						"inputSchema": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							}
						},
						"mcpName": "weather-mcp"
					}
				]
			}`))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		tools, err := client.ListTools(context.Background())
		require.NoError(t, err)
		assert.Len(t, tools, 2)

		// Verify first tool
		assert.Equal(t, "search-tool", tools[0].Name)
		assert.Equal(t, "Search for information", tools[0].Description)
		assert.Equal(t, "search-mcp", tools[0].MCPName)

		// Verify second tool
		assert.Equal(t, "weather-tool", tools[1].Name)
		assert.Equal(t, "Get weather information", tools[1].Description)
		assert.Equal(t, "weather-mcp", tools[1].MCPName)
	})
}

func TestClient_ListTools_Failure(t *testing.T) {
	t.Run("Should return error when server responds with error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		_, err := client.ListTools(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tools request failed")
	})
}

func TestNewProxyClient(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "with http prefix",
			baseURL:  "http://localhost:7077",
			expected: "http://localhost:7077",
		},
		{
			name:     "with https prefix",
			baseURL:  "https://proxy.example.com",
			expected: "https://proxy.example.com",
		},
		{
			name:     "without prefix",
			baseURL:  "localhost:7077",
			expected: "http://localhost:7077",
		},
		{
			name:     "with trailing slash",
			baseURL:  "http://localhost:7077/",
			expected: "http://localhost:7077",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewProxyClient(tt.baseURL, 5*time.Second)
			assert.Equal(t, tt.expected, client.baseURL)
			client.Close()
		})
	}
}

func TestClient_CallTool(t *testing.T) {
	t.Run("Should successfully call tool", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/admin/tools/call", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			// Verify request body
			var req ToolCallRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "test-mcp", req.MCPName)
			assert.Equal(t, "test-tool", req.ToolName)
			assert.Equal(t, "test query", req.Arguments["query"])

			// Send response
			resp := ToolCallResponse{
				Result: map[string]any{
					"data":  "test result",
					"count": 42,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		result, err := client.CallTool(context.Background(), "test-mcp", "test-tool", map[string]any{
			"query": "test query",
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test result", resultMap["data"])
		assert.Equal(t, float64(42), resultMap["count"])
	})

	t.Run("Should handle tool execution error", func(t *testing.T) {
		// Create test server that returns an error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := ToolCallResponse{
				Error: "Tool not found",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		_, err := client.CallTool(context.Background(), "test-mcp", "unknown-tool", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Tool not found")
	})

	t.Run("Should handle HTTP error", func(t *testing.T) {
		// Create test server that returns HTTP error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("MCP not found"))
		}))
		defer server.Close()

		client := NewProxyClient(server.URL, 5*time.Second)
		defer client.Close()

		_, err := client.CallTool(context.Background(), "unknown-mcp", "test-tool", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "tool call failed (status 404)")
	})
}
