package autoload

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

func TestAutoLoader_Load(t *testing.T) {
	logger.InitForTests()
	t.Run("Should load valid configuration files", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  true,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create a valid workflow configuration file
		workflowContent := `resource: workflow
id: test-workflow
version: "1.0"
description: Test workflow
tasks:
  - type: basic
    id: test-task
    agent: test-agent
`
		workflowPath := filepath.Join(tempDir, "workflows", "test.yaml")
		err := os.MkdirAll(filepath.Dir(workflowPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(workflowPath, []byte(workflowContent), 0644)
		require.NoError(t, err)
		err = loader.Load(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 1, registry.Count())
		// Verify the configuration was registered correctly
		retrievedConfig, err := registry.Get("workflow", "test-workflow")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedConfig)
	})

	t.Run("Should handle missing resource field in strict mode", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  true,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create an invalid configuration file without resource field
		invalidContent := `id: test-workflow
version: "1.0"
description: Test workflow without resource field
`
		workflowPath := filepath.Join(tempDir, "workflows", "invalid.yaml")
		err := os.MkdirAll(filepath.Dir(workflowPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(workflowPath, []byte(invalidContent), 0644)
		require.NoError(t, err)
		err = loader.Load(context.Background())
		assert.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "AUTOLOAD_FILE_FAILED", coreErr.Code)
		assert.Equal(t, 0, registry.Count())
	})

	t.Run("Should skip invalid files in non-strict mode", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  false,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create both valid and invalid configuration files
		validContent := `resource: workflow
id: valid-workflow
version: "1.0"
description: Valid workflow
tasks:
  - type: basic
    id: test-task
    agent: test-agent
`
		invalidContent := `id: invalid-workflow
version: "1.0"
description: Invalid workflow without resource field
`
		validPath := filepath.Join(tempDir, "workflows", "valid.yaml")
		invalidPath := filepath.Join(tempDir, "workflows", "invalid.yaml")
		err := os.MkdirAll(filepath.Dir(validPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(validPath, []byte(validContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(invalidPath, []byte(invalidContent), 0644)
		require.NoError(t, err)
		err = loader.Load(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 1, registry.Count())
		// Verify only the valid configuration was registered
		retrievedConfig, err := registry.Get("workflow", "valid-workflow")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedConfig)
	})

	t.Run("Should handle missing ID field in strict mode", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  true,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create an invalid configuration file without ID field
		invalidContent := `resource: workflow
version: "1.0"
description: Test workflow without ID field
`
		workflowPath := filepath.Join(tempDir, "workflows", "invalid.yaml")
		err := os.MkdirAll(filepath.Dir(workflowPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(workflowPath, []byte(invalidContent), 0644)
		require.NoError(t, err)
		err = loader.Load(context.Background())
		assert.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "AUTOLOAD_FILE_FAILED", coreErr.Code)
		assert.Equal(t, 0, registry.Count())
	})

	t.Run("Should handle context cancellation", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  true,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create a configuration file
		workflowContent := `resource: workflow
id: test-workflow
version: "1.0"
`
		workflowPath := filepath.Join(tempDir, "workflows", "test.yaml")
		err := os.MkdirAll(filepath.Dir(workflowPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(workflowPath, []byte(workflowContent), 0644)
		require.NoError(t, err)
		// Cancel the context immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err = loader.Load(ctx)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Should skip loading when disabled", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: false,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		err := loader.Load(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 0, registry.Count())
	})

	t.Run("Should handle empty discovery results", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  true,
			Include: []string{"nonexistent/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		err := loader.Load(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 0, registry.Count())
	})
}

func TestAutoLoader_Discover(t *testing.T) {
	logger.InitForTests()
	t.Run("Should return discovered files list", func(t *testing.T) {
		tempDir := t.TempDir()
		config := &Config{
			Enabled: true,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, nil)
		// Create test files
		workflowPath := filepath.Join(tempDir, "workflows", "test.yaml")
		err := os.MkdirAll(filepath.Dir(workflowPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(workflowPath, []byte("test"), 0644)
		require.NoError(t, err)
		files, err := loader.Discover(context.Background())
		assert.NoError(t, err)
		assert.Len(t, files, 1)
		assert.Contains(t, files[0], "workflows/test.yaml")
	})
}

func TestAutoLoader_LoadWithResult(t *testing.T) {
	logger.InitForTests()
	t.Run("Should return detailed results for successful loading", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  false,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create multiple valid configuration files
		workflowContent1 := `resource: workflow
id: workflow-1
version: "1.0"
description: First workflow
`
		workflowContent2 := `resource: workflow
id: workflow-2
version: "1.0"
description: Second workflow
`
		workflowsDir := filepath.Join(tempDir, "workflows")
		err := os.MkdirAll(workflowsDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(workflowsDir, "workflow1.yaml"), []byte(workflowContent1), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(workflowsDir, "workflow2.yaml"), []byte(workflowContent2), 0644)
		require.NoError(t, err)
		result, err := loader.LoadWithResult(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.FilesProcessed)
		assert.Equal(t, 2, result.ConfigsLoaded)
		assert.Empty(t, result.Errors)
	})

	t.Run("Should aggregate errors in non-strict mode", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  false,
			Include: []string{"**/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create mixed valid and invalid files
		validContent := `resource: workflow
id: valid-workflow
version: "1.0"
`
		invalidContent1 := `id: missing-resource
version: "1.0"
`
		invalidContent2 := `resource: workflow
version: "1.0"
`
		err := os.WriteFile(filepath.Join(tempDir, "valid.yaml"), []byte(validContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "invalid1.yaml"), []byte(invalidContent1), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "invalid2.yaml"), []byte(invalidContent2), 0644)
		require.NoError(t, err)
		result, err := loader.LoadWithResult(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 3, result.FilesProcessed)
		assert.Equal(t, 1, result.ConfigsLoaded)
		assert.Len(t, result.Errors, 2)
		// Verify error details
		errorFiles := make([]string, len(result.Errors))
		for i, loadErr := range result.Errors {
			errorFiles[i] = filepath.Base(loadErr.File)
		}
		assert.Contains(t, errorFiles, "invalid1.yaml")
		assert.Contains(t, errorFiles, "invalid2.yaml")
	})

	t.Run("Should fail fast in strict mode with detailed error", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  true,
			Include: []string{"**/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		invalidContent := `id: missing-resource
version: "1.0"
`
		err := os.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte(invalidContent), 0644)
		require.NoError(t, err)
		result, err := loader.LoadWithResult(context.Background())
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, result.FilesProcessed)
		assert.Equal(t, 0, result.ConfigsLoaded)
		assert.Len(t, result.Errors, 1)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "AUTOLOAD_FILE_FAILED", coreErr.Code)
	})

	t.Run("Should handle no valid configs in strict mode", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  true,
			Include: []string{"**/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create only invalid files - in strict mode, it will fail on the first error
		invalidContent1 := `id: missing-resource`
		invalidContent2 := `resource: workflow`
		err := os.WriteFile(filepath.Join(tempDir, "invalid1.yaml"), []byte(invalidContent1), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "invalid2.yaml"), []byte(invalidContent2), 0644)
		require.NoError(t, err)
		result, err := loader.LoadWithResult(context.Background())
		assert.Error(t, err)
		assert.NotNil(t, result)
		// In strict mode, it fails fast on first error, so FilesProcessed will be 1
		assert.Equal(t, 1, result.FilesProcessed)
		assert.Equal(t, 0, result.ConfigsLoaded)
		assert.Len(t, result.Errors, 1)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "AUTOLOAD_FILE_FAILED", coreErr.Code)
	})
}

func TestAutoLoader_Stats(t *testing.T) {
	logger.InitForTests()
	t.Run("Should return correct statistics", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{Enabled: true, Include: []string{"**/*.yaml"}}
		loader := New(tempDir, config, registry)
		// Manually register some configurations for testing
		workflowConfig := map[string]any{"resource": "workflow", "id": "test-workflow"}
		agentConfig := map[string]any{"resource": "agent", "id": "test-agent"}
		err := registry.Register(workflowConfig, "test")
		require.NoError(t, err)
		err = registry.Register(agentConfig, "test")
		require.NoError(t, err)
		stats := loader.Stats()
		assert.Equal(t, 2, stats["total_configs"])
		assert.Equal(t, 1, stats["workflows"])
		assert.Equal(t, 1, stats["agents"])
		assert.Equal(t, 0, stats["tools"])
		assert.Equal(t, 0, stats["mcps"])
	})
}

func TestAutoLoader_Validate(t *testing.T) {
	logger.InitForTests()
	t.Run("Should perform dry-run validation without affecting main registry", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewConfigRegistry()
		config := &Config{
			Enabled: true,
			Strict:  false,
			Include: []string{"workflows/*.yaml"},
		}
		loader := New(tempDir, config, registry)
		// Create test files
		validContent := `resource: workflow
id: test-workflow
version: "1.0"
`
		invalidContent := `id: missing-resource
version: "1.0"
`
		workflowsDir := filepath.Join(tempDir, "workflows")
		err := os.MkdirAll(workflowsDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(workflowsDir, "valid.yaml"), []byte(validContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(workflowsDir, "invalid.yaml"), []byte(invalidContent), 0644)
		require.NoError(t, err)
		// Original registry should be empty
		assert.Equal(t, 0, registry.Count())
		// Run validation
		result, err := loader.Validate(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.FilesProcessed)
		assert.Equal(t, 1, result.ConfigsLoaded)
		assert.Len(t, result.Errors, 1)
		// Original registry should still be empty
		assert.Equal(t, 0, registry.Count())
	})
}

func TestAutoLoader_GetConfig(t *testing.T) {
	t.Run("Should return the autoload configuration", func(t *testing.T) {
		tempDir := t.TempDir()
		config := &Config{
			Enabled: true,
			Strict:  false,
			Include: []string{"**/*.yaml"},
		}
		loader := New(tempDir, config, nil)
		returnedConfig := loader.GetConfig()
		assert.Equal(t, config, returnedConfig)
		assert.True(t, returnedConfig.Enabled)
		assert.False(t, returnedConfig.Strict)
		assert.Equal(t, []string{"**/*.yaml"}, returnedConfig.Include)
	})
}

func TestAutoLoader_validateFilePath(t *testing.T) {
	tempDir := t.TempDir()
	loader := &AutoLoader{projectRoot: tempDir}

	t.Run("Should validate valid file path", func(t *testing.T) {
		validPath := filepath.Join(tempDir, "config.yaml")
		err := os.WriteFile(validPath, []byte("test"), 0644)
		require.NoError(t, err)
		err = loader.validateFilePath(validPath)
		if err != nil {
			t.Logf("tempDir: %s", tempDir)
			t.Logf("validPath: %s", validPath)
			t.Logf("error: %s", err.Error())
		}
		assert.NoError(t, err)
	})

	t.Run("Should reject path outside project root", func(t *testing.T) {
		outsidePath := "/etc/passwd"
		err := loader.validateFilePath(outsidePath)
		assert.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "PATH_TRAVERSAL_ATTEMPT", coreErr.Code)
	})
}
