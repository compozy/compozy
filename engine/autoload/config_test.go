package autoload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestNewConfig(t *testing.T) {
	t.Run("Should create config with defaults", func(t *testing.T) {
		config := NewConfig()
		assert.False(t, config.Enabled)
		assert.True(t, config.Strict)
		assert.Empty(t, config.Include)
		assert.Empty(t, config.Exclude)
		assert.False(t, config.WatchEnabled)
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("Should pass validation when disabled", func(t *testing.T) {
		config := &Config{
			Enabled: false,
		}
		assert.NoError(t, config.Validate())
	})
	t.Run("Should fail validation when enabled without include patterns", func(t *testing.T) {
		config := &Config{
			Enabled: true,
			Include: []string{},
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "include patterns are required")
	})
	t.Run("Should pass validation with valid include patterns", func(t *testing.T) {
		config := &Config{
			Enabled: true,
			Include: []string{"workflows/**/*.yaml", "tasks/**/*.yaml"},
		}
		assert.NoError(t, config.Validate())
	})
	t.Run("Should fail validation with empty include pattern", func(t *testing.T) {
		config := &Config{
			Enabled: true,
			Include: []string{"workflows/**/*.yaml", ""},
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty include pattern")
	})
	t.Run("Should fail validation with empty exclude pattern", func(t *testing.T) {
		config := &Config{
			Enabled: true,
			Include: []string{"workflows/**/*.yaml"},
			Exclude: []string{"test/**", ""},
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty exclude pattern")
	})
	t.Run("Should pass validation with valid exclude patterns", func(t *testing.T) {
		config := &Config{
			Enabled: true,
			Include: []string{"workflows/**/*.yaml"},
			Exclude: []string{"**/test/**", "**/*.example.yaml"},
		}
		assert.NoError(t, config.Validate())
	})
}

func TestConfig_YAML_Marshaling(t *testing.T) {
	t.Run("Should marshal to YAML correctly", func(t *testing.T) {
		config := &Config{
			Enabled:      true,
			Strict:       false,
			Include:      []string{"workflows/**/*.yaml", "tasks/**/*.yaml"},
			Exclude:      []string{"**/test/**"},
			WatchEnabled: true,
		}
		marshaled, err := yaml.Marshal(config)
		assert.NoError(t, err)
		expected := `enabled: true
strict: false
include:
    - workflows/**/*.yaml
    - tasks/**/*.yaml
exclude:
    - '**/test/**'
watch_enabled: true
`
		var got, want Config
		assert.NoError(t, yaml.Unmarshal(marshaled, &got))
		assert.NoError(t, yaml.Unmarshal([]byte(expected), &want))
		assert.Equal(t, want, got)
	})
	t.Run("Should unmarshal from YAML correctly", func(t *testing.T) {
		yamlData := `
enabled: true
strict: false
include:
  - "workflows/**/*.yaml"
  - "tasks/**/*.yaml"
exclude:
  - "**/test/**"
watch_enabled: true
`
		var config Config
		err := yaml.Unmarshal([]byte(yamlData), &config)
		assert.NoError(t, err)
		assert.True(t, config.Enabled)
		assert.False(t, config.Strict)
		assert.Equal(t, []string{"workflows/**/*.yaml", "tasks/**/*.yaml"}, config.Include)
		assert.Equal(t, []string{"**/test/**"}, config.Exclude)
		assert.True(t, config.WatchEnabled)
	})
	t.Run("Should handle minimal YAML configuration", func(t *testing.T) {
		yamlData := `
enabled: true
include:
  - "**/*.yaml"
`
		var config Config
		err := yaml.Unmarshal([]byte(yamlData), &config)
		assert.NoError(t, err)
		assert.True(t, config.Enabled)
		assert.False(t, config.Strict) // Default is false when unmarshaling
		assert.Equal(t, []string{"**/*.yaml"}, config.Include)
		assert.Empty(t, config.Exclude)
		assert.False(t, config.WatchEnabled)
	})
}

func TestConfig_SetDefaults(t *testing.T) {
	t.Run("Should set strict to true when disabled", func(t *testing.T) {
		config := &Config{
			Enabled: false,
			Strict:  false,
		}
		config.SetDefaults()
		assert.True(t, config.Strict)
	})
	t.Run("Should not change strict when enabled", func(t *testing.T) {
		config := &Config{
			Enabled: true,
			Strict:  false,
		}
		config.SetDefaults()
		assert.False(t, config.Strict)
	})
	t.Run("Should be idempotent - no changes on repeated calls", func(t *testing.T) {
		config := &Config{
			Enabled: false,
			Strict:  false,
		}
		config.SetDefaults()
		assert.True(t, config.Strict)
		config.Strict = false
		config.SetDefaults()
		assert.False(t, config.Strict)
	})
}

func TestConfig_GetAllExcludes(t *testing.T) {
	t.Run("Should return default excludes when no user excludes", func(t *testing.T) {
		config := &Config{}
		excludes := config.GetAllExcludes()
		assert.Equal(t, DefaultExcludes, excludes)
	})
	t.Run("Should combine default and user excludes", func(t *testing.T) {
		config := &Config{
			Exclude: []string{"**/custom/**", "*.tmp.yaml"},
		}
		excludes := config.GetAllExcludes()
		expected := make([]string, 0, len(DefaultExcludes)+2)
		expected = append(expected, DefaultExcludes...)
		expected = append(expected, "**/custom/**", "*.tmp.yaml")
		assert.Equal(t, expected, excludes)
	})
}

func TestDefaultExcludes(t *testing.T) {
	t.Run("Should contain expected default patterns", func(t *testing.T) {
		expected := []string{
			"**/.#*",   // Emacs lock files
			"**/*~",    // Backup files
			"**/*.bak", // Backup files
			"**/*.swp", // Vim swap files
			"**/*.tmp", // Temporary files
			"**/._*",   // macOS resource forks
		}
		assert.Equal(t, expected, DefaultExcludes)
	})
}
