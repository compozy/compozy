package runtime_test

import (
	"testing"

	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
)

func TestRuntimeConfigOptions(t *testing.T) {
	t.Run("Should apply WithConfig option", func(t *testing.T) {
		baseConfig := &runtime.Config{
			RuntimeType:    runtime.RuntimeTypeBun,
			EntrypointPath: "/test/entrypoint.ts",
			BunPermissions: []string{"--allow-read"},
		}

		config := runtime.DefaultConfig()
		option := runtime.WithConfig(baseConfig)
		option(config)

		assert.Equal(t, runtime.RuntimeTypeBun, config.RuntimeType)
		assert.Equal(t, "/test/entrypoint.ts", config.EntrypointPath)
		assert.Equal(t, []string{"--allow-read"}, config.BunPermissions)
	})

	t.Run("Should handle nil WithConfig", func(t *testing.T) {
		config := runtime.DefaultConfig()
		originalType := config.RuntimeType

		option := runtime.WithConfig(nil)
		option(config)

		// Config should remain unchanged
		assert.Equal(t, originalType, config.RuntimeType)
	})

	t.Run("Should apply WithRuntimeType option", func(t *testing.T) {
		config := runtime.DefaultConfig()
		option := runtime.WithRuntimeType(runtime.RuntimeTypeNode)
		option(config)

		assert.Equal(t, runtime.RuntimeTypeNode, config.RuntimeType)
	})

	t.Run("Should apply WithEntrypointPath option", func(t *testing.T) {
		config := runtime.DefaultConfig()
		path := "/custom/entrypoint.ts"
		option := runtime.WithEntrypointPath(path)
		option(config)

		assert.Equal(t, path, config.EntrypointPath)
	})

	t.Run("Should apply WithBunPermissions option", func(t *testing.T) {
		config := runtime.DefaultConfig()
		perms := []string{"--allow-net", "--allow-read", "--allow-write"}
		option := runtime.WithBunPermissions(perms)
		option(config)

		assert.Equal(t, perms, config.BunPermissions)
	})

	t.Run("Should apply WithNodeOptions option", func(t *testing.T) {
		config := runtime.DefaultConfig()
		opts := []string{"--experimental-modules", "--no-warnings"}
		option := runtime.WithNodeOptions(opts)
		option(config)

		assert.Equal(t, opts, config.NodeOptions)
	})
}

func TestDefaultConfigRuntimeFields(t *testing.T) {
	t.Run("Should have Bun as default runtime type", func(t *testing.T) {
		config := runtime.DefaultConfig()

		assert.Equal(t, runtime.RuntimeTypeBun, config.RuntimeType)
		assert.NotEmpty(t, config.BunPermissions)
		assert.Contains(t, config.BunPermissions, "--allow-read")
	})
}

func TestTestConfigRuntimeFields(t *testing.T) {
	t.Run("Should have Bun as default runtime type for tests", func(t *testing.T) {
		config := runtime.TestConfig()

		assert.Equal(t, runtime.RuntimeTypeBun, config.RuntimeType)
		assert.NotEmpty(t, config.BunPermissions)
		assert.Contains(t, config.BunPermissions, "--allow-read")
	})
}
