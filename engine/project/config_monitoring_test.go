package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MonitoringConfig(t *testing.T) {
	t.Run("Should load monitoring config from YAML", func(t *testing.T) {
		// Create temporary directory and config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
monitoring:
  enabled: true
  path: /custom/metrics
workflows:
  - source: ./workflow.yaml
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		// Create dummy workflow file
		workflowPath := filepath.Join(tmpDir, "workflow.yaml")
		err = os.WriteFile(workflowPath, []byte("name: test-workflow\nversion: 0.1.0"), 0644)
		require.NoError(t, err)
		// Load project
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg, err := Load(cwd, configPath, "")
		require.NoError(t, err)
		// Verify monitoring config
		assert.NotNil(t, cfg.MonitoringConfig)
		assert.True(t, cfg.MonitoringConfig.Enabled)
		assert.Equal(t, "/custom/metrics", cfg.MonitoringConfig.Path)
	})
	t.Run("Should use default monitoring config when not specified", func(t *testing.T) {
		// Create temporary directory and config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
workflows:
  - source: ./workflow.yaml
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		// Create dummy workflow file
		workflowPath := filepath.Join(tmpDir, "workflow.yaml")
		err = os.WriteFile(workflowPath, []byte("name: test-workflow\nversion: 0.1.0"), 0644)
		require.NoError(t, err)
		// Load project
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg, err := Load(cwd, configPath, "")
		require.NoError(t, err)
		// Verify default monitoring config
		assert.NotNil(t, cfg.MonitoringConfig)
		assert.False(t, cfg.MonitoringConfig.Enabled)
		assert.Equal(t, "/metrics", cfg.MonitoringConfig.Path)
	})
	t.Run("Should give precedence to environment variable", func(t *testing.T) {
		// Set environment variable
		t.Setenv("MONITORING_ENABLED", "true")
		// Create temporary directory and config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
monitoring:
  enabled: false
  path: /metrics
workflows:
  - source: ./workflow.yaml
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		// Create dummy workflow file
		workflowPath := filepath.Join(tmpDir, "workflow.yaml")
		err = os.WriteFile(workflowPath, []byte("name: test-workflow\nversion: 0.1.0"), 0644)
		require.NoError(t, err)
		// Load project
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg, err := Load(cwd, configPath, "")
		require.NoError(t, err)
		// Verify environment variable took precedence
		assert.NotNil(t, cfg.MonitoringConfig)
		assert.True(t, cfg.MonitoringConfig.Enabled) // Env var overrides YAML
		assert.Equal(t, "/metrics", cfg.MonitoringConfig.Path)
	})
}

func TestConfig_Validate_Monitoring(t *testing.T) {
	t.Run("Should validate valid monitoring config", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			MonitoringConfig: &monitoring.Config{
				Enabled: true,
				Path:    "/metrics",
			},
		}
		err = cfg.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should fail validation for invalid monitoring path", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			MonitoringConfig: &monitoring.Config{
				Enabled: true,
				Path:    "/api/metrics", // Invalid path
			},
		}
		err = cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "monitoring configuration validation failed")
		assert.Contains(t, err.Error(), "monitoring path cannot be under /api/")
	})
	t.Run("Should pass validation when monitoring is nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:             "test-project",
			Version:          "0.1.0",
			CWD:              cwd,
			MonitoringConfig: nil,
		}
		err = cfg.Validate()
		assert.NoError(t, err)
	})
}
