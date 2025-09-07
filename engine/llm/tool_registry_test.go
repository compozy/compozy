package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toolsResponse mirrors engine/mcp/client.go response structure
type toolsResponse struct {
	Tools []mcp.ToolDefinition `json:"tools"`
}

func TestToolRegistry_AllowedMCPFiltering(t *testing.T) {
	t.Run("Should list all MCP tools when allowlist is empty", func(t *testing.T) {
		srv := makeToolsServer(t, []mcp.ToolDefinition{
			{Name: "tool-a", Description: "A", MCPName: "mcp1"},
			{Name: "tool-b", Description: "B", MCPName: "mcp2"},
		})
		defer srv.Close()

		client := mcp.NewProxyClient(srv.URL, 2*time.Second)
		reg := NewToolRegistry(ToolRegistryConfig{ProxyClient: client, CacheTTL: 1 * time.Millisecond})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tools, err := reg.ListAll(ctx)
		require.NoError(t, err)
		names := namesOf(tools)
		assert.ElementsMatch(t, []string{"tool-a", "tool-b"}, names)
	})

	t.Run("Should list and find only allowed MCP tools when allowlist set", func(t *testing.T) {
		srv := makeToolsServer(t, []mcp.ToolDefinition{
			{Name: "x-search", Description: "X", MCPName: "mcp1"},
			{Name: "y-analyze", Description: "Y", MCPName: "mcp2"},
		})
		defer srv.Close()

		client := mcp.NewProxyClient(srv.URL, 2*time.Second)
		reg := NewToolRegistry(ToolRegistryConfig{
			ProxyClient:     client,
			CacheTTL:        1 * time.Millisecond,
			AllowedMCPNames: []string{"mcp2"},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tools, err := reg.ListAll(ctx)
		require.NoError(t, err)
		names := namesOf(tools)
		assert.ElementsMatch(t, []string{"y-analyze"}, names)

		// Verify tools through ListAll results instead of using internal Find method
		foundYAnalyze := false
		foundXSearch := false
		for _, tool := range tools {
			if tool.Name() == "y-analyze" {
				foundYAnalyze = true
			}
			if tool.Name() == "x-search" {
				foundXSearch = true
			}
		}
		assert.True(t, foundYAnalyze, "expected to find allowed tool y-analyze")
		assert.False(t, foundXSearch, "did not expect to find filtered tool x-search")
	})
}

func makeToolsServer(t *testing.T, defs []mcp.ToolDefinition) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/tools" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(toolsResponse{Tools: defs})
	}))
}

func namesOf(ts []Tool) []string {
	out := make([]string, len(ts))
	for i := range ts {
		out[i] = ts[i].Name()
	}
	return out
}
