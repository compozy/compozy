package project

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetRuntimeDefaults(t *testing.T) {
	cwd, _ := core.CWDFromPath(".")

	t.Run("Should set default permissions when nil", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  cwd,
			Runtime: RuntimeConfig{
				Type:        "",  // Will be set to default
				Entrypoint:  "",  // Remains empty unless explicitly provided
				Permissions: nil, // Should get default permissions
			},
		}

		config.setRuntimeDefaults()

		assert.Equal(t, "bun", config.Runtime.Type)
		assert.Equal(t, "", config.Runtime.Entrypoint)
		assert.Equal(t, []string{"--allow-read"}, config.Runtime.Permissions)
	})

	t.Run("Should preserve explicitly empty permissions", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  cwd,
			Runtime: RuntimeConfig{
				Type:        "bun",
				Entrypoint:  "./tools.ts",
				Permissions: []string{}, // Explicitly empty - should be preserved
			},
		}

		config.setRuntimeDefaults()

		assert.Equal(t, "bun", config.Runtime.Type)
		assert.Equal(t, "./tools.ts", config.Runtime.Entrypoint)
		assert.Equal(t, []string{}, config.Runtime.Permissions) // Should remain empty
	})

	t.Run("Should preserve existing non-empty permissions", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  cwd,
			Runtime: RuntimeConfig{
				Type:        "bun",
				Entrypoint:  "./tools.ts",
				Permissions: []string{"--allow-read", "--allow-write"},
			},
		}

		config.setRuntimeDefaults()

		assert.Equal(t, "bun", config.Runtime.Type)
		assert.Equal(t, "./tools.ts", config.Runtime.Entrypoint)
		assert.Equal(t, []string{"--allow-read", "--allow-write"}, config.Runtime.Permissions)
	})
}

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
		err1 := config.Validate(t.Context())
		assert.NoError(t, err1)
		assert.True(t, config.autoloadValidated)
		assert.NoError(t, config.autoloadValidError)
		err2 := config.Validate(t.Context())
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
		err1 := config.Validate(t.Context())
		require.Error(t, err1)
		assert.Contains(t, err1.Error(), "autoload.include patterns are required when autoload is enabled")
		assert.True(t, config.autoloadValidated)
		require.Error(t, config.autoloadValidError)
		assert.Contains(
			t,
			config.autoloadValidError.Error(),
			"autoload.include patterns are required when autoload is enabled",
		)
		err2 := config.Validate(t.Context())
		require.Error(t, err2)
		assert.Contains(t, err2.Error(), "autoload.include patterns are required when autoload is enabled")
		assert.Equal(t, err1.Error(), err2.Error())
	})

	t.Run("Should skip validation when autoload is nil", func(t *testing.T) {
		config := &Config{
			Name:     "test-project",
			CWD:      cwd,
			AutoLoad: nil,
		}
		err := config.Validate(t.Context())
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

func TestConfig_ValidateKnowledge(t *testing.T) {
	cfg := &Config{
		Embedders: []knowledge.EmbedderConfig{
			{
				ID:       "embedder",
				Provider: "openai",
				Model:    "text-embedding-3-small",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
				Config: knowledge.EmbedderRuntimeConfig{
					Dimension: 1536,
				},
			},
		},
		VectorDBs: []knowledge.VectorDBConfig{
			{
				ID:   "vectordb",
				Type: knowledge.VectorDBTypePGVector,
				Config: knowledge.VectorDBConnConfig{
					DSN:       "{{ .secrets.PGVECTOR_DSN }}",
					Dimension: 1536,
				},
			},
		},
		KnowledgeBases: []knowledge.BaseConfig{
			{
				ID:       "kb",
				Embedder: "embedder",
				VectorDB: "vectordb",
				Sources: []knowledge.SourceConfig{
					{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
				},
			},
		},
		Knowledge: []core.KnowledgeBinding{
			{ID: "kb"},
		},
	}
	require.NoError(t, cfg.validateKnowledge(t.Context()))
	cfg.Knowledge = append(cfg.Knowledge, core.KnowledgeBinding{ID: "kb"})
	err := cfg.validateKnowledge(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one knowledge binding")
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
		err = cfg.Validate(t.Context())
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
		err = cfg.Validate(t.Context())
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
		err = cfg.Validate(t.Context())
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
				Type:                 "bun",
				Entrypoint:           "tools.ts",
				Permissions:          []string{"read"},
				ToolExecutionTimeout: 90 * time.Second,
			},
		}
		err = cfg.validateRuntimeConfig(t.Context())
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
		err = cfg.validateRuntimeConfig(t.Context())
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
		err = cfg.validateRuntimeConfig(t.Context())
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
		err = cfg.validateRuntimeConfig(t.Context())
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
		err = cfg.validateRuntimeConfig(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has unsupported extension '.py'")
	})

	t.Run("Should fail validation for negative tool execution timeout", func(t *testing.T) {
		tmpDir := t.TempDir()
		entrypoint := filepath.Join(tmpDir, "tools.ts")
		require.NoError(t, os.WriteFile(entrypoint, []byte("export {}"), 0o644))
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg := &Config{
			Name:    "test-project",
			Version: "0.1.0",
			CWD:     cwd,
			Runtime: RuntimeConfig{
				Type:                 "bun",
				Entrypoint:           "tools.ts",
				Permissions:          []string{"read"},
				ToolExecutionTimeout: -5 * time.Second,
			},
		}
		err = cfg.validateRuntimeConfig(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool_execution_timeout must be non-negative")
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
			err = cfg.validateRuntimeConfig(t.Context())
			assert.NoError(t, err)
		},
	)
}

func TestDefaultModel_GetDefaultModel(t *testing.T) {
	t.Run("Should return nil when no models configured", func(t *testing.T) {
		config := &Config{
			Models: nil,
		}
		assert.Nil(t, config.GetDefaultModel())
	})
	t.Run("Should return nil when no default model set", func(t *testing.T) {
		config := &Config{
			Models: []*core.ProviderConfig{
				{Provider: "openai", Model: "gpt-4"},
				{Provider: "anthropic", Model: "claude-3"},
			},
		}
		assert.Nil(t, config.GetDefaultModel())
	})
	t.Run("Should return the default model when one is set", func(t *testing.T) {
		config := &Config{
			Models: []*core.ProviderConfig{
				{Provider: "openai", Model: "gpt-4", Default: true},
				{Provider: "anthropic", Model: "claude-3"},
			},
		}
		defaultModel := config.GetDefaultModel()
		assert.NotNil(t, defaultModel)
		assert.Equal(t, "openai", string(defaultModel.Provider))
		assert.Equal(t, "gpt-4", defaultModel.Model)
		assert.True(t, defaultModel.Default)
	})
	t.Run("Should return first default when multiple exist", func(t *testing.T) {
		config := &Config{
			Models: []*core.ProviderConfig{
				{Provider: "openai", Model: "gpt-4", Default: true},
				{Provider: "anthropic", Model: "claude-3", Default: true},
			},
		}
		defaultModel := config.GetDefaultModel()
		assert.NotNil(t, defaultModel)
		assert.Equal(t, "openai", string(defaultModel.Provider))
	})
}

func TestDefaultModel_Validation(t *testing.T) {
	// Helper to create a valid CWD
	createTestCWD := func(t *testing.T) *core.PathCWD {
		dir := t.TempDir()
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		return cwd
	}
	t.Run("Should pass validation with no default model", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  createTestCWD(t),
			Models: []*core.ProviderConfig{
				{Provider: "openai", Model: "gpt-4"},
				{Provider: "anthropic", Model: "claude-3"},
			},
		}
		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})
	t.Run("Should pass validation with one default model", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  createTestCWD(t),
			Models: []*core.ProviderConfig{
				{Provider: "openai", Model: "gpt-4", Default: true},
				{Provider: "anthropic", Model: "claude-3"},
			},
		}
		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})
	t.Run("Should fail validation with multiple default models", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  createTestCWD(t),
			Models: []*core.ProviderConfig{
				{Provider: "openai", Model: "gpt-4", Default: true},
				{Provider: "anthropic", Model: "claude-3", Default: true},
				{Provider: "google", Model: "gemini-pro"},
			},
		}
		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only one model can be marked as default")
	})
	t.Run("Should pass validation with empty models array", func(t *testing.T) {
		config := &Config{
			Name:   "test-project",
			CWD:    createTestCWD(t),
			Models: []*core.ProviderConfig{},
		}
		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})
	t.Run("Should handle nil models in array gracefully", func(t *testing.T) {
		config := &Config{
			Name: "test-project",
			CWD:  createTestCWD(t),
			Models: []*core.ProviderConfig{
				nil,
				{Provider: "openai", Model: "gpt-4", Default: true},
				nil,
			},
		}
		err := config.Validate(t.Context())
		assert.NoError(t, err)
		defaultModel := config.GetDefaultModel()
		assert.NotNil(t, defaultModel)
		assert.Equal(t, "openai", string(defaultModel.Provider))
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
		cwd, err := core.CWDFromPath(tmpDir)
		require.NoError(t, err)
		cfg, err := Load(t.Context(), cwd, configPath, "")
		require.NoError(t, err)
		assert.Equal(t, "bun", cfg.Runtime.Type)
		assert.Equal(t, "", cfg.Runtime.Entrypoint)
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
		err = cfg.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid runtime type")
	})
}

func TestProjectConfigCache_ReusesCachedEntry(t *testing.T) {
	projectConfigCacheStore.reset()
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "compozy.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte("name: test"), 0644))

	var loads int
	loader := func() (*Config, error) {
		loads++
		return &Config{Name: "cached-project"}, nil
	}

	configOne, err := projectConfigCacheStore.Load(filePath, loader)
	require.NoError(t, err)
	require.NotNil(t, configOne)
	assert.Equal(t, 1, loads)

	configOne.Name = "mutated-name"

	configTwo, err := projectConfigCacheStore.Load(filePath, loader)
	require.NoError(t, err)
	require.NotNil(t, configTwo)
	assert.Equal(t, 1, loads)
	assert.Equal(t, "cached-project", configTwo.Name)
	assert.NotSame(t, configOne, configTwo)
}

func TestProjectConfigCache_RefreshesOnFileChange(t *testing.T) {
	projectConfigCacheStore.reset()
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "compozy.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte("name: test"), 0644))

	var loads int
	loader := func() (*Config, error) {
		loads++
		return &Config{Name: fmt.Sprintf("config-%d", loads)}, nil
	}

	configOne, err := projectConfigCacheStore.Load(filePath, loader)
	require.NoError(t, err)
	require.NotNil(t, configOne)
	assert.Equal(t, 1, loads)
	assert.Equal(t, "config-1", configOne.Name)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	newTime := info.ModTime().Add(time.Second)
	require.NoError(t, os.Chtimes(filePath, newTime, newTime))

	configTwo, err := projectConfigCacheStore.Load(filePath, loader)
	require.NoError(t, err)
	require.NotNil(t, configTwo)
	assert.Equal(t, 2, loads)
	assert.Equal(t, "config-2", configTwo.Name)
}
