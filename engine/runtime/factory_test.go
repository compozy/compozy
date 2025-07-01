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
		assert.Error(t, err)
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

		// This will currently use the existing Manager until BunManager is implemented
		rt, err := factory.CreateRuntime(ctx, config)

		require.NoError(t, err)
		assert.NotNil(t, rt)

		// Verify it implements the Runtime interface
		assert.Implements(t, (*runtime.Runtime)(nil), rt)
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
		assert.Error(t, err)
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
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime type: python")
	})
}

func TestNewDefaultFactory(t *testing.T) {
	t.Run("Should create factory with project root", func(t *testing.T) {
		projectRoot := "/test/project"
		factory := runtime.NewDefaultFactory(projectRoot)

		assert.NotNil(t, factory)

		// Verify it implements the Factory interface
		assert.Implements(t, (*runtime.Factory)(nil), factory)
	})
}
