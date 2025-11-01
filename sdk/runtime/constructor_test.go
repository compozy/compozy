package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engineruntime "github.com/compozy/compozy/engine/runtime"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
)

func TestNew_MinimalConfig(t *testing.T) {
	t.Run("Should create runtime config with bun type", func(t *testing.T) {
		cfg, err := New(t.Context(), "bun")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, engineruntime.RuntimeTypeBun, cfg.RuntimeType)
	})

	t.Run("Should create runtime config with node type", func(t *testing.T) {
		cfg, err := New(t.Context(), "node")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, engineruntime.RuntimeTypeNode, cfg.RuntimeType)
	})

	t.Run("Should normalize runtime type to lowercase", func(t *testing.T) {
		cfg, err := New(t.Context(), "BUN")
		require.NoError(t, err)
		assert.Equal(t, engineruntime.RuntimeTypeBun, cfg.RuntimeType)
	})

	t.Run("Should trim whitespace from runtime type", func(t *testing.T) {
		cfg, err := New(t.Context(), "  node  ")
		require.NoError(t, err)
		assert.Equal(t, engineruntime.RuntimeTypeNode, cfg.RuntimeType)
	})
}

func TestNew_FullConfig(t *testing.T) {
	t.Run("Should apply all options correctly", func(t *testing.T) {
		nativeTools := &engineruntime.NativeToolsConfig{
			CallAgents:    true,
			CallWorkflows: true,
		}
		cfg, err := New(t.Context(), "bun",
			WithEntrypointPath("./tools/main.ts"),
			WithBunPermissions([]string{"--allow-read", "--allow-net"}),
			WithMaxMemoryMB(512),
			WithNativeTools(nativeTools),
			WithEnvironment("production"),
		)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, engineruntime.RuntimeTypeBun, cfg.RuntimeType)
		assert.Equal(t, "./tools/main.ts", cfg.EntrypointPath)
		assert.Equal(t, []string{"--allow-read", "--allow-net"}, cfg.BunPermissions)
		assert.Equal(t, 512, cfg.MaxMemoryMB)
		assert.Equal(t, "production", cfg.Environment)
		assert.NotNil(t, cfg.NativeTools)
		assert.True(t, cfg.NativeTools.CallAgents)
		assert.True(t, cfg.NativeTools.CallWorkflows)
	})
}

func TestNew_ValidationErrors(t *testing.T) {
	t.Run("Should fail with nil context", func(t *testing.T) {
		var nilCtx context.Context
		cfg, err := New(nilCtx, "bun")
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "context is required")
	})

	t.Run("Should fail with empty runtime type", func(t *testing.T) {
		cfg, err := New(t.Context(), "")
		require.Error(t, err)
		assert.Nil(t, cfg)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		assert.Contains(t, err.Error(), "runtime type")
	})

	t.Run("Should fail with invalid runtime type", func(t *testing.T) {
		cfg, err := New(t.Context(), "python")
		require.Error(t, err)
		assert.Nil(t, cfg)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		assert.Contains(t, err.Error(), "not supported")
	})

	t.Run("Should fail with whitespace-only runtime type", func(t *testing.T) {
		cfg, err := New(t.Context(), "   ")
		require.Error(t, err)
		assert.Nil(t, cfg)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
}

func TestNew_DeepCopy(t *testing.T) {
	t.Run("Should return independent copy", func(t *testing.T) {
		permissions := []string{"--allow-read"}
		cfg1, err := New(t.Context(), "bun",
			WithBunPermissions(permissions),
		)
		require.NoError(t, err)

		cfg2, err := New(t.Context(), "bun",
			WithBunPermissions(permissions),
		)
		require.NoError(t, err)

		cfg1.BunPermissions[0] = "--allow-write"

		assert.Equal(t, "--allow-read", cfg2.BunPermissions[0])
		assert.NotEqual(t, cfg1.BunPermissions[0], cfg2.BunPermissions[0])
	})

	t.Run("Should deep copy native tools config", func(t *testing.T) {
		tools := &engineruntime.NativeToolsConfig{
			CallAgents: true,
		}
		cfg, err := New(t.Context(), "bun",
			WithNativeTools(tools),
		)
		require.NoError(t, err)

		tools.CallAgents = false

		assert.True(t, cfg.NativeTools.CallAgents)
	})
}

func TestNew_EntrypointValidation(t *testing.T) {
	t.Run("Should allow empty entrypoint", func(t *testing.T) {
		cfg, err := New(t.Context(), "bun")
		require.NoError(t, err)
		assert.Empty(t, cfg.EntrypointPath)
	})

	t.Run("Should trim entrypoint whitespace", func(t *testing.T) {
		cfg, err := New(t.Context(), "bun",
			WithEntrypointPath("  ./main.ts  "),
		)
		require.NoError(t, err)
		assert.Equal(t, "./main.ts", cfg.EntrypointPath)
	})

	t.Run("Should accept valid entrypoint path", func(t *testing.T) {
		cfg, err := New(t.Context(), "bun",
			WithEntrypointPath("./tools/runtime.ts"),
		)
		require.NoError(t, err)
		assert.Equal(t, "./tools/runtime.ts", cfg.EntrypointPath)
	})
}

func TestNew_DefaultValues(t *testing.T) {
	t.Run("Should apply default config values", func(t *testing.T) {
		cfg, err := New(t.Context(), "bun")
		require.NoError(t, err)
		require.NotNil(t, cfg)

		defaultCfg := engineruntime.DefaultConfig()
		assert.Equal(t, defaultCfg.BackoffInitialInterval, cfg.BackoffInitialInterval)
		assert.Equal(t, defaultCfg.BackoffMaxInterval, cfg.BackoffMaxInterval)
		assert.Equal(t, defaultCfg.BackoffMaxElapsedTime, cfg.BackoffMaxElapsedTime)
		assert.Equal(t, defaultCfg.ToolExecutionTimeout, cfg.ToolExecutionTimeout)
		assert.Equal(t, defaultCfg.MaxMemoryMB, cfg.MaxMemoryMB)
		assert.Equal(t, defaultCfg.MaxStderrCaptureSize, cfg.MaxStderrCaptureSize)
	})
}

func TestNew_NodeRuntime(t *testing.T) {
	t.Run("Should create node runtime config", func(t *testing.T) {
		cfg, err := New(t.Context(), "node",
			WithNodeOptions([]string{"--max-old-space-size=4096"}),
			WithEntrypointPath("./main.js"),
		)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, engineruntime.RuntimeTypeNode, cfg.RuntimeType)
		assert.Equal(t, []string{"--max-old-space-size=4096"}, cfg.NodeOptions)
		assert.Equal(t, "./main.js", cfg.EntrypointPath)
	})
}
