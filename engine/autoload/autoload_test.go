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
