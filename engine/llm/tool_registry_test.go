package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	nativeuser "github.com/compozy/compozy/engine/tool/nativeuser"
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

		client := mcp.NewProxyClient(t.Context(), srv.URL, 2*time.Second)
		reg, err := NewToolRegistry(
			t.Context(),
			ToolRegistryConfig{ProxyClient: client, CacheTTL: 1 * time.Millisecond},
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
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

		client := mcp.NewProxyClient(t.Context(), srv.URL, 2*time.Second)
		reg, err := NewToolRegistry(t.Context(), ToolRegistryConfig{
			ProxyClient:     client,
			CacheTTL:        1 * time.Millisecond,
			AllowedMCPNames: []string{"mcp2"},
		})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		tools, err := reg.ListAll(ctx)
		require.NoError(t, err)
		names := namesOf(tools)
		assert.ElementsMatch(t, []string{"y-analyze"}, names)

		// Verify filtering via ListAll; dedicated deny-list tests cover Find semantics.
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

	t.Run("Should exclude denied MCP tools when deny list provided", func(t *testing.T) {
		srv := makeToolsServer(t, []mcp.ToolDefinition{
			{Name: "alpha", Description: "A", MCPName: "mcp1"},
			{Name: "beta", Description: "B", MCPName: "mcp2"},
		})
		defer srv.Close()

		client := mcp.NewProxyClient(t.Context(), srv.URL, 2*time.Second)
		reg, err := NewToolRegistry(t.Context(), ToolRegistryConfig{
			ProxyClient:    client,
			CacheTTL:       1 * time.Millisecond,
			DeniedMCPNames: []string{"mcp2"},
		})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		tools, err := reg.ListAll(ctx)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"alpha"}, namesOf(tools))

		_, ok := reg.Find(ctx, "beta")
		assert.False(t, ok, "expected beta to be filtered by deny list")
	})

	t.Run("Should prefer deny list over allow list", func(t *testing.T) {
		srv := makeToolsServer(t, []mcp.ToolDefinition{
			{Name: "alpha", Description: "A", MCPName: "mcp1"},
			{Name: "beta", Description: "B", MCPName: "mcp2"},
		})
		defer srv.Close()

		client := mcp.NewProxyClient(t.Context(), srv.URL, 2*time.Second)
		reg, err := NewToolRegistry(t.Context(), ToolRegistryConfig{
			ProxyClient:     client,
			CacheTTL:        1 * time.Millisecond,
			AllowedMCPNames: []string{"mcp1", "mcp2"},
			DeniedMCPNames:  []string{"mcp1"},
		})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		tools, err := reg.ListAll(ctx)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"beta"}, namesOf(tools))

		_, okAlpha := reg.Find(ctx, "alpha")
		assert.False(t, okAlpha, "expected alpha to be denied even though allowlisted")
		_, okBeta := reg.Find(ctx, "beta")
		assert.True(t, okBeta, "expected beta to remain accessible")
	})
}

func TestToolRegistry_FindBackgroundRefresh(t *testing.T) {
	initial := []mcp.ToolDefinition{
		{Name: "tool-a", Description: "A", MCPName: "mcp1"},
	}
	dynamic := newDynamicToolsServer(t, initial)
	clock := newFakeClock(time.Unix(0, 0))

	client := mcp.NewProxyClient(t.Context(), dynamic.URL(), 2*time.Second)
	reg, err := NewToolRegistry(t.Context(), ToolRegistryConfig{
		ProxyClient: client,
		CacheTTL:    1 * time.Minute,
	})
	require.NoError(t, err)
	regImpl := reg.(*toolRegistry)
	regImpl.now = clock.Now

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	tool, ok := reg.Find(ctx, "tool-a")
	require.True(t, ok)
	require.Equal(t, "tool-a", tool.Name())
	require.Eventually(t, func() bool {
		return dynamic.Hits() == 1
	}, time.Second, 10*time.Millisecond)

	clock.Advance(2 * time.Minute)
	dynamic.Set([]mcp.ToolDefinition{
		{Name: "tool-a", Description: "A", MCPName: "mcp1"},
		{Name: "tool-b", Description: "B", MCPName: "mcp2"},
	})

	tool, ok = reg.Find(ctx, "tool-a")
	require.True(t, ok)
	require.Equal(t, "tool-a", tool.Name())

	require.Eventually(t, func() bool {
		return dynamic.Hits() >= 2
	}, time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		tl, found := reg.Find(ctx, "tool-b")
		return found && tl.Name() == "tool-b"
	}, time.Second, 10*time.Millisecond)
}

func TestToolRegistry_FindRefreshesOnStaleMiss(t *testing.T) {
	initial := []mcp.ToolDefinition{
		{Name: "tool-a", Description: "A", MCPName: "mcp1"},
	}
	dynamic := newDynamicToolsServer(t, initial)
	clock := newFakeClock(time.Unix(0, 0))

	client := mcp.NewProxyClient(t.Context(), dynamic.URL(), 2*time.Second)
	reg, err := NewToolRegistry(t.Context(), ToolRegistryConfig{
		ProxyClient: client,
		CacheTTL:    30 * time.Second,
	})
	require.NoError(t, err)
	regImpl := reg.(*toolRegistry)
	regImpl.now = clock.Now

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	_, ok := reg.Find(ctx, "tool-a")
	require.True(t, ok)
	require.Equal(t, 1, dynamic.Hits())

	clock.Advance(1 * time.Minute)
	dynamic.Set([]mcp.ToolDefinition{
		{Name: "tool-b", Description: "B", MCPName: "mcp2"},
	})

	tool, ok := reg.Find(ctx, "tool-b")
	require.True(t, ok, "expected synchronous refresh to find new tool")
	require.Equal(t, "tool-b", tool.Name())
	require.GreaterOrEqual(t, dynamic.Hits(), 2)
}

func buildNativeToolConfig() *tool.Config {
	return &tool.Config{
		ID:             "native-tool",
		Description:    "Native tool",
		Implementation: tool.ImplementationNative,
		InputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		},
		OutputSchema: &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"result": map[string]any{"type": "string"},
			},
			"required": []string{"result"},
		},
		Config: &core.Input{"sample": true},
	}
}

func TestNativeToolAdapter(t *testing.T) {
	t.Run("Should execute native handler successfully", func(t *testing.T) {
		nativeuser.Reset()
		t.Cleanup(nativeuser.Reset)
		handler := func(_ context.Context, input map[string]any, cfg map[string]any) (map[string]any, error) {
			assert.Equal(t, map[string]any{"sample": true}, cfg)
			assert.Equal(t, "alice", input["name"])
			return map[string]any{"result": "ok"}, nil
		}
		require.NoError(t, nativeuser.Register("native-tool", handler))
		adapter := NewNativeToolAdapter(buildNativeToolConfig())
		output, err := adapter.Call(t.Context(), `{"name":"alice"}`)
		require.NoError(t, err)
		assert.Contains(t, output, "\"result\":\"ok\"")
	})

	t.Run("Should validate input schema", func(t *testing.T) {
		nativeuser.Reset()
		t.Cleanup(nativeuser.Reset)
		require.NoError(
			t,
			nativeuser.Register(
				"native-tool",
				func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
					return map[string]any{"result": "ok"}, nil
				},
			),
		)
		adapter := NewNativeToolAdapter(buildNativeToolConfig())
		_, err := adapter.Call(t.Context(), `{"unexpected":true}`)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_TOOL_INPUT", coreErr.Code)
	})

	t.Run("Should recover from panic", func(t *testing.T) {
		nativeuser.Reset()
		t.Cleanup(nativeuser.Reset)
		require.NoError(
			t,
			nativeuser.Register(
				"native-tool",
				func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
					panic("boom")
				},
			),
		)
		adapter := NewNativeToolAdapter(buildNativeToolConfig())
		_, err := adapter.Call(t.Context(), `{"name":"alice"}`)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "TOOL_EXECUTION_ERROR", coreErr.Code)
	})

	t.Run("Should validate output schema", func(t *testing.T) {
		nativeuser.Reset()
		t.Cleanup(nativeuser.Reset)
		require.NoError(
			t,
			nativeuser.Register(
				"native-tool",
				func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
					return map[string]any{"unexpected": true}, nil
				},
			),
		)
		adapter := NewNativeToolAdapter(buildNativeToolConfig())
		_, err := adapter.Call(t.Context(), `{"name":"alice"}`)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "TOOL_INVALID_OUTPUT", coreErr.Code)
	})
}

func TestToolRegistry_InvalidateCacheClearsIndex(t *testing.T) {
	dynamic := newDynamicToolsServer(t, []mcp.ToolDefinition{
		{Name: "alpha", Description: "A", MCPName: "mcp-one"},
	})

	client := mcp.NewProxyClient(t.Context(), dynamic.URL(), 2*time.Second)
	reg, err := NewToolRegistry(t.Context(), ToolRegistryConfig{
		ProxyClient: client,
		CacheTTL:    time.Minute,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	tool, ok := reg.Find(ctx, "alpha")
	require.True(t, ok, "expected to resolve initial MCP tool")
	require.Equal(t, "alpha", tool.Name())

	dynamic.Set([]mcp.ToolDefinition{
		{Name: "beta", Description: "B", MCPName: "mcp-one"},
	})
	reg.InvalidateCache(ctx)

	tool, ok = reg.Find(ctx, "beta")
	require.True(t, ok, "expected lookup to find refreshed tool after cache invalidation")
	require.Equal(t, "beta", tool.Name())
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

type dynamicToolsServer struct {
	srv  *httptest.Server
	mu   sync.Mutex
	defs []mcp.ToolDefinition
	hits int
}

func newDynamicToolsServer(t *testing.T, defs []mcp.ToolDefinition) *dynamicToolsServer {
	server := &dynamicToolsServer{
		defs: append([]mcp.ToolDefinition(nil), defs...),
	}
	server.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/tools" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		server.mu.Lock()
		server.hits++
		current := append([]mcp.ToolDefinition(nil), server.defs...)
		server.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(toolsResponse{Tools: current})
	}))
	t.Cleanup(server.Close)
	return server
}

func (d *dynamicToolsServer) URL() string {
	return d.srv.URL
}

func (d *dynamicToolsServer) Close() {
	d.srv.Close()
}

func (d *dynamicToolsServer) Set(defs []mcp.ToolDefinition) {
	d.mu.Lock()
	d.defs = append([]mcp.ToolDefinition(nil), defs...)
	d.mu.Unlock()
}

func (d *dynamicToolsServer) Hits() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.hits
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}
