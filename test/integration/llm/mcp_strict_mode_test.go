package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRuntime implements runtime.Runtime with inert methods for construction-time tests
type stubRuntime struct{}

func (s *stubRuntime) ExecuteTool(
	_ context.Context,
	_ string,
	_ core.ID,
	_ *core.Input,
	_ *core.Input,
	_ core.EnvMap,
) (*core.Output, error) {
	return &core.Output{"ok": true}, nil
}

func (s *stubRuntime) ExecuteToolWithTimeout(
	_ context.Context,
	_ string,
	_ core.ID,
	_ *core.Input,
	_ *core.Input,
	_ core.EnvMap,
	_ time.Duration,
) (*core.Output, error) {
	return &core.Output{"ok": true}, nil
}
func (s *stubRuntime) GetGlobalTimeout() time.Duration { return 0 }

// NOTE: We avoid using production runtime here; NewService only needs a Runtime value to construct.

func TestStrictMode_MCPRegistration(t *testing.T) {
	t.Run("Should error when strict mode enabled and MCP registration fails", func(t *testing.T) {
		// Proxy that fails admin/mcps
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/admin/mcps" && r.Method == http.MethodPost {
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		// Use context with timeout to prevent test hangs
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		ag := &agent.Config{
			ID:            "strict-agent",
			LLMProperties: agent.LLMProperties{MCPs: []mcp.Config{{ID: "fs", URL: "http://example"}}},
		}

		// Use a stub runtime; orchestrator won't execute tools in this test
		var rt runtime.Runtime = &stubRuntime{}

		_, err := llm.NewService(ctx, rt, ag,
			llm.WithProxyURL(srv.URL),
			llm.WithStrictMCPRegistration(true),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to register")
	})

	t.Run("Should not error when strict mode disabled and MCP registration fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/admin/mcps" && r.Method == http.MethodPost {
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		ag := &agent.Config{
			ID:            "non-strict-agent",
			LLMProperties: agent.LLMProperties{MCPs: []mcp.Config{{ID: "fs", URL: "http://example"}}},
		}
		var rt runtime.Runtime = &stubRuntime{}

		svc, err := llm.NewService(ctx, rt, ag,
			llm.WithProxyURL(srv.URL),
			llm.WithStrictMCPRegistration(false),
		)
		require.NoError(t, err)
		require.NotNil(t, svc)
		_ = svc.Close()
	})
}
