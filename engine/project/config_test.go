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

// TestLoad_MaxNestingDepth has been removed as MaxNestingDepth is now configured server-side
// in pkg/config and not exposed in project configuration.

func TestRuntimeConfig_Validation(t *testing.T) {
	t.Run("Should validate valid bun runtime config", func(t *testing.T) {
		tmpDir := t.TempDir()
		entrypoint := filepath.Join(tmpDir, "tools.ts")
		err := os.WriteFile(entrypoint, []byte("export {}"), 0644)
		require.NoError(t, err)
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "bun",
				Entrypoint:  "tools.ts",
				Permissions: []string{"read"},
			},
		}
		err = cfg.validateRuntimeConfig()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid node runtime config", func(t *testing.T) {
		tmpDir := t.TempDir()
		entrypoint := filepath.Join(tmpDir, "tools.js")
		err := os.WriteFile(entrypoint, []byte("module.exports = {}"), 0644)
		require.NoError(t, err)
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "node",
				Entrypoint:  "tools.js",
				Permissions: []string{"read", "write"},
			},
		}
		err = cfg.validateRuntimeConfig()
		assert.NoError(t, err)
	})

	t.Run("Should fail validation for invalid runtime type", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "invalid",
				Entrypoint:  "tools.ts",
				Permissions: []string{"read"},
			},
		}
		err = cfg.validateRuntimeConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid runtime type")
	})

	t.Run("Should fail validation for missing entrypoint file", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "bun",
				Entrypoint:  "nonexistent.ts",
				Permissions: []string{"read"},
			},
		}
		err = cfg.validateRuntimeConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "entrypoint file 'nonexistent.ts' does not exist")
	})

	t.Run("Should fail validation for invalid entrypoint extension", func(t *testing.T) {
		tmpDir := t.TempDir()
		entrypoint := filepath.Join(tmpDir, "tools.py")
		err := os.WriteFile(entrypoint, []byte("# python file"), 0644)
		require.NoError(t, err)
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "bun",
				Entrypoint:  "tools.py",
				Permissions: []string{"read"},
			},
		}
		err = cfg.validateRuntimeConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has unsupported extension '.py'")
	})

	t.Run(
		"Should pass validation when runtime config has empty values (will be filled by defaults later)",
		func(t *testing.T) {
			tmpDir := t.TempDir()
			cwd, err := core.CWDFromPath(tmpDir)
			require.NoError(t, err)
			cfg := &Config{
				Name:    "test-project",
				Version: "0.1.0",
				CWD:     cwd,
				// Runtime field will be zero value - validation passes, defaults applied later
			}
			err = cfg.validateRuntimeConfig()
			assert.NoError(t, err)
		},
	)
}

func TestRuntimeConfig_Defaults(t *testing.T) {
	t.Run("Should set default runtime config when zero value", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			// Runtime field will be zero value
		}
		setRuntimeDefaults(&cfg.Runtime)
		assert.Equal(t, "bun", cfg.Runtime.Type)
		assert.Equal(t, "./tools.ts", cfg.Runtime.Entrypoint)
		assert.Equal(t, []string{"--allow-read"}, cfg.Runtime.Permissions)
	})

	t.Run("Should preserve existing runtime config", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "node",
				Entrypoint:  "custom.js",
				Permissions: []string{"read", "write"},
			},
		}
		setRuntimeDefaults(&cfg.Runtime)
		assert.Equal(t, "node", cfg.Runtime.Type)
		assert.Equal(t, "custom.js", cfg.Runtime.Entrypoint)
		assert.Equal(t, []string{"read", "write"}, cfg.Runtime.Permissions)
	})

	t.Run("Should set default type when empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "",
				Entrypoint:  "tools.ts",
				Permissions: []string{"read"},
			},
		}
		setRuntimeDefaults(&cfg.Runtime)
		assert.Equal(t, "bun", cfg.Runtime.Type)
	})

	t.Run("Should set default entrypoint when empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "bun",
				Entrypoint:  "",
				Permissions: []string{"read"},
			},
		}
		setRuntimeDefaults(&cfg.Runtime)
		assert.Equal(t, "./tools.ts", cfg.Runtime.Entrypoint)
	})

	t.Run("Should set default permissions when nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:        "bun",
				Entrypoint:  "tools.ts",
				Permissions: nil,
			},
		}
		setRuntimeDefaults(&cfg.Runtime)
		assert.Equal(t, []string{"--allow-read"}, cfg.Runtime.Permissions)
	})
}

func TestLoad_RuntimeConfig(t *testing.T) {
	t.Run("Should load runtime config from YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
runtime:
  type: node
  entrypoint: custom-tools.js
  permissions: ["read", "write"]
workflows:
  - source: ./workflow.yaml
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		workflowPath := filepath.Join(tmpDir, "workflow.yaml")
		err = os.WriteFile(workflowPath, []byte("name: test-workflow\nversion: 0.1.0"), 0644)
		require.NoError(t, err)
		entrypointPath := filepath.Join(tmpDir, "custom-tools.js")
		err = os.WriteFile(entrypointPath, []byte("module.exports = {}"), 0644)
		require.NoError(t, err)
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		assert.Equal(t, "node", cfg.Runtime.Type)
		assert.Equal(t, "custom-tools.js", cfg.Runtime.Entrypoint)
		assert.Equal(t, []string{"read", "write"}, cfg.Runtime.Permissions)
	})

	t.Run("Should use default runtime config when not specified", func(t *testing.T) {
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
		workflowPath := filepath.Join(tmpDir, "workflow.yaml")
		err = os.WriteFile(workflowPath, []byte("name: test-workflow\nversion: 0.1.0"), 0644)
		require.NoError(t, err)
		entrypointPath := filepath.Join(tmpDir, "tools.ts")
		err = os.WriteFile(entrypointPath, []byte("export {}"), 0644)
		require.NoError(t, err)
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		assert.Equal(t, "bun", cfg.Runtime.Type)
		assert.Equal(t, "./tools.ts", cfg.Runtime.Entrypoint)
		assert.Equal(t, []string{"--allow-read"}, cfg.Runtime.Permissions)
	})

	t.Run("Should fail validation for invalid runtime config in YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "compozy.yaml")
		configContent := `
name: test-project
version: 0.1.0
runtime:
  type: invalid-runtime
  entrypoint: tools.ts
  permissions: ["read"]
workflows:
  - source: ./workflow.yaml
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		workflowPath := filepath.Join(tmpDir, "workflow.yaml")
		err = os.WriteFile(workflowPath, []byte("name: test-workflow\nversion: 0.1.0"), 0644)
		require.NoError(t, err)
		// Create the entrypoint file so validation doesn't fail on missing file
		entrypointPath := filepath.Join(tmpDir, "tools.ts")
		err = os.WriteFile(entrypointPath, []byte("export {}"), 0644)
		require.NoError(t, err)
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err) // Load should succeed
		// But validation should fail due to invalid runtime type
		err = cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid runtime type")
	})
}
