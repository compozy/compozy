package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Initialize logger for tests
	logger.InitForTests()
}

func TestNewProxyTool(t *testing.T) {
	t.Run("Should create proxy tool with correct properties", func(t *testing.T) {
		toolDef := mcp.ToolDefinition{
			Name:        "test-tool",
			Description: "A test tool",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query",
					},
				},
			},
			MCPName: "test-mcp",
		}

		proxyClient := mcp.NewProxyClient("http://localhost:7077", "", 0)
		defer proxyClient.Close()

		tool := NewProxyTool(toolDef, proxyClient)
		require.NotNil(t, tool)

		assert.Equal(t, "test-tool", tool.Name())
		assert.Equal(t, "A test tool", tool.Description())
	})
}

func TestProxyTool_Call(t *testing.T) {
	t.Run("Should execute tool via proxy and return result", func(t *testing.T) {
		// Create test server to mock the proxy
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/admin/tools/call", r.URL.Path)

			// Verify request
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "search-mcp", req["mcpName"])
			assert.Equal(t, "search-tool", req["toolName"])

			// Send successful response
			resp := map[string]any{
				"result": map[string]any{
					"results": []string{"result1", "result2"},
					"count":   2,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		toolDef := mcp.ToolDefinition{
			Name:        "search-tool",
			Description: "Search for information",
			MCPName:     "search-mcp",
		}

		proxyClient := mcp.NewProxyClient(server.URL, "", 5*time.Second)
		defer proxyClient.Close()

		tool := NewProxyTool(toolDef, proxyClient)

		input := `{"query": "test search", "limit": 10}`
		result, err := tool.Call(context.Background(), input)

		require.NoError(t, err)

		// Verify the result is valid JSON
		var resultData map[string]any
		err = json.Unmarshal([]byte(result), &resultData)
		require.NoError(t, err)

		// Check the structure
		assert.NotNil(t, resultData["results"])
		assert.Equal(t, float64(2), resultData["count"])
	})

	t.Run("Should return error for invalid JSON input", func(t *testing.T) {
		toolDef := mcp.ToolDefinition{
			Name:    "test-tool",
			MCPName: "test-mcp",
		}

		proxyClient := mcp.NewProxyClient("http://localhost:7077", "", 0)
		defer proxyClient.Close()

		tool := NewProxyTool(toolDef, proxyClient)

		invalidInput := `{invalid json`
		_, err := tool.Call(context.Background(), invalidInput)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse tool arguments")
	})
}

func TestProxyTool_ArgsType(t *testing.T) {
	t.Run("Should return input schema", func(t *testing.T) {
		inputSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type": "string",
				},
			},
		}

		toolDef := mcp.ToolDefinition{
			Name:        "test-tool",
			InputSchema: inputSchema,
			MCPName:     "test-mcp",
		}

		proxyClient := mcp.NewProxyClient("http://localhost:7077", "", 0)
		defer proxyClient.Close()

		tool := NewProxyTool(toolDef, proxyClient)

		// Cast to our custom type to access ArgsType (not part of langchain interface)
		if pTool, ok := tool.(*ProxyTool); ok {
			argsType := pTool.ArgsType()
			assert.Equal(t, inputSchema, argsType)
		} else {
			t.Fatal("Tool is not of ProxyTool type")
		}
	})
}
