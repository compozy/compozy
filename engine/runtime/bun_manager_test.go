package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBunManager(t *testing.T) {
	t.Run("Should create BunManager when Bun is available", func(t *testing.T) {
		// Skip if Bun is not available
		if !isBunAvailable() {
			t.Skip("Bun is not available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		bm, err := runtime.NewBunManager(ctx, tmpDir)

		require.NoError(t, err)
		assert.NotNil(t, bm)

		// Verify it implements the Runtime interface
		assert.Implements(t, (*runtime.Runtime)(nil), bm)
	})

	t.Run("Should return error when Bun is not available", func(t *testing.T) {
		if isBunAvailable() {
			t.Skip("Bun is available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		bm, err := runtime.NewBunManager(ctx, tmpDir)

		assert.Nil(t, bm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bun executable not found")
	})

	t.Run("Should create worker file during initialization", func(t *testing.T) {
		if !isBunAvailable() {
			t.Skip("Bun is not available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		_, err = runtime.NewBunManager(ctx, tmpDir)
		require.NoError(t, err)

		// Check that worker file was created
		storeDir := core.GetStoreDir(tmpDir)
		workerPath := filepath.Join(storeDir, "bun_worker.ts")
		_, err = os.Stat(workerPath)
		assert.NoError(t, err, "Worker file should be created")
	})

	t.Run("Should apply configuration options", func(t *testing.T) {
		if !isBunAvailable() {
			t.Skip("Bun is not available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		config := &runtime.Config{
			RuntimeType:          runtime.RuntimeTypeBun,
			EntrypointPath:       "./custom-tools.ts",
			ToolExecutionTimeout: 30 * time.Second,
			BunPermissions: []string{
				"--allow-read",
				"--allow-net",
			},
		}

		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		// Verify timeout configuration
		assert.Equal(t, 30*time.Second, bm.GetGlobalTimeout())
	})
}

func TestBunManager_ExecuteTool(t *testing.T) {
	if !isBunAvailable() {
		t.Skip("Bun is not available, skipping test")
	}

	t.Run("Should validate tool execution inputs", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		bm, err := runtime.NewBunManager(ctx, tmpDir)
		require.NoError(t, err)

		toolExecID, _ := core.NewID()

		// Test empty tool ID
		_, err = bm.ExecuteTool(ctx, "", toolExecID, &core.Input{}, core.EnvMap{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool_id cannot be empty")

		// Test invalid tool ID with directory traversal
		_, err = bm.ExecuteTool(ctx, "../malicious", toolExecID, &core.Input{}, core.EnvMap{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory traversal patterns")

		// Test invalid tool ID with invalid characters
		_, err = bm.ExecuteTool(ctx, "tool@invalid", toolExecID, &core.Input{}, core.EnvMap{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters")
	})

	t.Run("Should handle tool execution timeout", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a simple entrypoint with a slow tool
		entrypointContent := `
export async function slow_tool(input: any) {
    await new Promise(resolve => setTimeout(resolve, 2000));
    return { result: "completed" };
}
`
		err = os.WriteFile(tmpDir+"/tools.ts", []byte(entrypointContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		toolExecID, _ := core.NewID()

		// Execute with very short timeout
		_, err = bm.ExecuteToolWithTimeout(
			ctx,
			"slow_tool",
			toolExecID,
			&core.Input{},
			core.EnvMap{},
			100*time.Millisecond,
		)
		assert.Error(t, err)
		// Note: The specific timeout error will depend on Bun's behavior
	})

	t.Run("Should execute simple tool successfully", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a simple entrypoint with a test tool
		entrypointContent := `
export async function test_tool(input: any) {
    return { message: "Hello from Bun!", input: input };
}
`
		err = os.WriteFile(tmpDir+"/tools.ts", []byte(entrypointContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		toolExecID, _ := core.NewID()
		input := &core.Input{"test": "data"}

		result, err := bm.ExecuteTool(ctx, "test_tool", toolExecID, input, core.EnvMap{})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify the response structure
		assert.Contains(t, *result, "message")
		assert.Equal(t, "Hello from Bun!", (*result)["message"])
	})

	t.Run("Should handle tool not found error", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create entrypoint with no tools
		entrypointContent := `
// Empty entrypoint
export function dummy() {}
`
		err = os.WriteFile(tmpDir+"/tools.ts", []byte(entrypointContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		toolExecID, _ := core.NewID()

		_, err = bm.ExecuteTool(ctx, "nonexistent_tool", toolExecID, &core.Input{}, core.EnvMap{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool execution failed")
	})

	t.Run("Should handle environment variables", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create tool that uses environment variables
		entrypointContent := `
export async function env_tool(input: any) {
    return {
        env_var: process.env.TEST_VAR,
        input: input
    };
}
`
		err = os.WriteFile(tmpDir+"/tools.ts", []byte(entrypointContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		toolExecID, _ := core.NewID()
		env := core.EnvMap{"TEST_VAR": "test_value"}

		result, err := bm.ExecuteTool(ctx, "env_tool", toolExecID, &core.Input{}, env)
		require.NoError(t, err)

		assert.Equal(t, "test_value", (*result)["env_var"])
	})
}

func TestBunManager_GetGlobalTimeout(t *testing.T) {
	t.Run("Should return configured timeout", func(t *testing.T) {
		if !isBunAvailable() {
			t.Skip("Bun is not available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		timeout := 45 * time.Second
		config := &runtime.Config{
			ToolExecutionTimeout: timeout,
		}

		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		assert.Equal(t, timeout, bm.GetGlobalTimeout())
	})
}

func TestBunManager_WorkerFileGeneration(t *testing.T) {
	t.Run("Should generate worker file with correct entrypoint", func(t *testing.T) {
		if !isBunAvailable() {
			t.Skip("Bun is not available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		config := &runtime.Config{
			EntrypointPath: "./custom-entrypoint.ts",
		}

		_, err = runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		// Read the generated worker file
		storeDir := core.GetStoreDir(tmpDir)
		workerPath := filepath.Join(storeDir, "bun_worker.ts")
		content, err := os.ReadFile(workerPath)
		require.NoError(t, err)

		// Verify the entrypoint path was replaced and made relative to worker location
		assert.Contains(t, string(content), `import * as allExports from "../custom-entrypoint.ts"`)
		assert.Contains(t, string(content), "#!/usr/bin/env bun")
	})

	t.Run("Should use default entrypoint when not specified", func(t *testing.T) {
		if !isBunAvailable() {
			t.Skip("Bun is not available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		_, err = runtime.NewBunManager(ctx, tmpDir)
		require.NoError(t, err)

		// Read the generated worker file
		storeDir := core.GetStoreDir(tmpDir)
		workerPath := filepath.Join(storeDir, "bun_worker.ts")
		content, err := os.ReadFile(workerPath)
		require.NoError(t, err)

		// Verify the default entrypoint path is used and made relative to worker location
		assert.Contains(t, string(content), `import * as allExports from "../tools.ts"`)
	})
}

// Helper function to check if Bun is available
func isBunAvailable() bool {
	// Use the same logic as the runtime package
	return runtime.IsBunAvailable()
}
