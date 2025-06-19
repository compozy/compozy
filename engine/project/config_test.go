package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_AutoloadValidationCaching(t *testing.T) {
	cwd, _ := core.CWDFromPath(".")
	t.Run("Should cache autoload validation results", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  cwd,
			AutoLoad: &autoload.Config{
				Enabled: true,
				Include: []string{"*.yaml"},
			},
		}
		err1 := config.Validate()
		assert.NoError(t, err1)
		assert.True(t, config.autoloadValidated)
		assert.NoError(t, config.autoloadValidError)
		err2 := config.Validate()
		assert.NoError(t, err2)
	})

	t.Run("Should cache autoload validation errors", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  cwd,
			AutoLoad: &autoload.Config{
				Enabled: true,
				Include: []string{},
			},
		}
		err1 := config.Validate()
		assert.Error(t, err1)
		assert.True(t, config.autoloadValidated)
		assert.Error(t, config.autoloadValidError)
		err2 := config.Validate()
		assert.Error(t, err2)
		assert.Equal(t, err1.Error(), err2.Error())
	})

	t.Run("Should skip validation when autoload is nil", func(t *testing.T) {
		config := &Config{
			Name:     "test-project",
			CWD:      cwd,
			AutoLoad: nil,
		}
		err := config.Validate()
		assert.NoError(t, err)
		assert.False(t, config.autoloadValidated)
	})
}

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
		cfg, err := Load(t.Context(), cwd, configPath, "")
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
		cfg, err := Load(t.Context(), cwd, configPath, "")
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
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		// Verify environment variable took precedence
		assert.NotNil(t, cfg.MonitoringConfig)
		assert.True(t, cfg.MonitoringConfig.Enabled) // Env var overrides YAML
		assert.Equal(t, "/metrics", cfg.MonitoringConfig.Path)
	})
	t.Run("Should give precedence to MONITORING_PATH environment variable", func(t *testing.T) {
		// Set environment variables
		t.Setenv("MONITORING_ENABLED", "true")
		t.Setenv("MONITORING_PATH", "/env/metrics")
		// Create temporary directory and config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
monitoring:
  enabled: false
  path: /yaml/metrics
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
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		// Verify environment variables took precedence
		assert.NotNil(t, cfg.MonitoringConfig)
		assert.True(t, cfg.MonitoringConfig.Enabled)               // MONITORING_ENABLED env var overrides YAML
		assert.Equal(t, "/env/metrics", cfg.MonitoringConfig.Path) // MONITORING_PATH env var overrides YAML
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

func TestLoad_MaxNestingDepth(t *testing.T) {
	t.Run("Should use default max nesting depth when not configured", func(t *testing.T) {
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
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		// Verify default max nesting depth
		assert.Equal(t, 20, cfg.Opts.MaxNestingDepth)
	})

	t.Run("Should load max nesting depth from YAML", func(t *testing.T) {
		// Create temporary directory and config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
config:
  max_nesting_depth: 50
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
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		// Verify configured max nesting depth
		assert.Equal(t, 50, cfg.Opts.MaxNestingDepth)
	})

	t.Run("Should give precedence to environment variable", func(t *testing.T) {
		// Set environment variable
		t.Setenv("MAX_NESTING_DEPTH", "100")
		// Create temporary directory and config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
config:
  max_nesting_depth: 50
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
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		// Verify environment variable took precedence
		assert.Equal(t, 100, cfg.Opts.MaxNestingDepth)
	})

	t.Run("Should use YAML value when environment variable is invalid", func(t *testing.T) {
		// Set invalid environment variable
		t.Setenv("MAX_NESTING_DEPTH", "invalid")
		// Create temporary directory and config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
config:
  max_nesting_depth: 75
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
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		// Should keep YAML value since env var is invalid
		assert.Equal(t, 75, cfg.Opts.MaxNestingDepth)
	})

	t.Run("Should use default when environment variable is negative", func(t *testing.T) {
		// Set negative environment variable
		t.Setenv("MAX_NESTING_DEPTH", "-10")
		// Create temporary directory and config file without max_nesting_depth
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
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		// Should use default since negative values are invalid
		assert.Equal(t, 20, cfg.Opts.MaxNestingDepth)
	})
}
