package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeWithDefaults(t *testing.T) {
	t.Run("Should return default config when nil", func(t *testing.T) {
		result := runtime.MergeWithDefaults(nil)
		defaultConfig := runtime.DefaultConfig()

		assert.Equal(t, defaultConfig.BackoffInitialInterval, result.BackoffInitialInterval)
		assert.Equal(t, defaultConfig.BackoffMaxInterval, result.BackoffMaxInterval)
		assert.Equal(t, defaultConfig.BackoffMaxElapsedTime, result.BackoffMaxElapsedTime)
		assert.Equal(t, defaultConfig.WorkerFilePerm, result.WorkerFilePerm)
		assert.Equal(t, defaultConfig.ToolExecutionTimeout, result.ToolExecutionTimeout)
		assert.Equal(t, defaultConfig.RuntimeType, result.RuntimeType)
		assert.Equal(t, defaultConfig.BunPermissions, result.BunPermissions)
		assert.Equal(t, defaultConfig.Environment, result.Environment)
		assert.Equal(t, defaultConfig.EntrypointPath, result.EntrypointPath)
	})

	t.Run("Should preserve non-zero values and fill zero values with defaults", func(t *testing.T) {
		config := &runtime.Config{
			BackoffInitialInterval: 50 * time.Millisecond, // Non-zero, should be preserved
			BackoffMaxInterval:     0,                     // Zero, should get default
			BackoffMaxElapsedTime:  10 * time.Second,      // Non-zero, should be preserved
			WorkerFilePerm:         0,                     // Zero, should get default
			ToolExecutionTimeout:   30 * time.Second,      // Non-zero, should be preserved
			RuntimeType:            "custom",              // Non-empty, should be preserved
			BunPermissions:         nil,                   // Nil, should get default
			Environment:            "",                    // Empty, should get default
			EntrypointPath:         "./custom.ts",         // Non-empty, should be preserved
		}

		result := runtime.MergeWithDefaults(config)
		defaultConfig := runtime.DefaultConfig()

		// Non-zero values should be preserved
		assert.Equal(t, 50*time.Millisecond, result.BackoffInitialInterval)
		assert.Equal(t, 10*time.Second, result.BackoffMaxElapsedTime)
		assert.Equal(t, 30*time.Second, result.ToolExecutionTimeout)
		assert.Equal(t, "custom", result.RuntimeType)
		assert.Equal(t, "./custom.ts", result.EntrypointPath)

		// Zero values should get defaults
		assert.Equal(t, defaultConfig.BackoffMaxInterval, result.BackoffMaxInterval)
		assert.Equal(t, defaultConfig.WorkerFilePerm, result.WorkerFilePerm)
		assert.Equal(t, defaultConfig.BunPermissions, result.BunPermissions)
		assert.Equal(t, defaultConfig.Environment, result.Environment)
	})

	t.Run("Should preserve all non-zero values", func(t *testing.T) {
		config := &runtime.Config{
			BackoffInitialInterval: 200 * time.Millisecond,
			BackoffMaxInterval:     10 * time.Second,
			BackoffMaxElapsedTime:  60 * time.Second,
			WorkerFilePerm:         0644,
			ToolExecutionTimeout:   120 * time.Second,
			RuntimeType:            "node",
			BunPermissions:         []string{"--allow-write"},
			Environment:            "production",
			EntrypointPath:         "./prod.ts",
		}

		result := runtime.MergeWithDefaults(config)

		// All values should be preserved since none are zero
		assert.Equal(t, 200*time.Millisecond, result.BackoffInitialInterval)
		assert.Equal(t, 10*time.Second, result.BackoffMaxInterval)
		assert.Equal(t, 60*time.Second, result.BackoffMaxElapsedTime)
		assert.Equal(t, os.FileMode(0644), result.WorkerFilePerm)
		assert.Equal(t, 120*time.Second, result.ToolExecutionTimeout)
		assert.Equal(t, "node", result.RuntimeType)
		assert.Equal(t, []string{"--allow-write"}, result.BunPermissions)
		assert.Equal(t, "production", result.Environment)
		assert.Equal(t, "./prod.ts", result.EntrypointPath)
	})
}

func TestNewBunManager(t *testing.T) {
	t.Run("Should create BunManager when Bun is available", func(t *testing.T) {
		// Skip if Bun is not available
		if !isBunAvailable() {
			t.Skip("Bun is not available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := t.Context()
		bm, err := runtime.NewBunManager(ctx, tmpDir, nil)

		require.NoError(t, err)
		assert.NotNil(t, bm)
	})

	t.Run("Should return error when Bun is not available", func(t *testing.T) {
		if isBunAvailable() {
			t.Skip("Bun is available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := t.Context()
		bm, err := runtime.NewBunManager(ctx, tmpDir, nil)

		assert.Nil(t, bm)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bun executable not found")
	})

	t.Run("Should create worker file during initialization", func(t *testing.T) {
		if !isBunAvailable() {
			t.Skip("Bun is not available, skipping test")
		}

		tmpDir, err := os.MkdirTemp("", "bun-manager-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		ctx := t.Context()
		_, err = runtime.NewBunManager(ctx, tmpDir, nil)
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

		ctx := t.Context()
		config := &runtime.Config{
			RuntimeType:          runtime.RuntimeTypeBun,
			EntrypointPath:       "./custom-tools.ts",
			ToolExecutionTimeout: 30 * time.Second,
			BunPermissions: []string{
				"--allow-read",
				"--allow-net",
			},
		}

		bm, err := runtime.NewBunManager(ctx, tmpDir, config)
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

		ctx := t.Context()
		bm, err := runtime.NewBunManager(ctx, tmpDir, nil)
		require.NoError(t, err)

		toolExecID, _ := core.NewID()

		// Test empty tool ID
		_, err = bm.ExecuteTool(ctx, "", toolExecID, &core.Input{}, nil, core.EnvMap{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tool_id cannot be empty")

		// Test invalid tool ID with directory traversal
		_, err = bm.ExecuteTool(ctx, "../malicious", toolExecID, &core.Input{}, nil, core.EnvMap{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "directory traversal patterns")

		// Test invalid tool ID with invalid characters
		_, err = bm.ExecuteTool(ctx, "tool@invalid", toolExecID, &core.Input{}, nil, core.EnvMap{})
		require.Error(t, err)
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

		ctx := t.Context()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, config)
		require.NoError(t, err)

		toolExecID, _ := core.NewID()

		// Execute with very short timeout
		_, err = bm.ExecuteToolWithTimeout(
			ctx,
			"slow_tool",
			toolExecID,
			&core.Input{},
			nil,
			core.EnvMap{},
			100*time.Millisecond,
		)
		require.Error(t, err)
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

		ctx := t.Context()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, config)
		require.NoError(t, err)

		toolExecID, _ := core.NewID()
		input := &core.Input{"test": "data"}

		result, err := bm.ExecuteTool(ctx, "test_tool", toolExecID, input, nil, core.EnvMap{})
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

		ctx := t.Context()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, config)
		require.NoError(t, err)

		toolExecID, _ := core.NewID()

		_, err = bm.ExecuteTool(ctx, "nonexistent_tool", toolExecID, &core.Input{}, nil, core.EnvMap{})
		require.Error(t, err)
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

		ctx := t.Context()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, config)
		require.NoError(t, err)

		toolExecID, _ := core.NewID()
		env := core.EnvMap{"TEST_VAR": "test_value"}

		result, err := bm.ExecuteTool(ctx, "env_tool", toolExecID, &core.Input{}, nil, env)
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

		ctx := t.Context()
		timeout := 45 * time.Second
		config := &runtime.Config{
			ToolExecutionTimeout: timeout,
		}

		bm, err := runtime.NewBunManager(ctx, tmpDir, config)
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

		ctx := t.Context()
		config := &runtime.Config{
			EntrypointPath: "./custom-entrypoint.ts",
		}

		_, err = runtime.NewBunManager(ctx, tmpDir, config)
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

		ctx := t.Context()
		_, err = runtime.NewBunManager(ctx, tmpDir, nil)
		require.NoError(t, err)

		// Read the generated worker file
		storeDir := core.GetStoreDir(tmpDir)
		workerPath := filepath.Join(storeDir, "bun_worker.ts")
		content, err := os.ReadFile(workerPath)
		require.NoError(t, err)

		// Verify the fallback entrypoint located inside the store is used when none is configured
		assert.Contains(t, string(content), `import * as allExports from "./default_entrypoint.ts"`)

		fallbackPath := filepath.Join(storeDir, "default_entrypoint.ts")
		fallbackContent, err := os.ReadFile(fallbackPath)
		require.NoError(t, err)
		assert.Equal(t, "export default {}\n", string(fallbackContent))
	})
}

// Helper function to check if Bun is available
func isBunAvailable() bool {
	// Use the same logic as the runtime package
	return runtime.IsBunAvailable()
}
