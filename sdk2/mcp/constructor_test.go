package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enginemcp "github.com/compozy/compozy/engine/mcp"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

func TestNew_MinimalStdioConfig(t *testing.T) {
	t.Run("Should create valid stdio MCP with command", func(t *testing.T) {
		cfg, err := New(t.Context(), "filesystem",
			WithCommand("mcp-server-filesystem"),
		)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "filesystem", cfg.ID)
		assert.Equal(t, "mcp-server-filesystem", cfg.Command)
		assert.Equal(t, "filesystem", cfg.Resource)
		assert.Equal(t, mcpproxy.TransportStdio, cfg.Transport)
	})
}

func TestNew_MinimalHTTPConfig(t *testing.T) {
	t.Run("Should create valid HTTP MCP with URL", func(t *testing.T) {
		cfg, err := New(t.Context(), "github",
			WithURL("https://api.github.com/mcp"),
		)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "github", cfg.ID)
		assert.Equal(t, "https://api.github.com/mcp", cfg.URL)
		assert.Equal(t, mcpproxy.TransportSSE, cfg.Transport)
	})
}

func TestNew_FullStdioConfig(t *testing.T) {
	t.Run("Should create fully configured stdio MCP", func(t *testing.T) {
		cfg, err := New(t.Context(), "filesystem",
			WithCommand("mcp-server-filesystem"),
			WithArgs([]string{"--root", "/data"}),
			WithEnv(map[string]string{
				"LOG_LEVEL": "debug",
				"ROOT_DIR":  "/workspace",
			}),
			WithStartTimeout(30*time.Second),
			WithMaxSessions(5),
			WithProto("2025-03-26"),
		)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "filesystem", cfg.ID)
		assert.Equal(t, "mcp-server-filesystem", cfg.Command)
		assert.Equal(t, []string{"--root", "/data"}, cfg.Args)
		assert.Equal(t, "debug", cfg.Env["LOG_LEVEL"])
		assert.Equal(t, "/workspace", cfg.Env["ROOT_DIR"])
		assert.Equal(t, 30*time.Second, cfg.StartTimeout)
		assert.Equal(t, 5, cfg.MaxSessions)
		assert.Equal(t, "2025-03-26", cfg.Proto)
	})
}

func TestNew_FullHTTPConfig(t *testing.T) {
	t.Run("Should create fully configured HTTP MCP", func(t *testing.T) {
		cfg, err := New(t.Context(), "github",
			WithURL("https://api.github.com/mcp"),
			WithHeaders(map[string]string{
				"Authorization": "Bearer token123",
				"X-Custom":      "value",
			}),
			WithTransport(mcpproxy.TransportStreamableHTTP),
			WithMaxSessions(10),
			WithProto("2025-03-26"),
		)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "github", cfg.ID)
		assert.Equal(t, "https://api.github.com/mcp", cfg.URL)
		assert.Equal(t, "Bearer token123", cfg.Headers["Authorization"])
		assert.Equal(t, "value", cfg.Headers["X-Custom"])
		assert.Equal(t, mcpproxy.TransportStreamableHTTP, cfg.Transport)
		assert.Equal(t, 10, cfg.MaxSessions)
	})
}

func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		opts    []Option
		wantErr string
	}{
		{
			name:    "Should fail with empty ID",
			id:      "",
			opts:    []Option{WithCommand("mcp-server")},
			wantErr: "id is invalid",
		},
		{
			name:    "Should fail with invalid ID characters",
			id:      "bad id",
			opts:    []Option{WithCommand("mcp-server")},
			wantErr: "id is invalid",
		},
		{
			name:    "Should fail without command or URL",
			id:      "test-mcp",
			opts:    []Option{},
			wantErr: "either command or url must be configured",
		},
		{
			name: "Should fail with both command and URL",
			id:   "test-mcp",
			opts: []Option{
				WithCommand("mcp-server"),
				WithURL("https://example.com"),
			},
			wantErr: "configure either command or url",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := New(t.Context(), tt.id, tt.opts...)
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNew_NilContext(t *testing.T) {
	t.Run("Should fail with nil context", func(t *testing.T) {
		var nilCtx context.Context
		cfg, err := New(nilCtx, "test-mcp", WithCommand("mcp-server"))
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "context is required")
	})
}

func TestNew_DeepCopy(t *testing.T) {
	t.Run("Should return deep copy of config", func(t *testing.T) {
		cfg1, err := New(t.Context(), "filesystem",
			WithCommand("mcp-server"),
			WithArgs([]string{"--root", "/data"}),
			WithEnv(map[string]string{"KEY": "value"}),
		)
		require.NoError(t, err)
		cfg1.Args[0] = "modified"
		cfg1.Env["KEY"] = "modified"
		cfg2, err := New(t.Context(), "filesystem",
			WithCommand("mcp-server"),
			WithArgs([]string{"--root", "/data"}),
			WithEnv(map[string]string{"KEY": "value"}),
		)
		require.NoError(t, err)
		assert.NotEqual(t, cfg1.Args[0], cfg2.Args[0])
		assert.NotEqual(t, cfg1.Env["KEY"], cfg2.Env["KEY"])
	})
}

func TestNew_WhitespaceTrimming(t *testing.T) {
	t.Run("Should trim whitespace from ID", func(t *testing.T) {
		cfg, err := New(t.Context(), "  filesystem  ",
			WithCommand("mcp-server"),
		)
		require.NoError(t, err)
		assert.Equal(t, "filesystem", cfg.ID)
	})
}

func TestNew_SetDefaultsApplied(t *testing.T) {
	t.Run("Should apply defaults for stdio transport", func(t *testing.T) {
		cfg, err := New(t.Context(), "filesystem",
			WithCommand("mcp-server"),
		)
		require.NoError(t, err)
		assert.Equal(t, "filesystem", cfg.Resource)
		assert.Equal(t, enginemcp.DefaultProtocolVersion, cfg.Proto)
		assert.Equal(t, mcpproxy.TransportStdio, cfg.Transport)
	})
	t.Run("Should apply defaults for HTTP transport", func(t *testing.T) {
		cfg, err := New(t.Context(), "github",
			WithURL("https://api.github.com"),
		)
		require.NoError(t, err)
		assert.Equal(t, "github", cfg.Resource)
		assert.Equal(t, enginemcp.DefaultProtocolVersion, cfg.Proto)
		assert.Equal(t, mcpproxy.TransportSSE, cfg.Transport)
	})
}

func TestNew_BuildErrorAggregation(t *testing.T) {
	t.Run("Should aggregate multiple validation errors", func(t *testing.T) {
		cfg, err := New(t.Context(), "",
			WithCommand(""),
			WithURL(""),
		)
		require.Error(t, err)
		assert.Nil(t, cfg)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		require.True(t, errors.Is(err, buildErr))
		assert.GreaterOrEqual(t, len(buildErr.Errors), 2)
	})
}
