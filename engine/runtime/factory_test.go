package runtime_test

import (
	"context"
	"os"
	"testing"

	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultFactory_CreateRuntime(t *testing.T) {
	t.Run("Should create runtime with nil config error", func(t *testing.T) {
		factory := runtime.NewDefaultFactory("/test/project")
		ctx := context.Background()

		rt, err := factory.CreateRuntime(ctx, nil)

		assert.Nil(t, rt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "runtime config must not be nil")
	})

	t.Run("Should create runtime with default (Bun) type", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir, err := os.MkdirTemp("", "runtime-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		factory := runtime.NewDefaultFactory(tmpDir)
		ctx := context.Background()
		config := &runtime.Config{}

		// This creates a BunManager instance for the default runtime type
		rt, err := factory.CreateRuntime(ctx, config)

		require.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("Should create runtime with explicit Bun type", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir, err := os.MkdirTemp("", "runtime-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		factory := runtime.NewDefaultFactory(tmpDir)
		ctx := context.Background()
		config := &runtime.Config{
			RuntimeType: runtime.RuntimeTypeBun,
		}

		rt, err := factory.CreateRuntime(ctx, config)

		require.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("Should return error for Node.js runtime (not yet implemented)", func(t *testing.T) {
		factory := runtime.NewDefaultFactory("/test/project")
		ctx := context.Background()
		config := &runtime.Config{
			RuntimeType: runtime.RuntimeTypeNode,
		}

		rt, err := factory.CreateRuntime(ctx, config)

		assert.Nil(t, rt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "node.js runtime not yet implemented")
	})

	t.Run("Should return error for unsupported runtime type", func(t *testing.T) {
		factory := runtime.NewDefaultFactory("/test/project")
		ctx := context.Background()
		config := &runtime.Config{
			RuntimeType: "python",
		}

		rt, err := factory.CreateRuntime(ctx, config)

		assert.Nil(t, rt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime type: python")
		assert.Contains(t, err.Error(), "supported types:")
	})

	t.Run("Should validate runtime type before processing", func(t *testing.T) {
		factory := runtime.NewDefaultFactory("/test/project")
		ctx := context.Background()

		// Note: empty string defaults to Bun, so we skip it
		invalidTypes := []string{"invalid", "PYTHON", "ruby", "java", "deno"}

		for _, invalidType := range invalidTypes {
			config := &runtime.Config{
				RuntimeType: invalidType,
			}

			rt, err := factory.CreateRuntime(ctx, config)

			assert.Nil(t, rt, "Runtime should be nil for invalid type: %s", invalidType)
			require.Error(t, err, "Should return error for invalid type: %s", invalidType)
			assert.Contains(
				t,
				err.Error(),
				"unsupported runtime type",
				"Error should mention unsupported type for: %s",
				invalidType,
			)
		}
	})
}
