package monitoring

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("Should return config with default values", func(t *testing.T) {
		cfg := DefaultConfig()
		assert.NotNil(t, cfg)
		assert.False(t, cfg.Enabled)
		assert.Equal(t, "/metrics", cfg.Path)
	})
}

func TestLoadWithEnv(t *testing.T) {
	t.Run("Should return defaults when no config provided", func(t *testing.T) {
		result, err := LoadWithEnv(t.Context(), nil)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Enabled)
		assert.Equal(t, "/metrics", result.Path)
	})
	t.Run("Should use YAML config values when provided", func(t *testing.T) {
		yamlConfig := &Config{
			Enabled: true,
			Path:    "/custom/metrics",
		}
		result, err := LoadWithEnv(t.Context(), yamlConfig)
		require.NoError(t, err)
		assert.True(t, result.Enabled)
		assert.Equal(t, "/custom/metrics", result.Path)
	})
	t.Run("Should use default path when YAML path is empty", func(t *testing.T) {
		yamlConfig := &Config{
			Enabled: true,
			Path:    "",
		}
		result, err := LoadWithEnv(t.Context(), yamlConfig)
		require.NoError(t, err)
		assert.True(t, result.Enabled)
		assert.Equal(t, "/metrics", result.Path)
	})
	t.Run("Should apply partial YAML config", func(t *testing.T) {
		yamlConfig := &Config{
			Enabled: true,
			// Path not specified, should use default
		}
		result, err := LoadWithEnv(t.Context(), yamlConfig)
		require.NoError(t, err)
		assert.True(t, result.Enabled)
		assert.Equal(t, "/metrics", result.Path)
	})
	t.Run("Should give precedence to environment variable when set to true", func(t *testing.T) {
		// Set environment variable
		t.Setenv("MONITORING_ENABLED", "true")
		yamlConfig := &Config{
			Enabled: false, // YAML says false
			Path:    "/metrics",
		}
		result, err := LoadWithEnv(t.Context(), yamlConfig)
		require.NoError(t, err)
		assert.True(t, result.Enabled) // Env var takes precedence
		assert.Equal(t, "/metrics", result.Path)
	})
	t.Run("Should give precedence to environment variable when set to false", func(t *testing.T) {
		// Set environment variable
		t.Setenv("MONITORING_ENABLED", "false")
		yamlConfig := &Config{
			Enabled: true, // YAML says true
			Path:    "/metrics",
		}
		result, err := LoadWithEnv(t.Context(), yamlConfig)
		require.NoError(t, err)
		assert.False(t, result.Enabled) // Env var takes precedence
		assert.Equal(t, "/metrics", result.Path)
	})
	t.Run("Should handle invalid environment variable value gracefully", func(t *testing.T) {
		// Set invalid environment variable
		t.Setenv("MONITORING_ENABLED", "not-a-bool")
		yamlConfig := &Config{
			Enabled: true,
			Path:    "/metrics",
		}
		result, err := LoadWithEnv(t.Context(), yamlConfig)
		require.NoError(t, err)
		// Should fall back to YAML config when env var is invalid
		assert.True(t, result.Enabled)
		assert.Equal(t, "/metrics", result.Path)
	})
	t.Run("Should handle various boolean string formats", func(t *testing.T) {
		testCases := []struct {
			envValue      string
			expectedValue bool
		}{
			{"TRUE", true},
			{"True", true},
			{"1", true},
			{"FALSE", false},
			{"False", false},
			{"0", false},
		}
		for _, tc := range testCases {
			// pin loop variable for the closure
			t.Run("env value "+tc.envValue, func(t *testing.T) {
				t.Setenv("MONITORING_ENABLED", tc.envValue)
				yamlConfig := &Config{
					Enabled: !tc.expectedValue, // Opposite of expected
					Path:    "/metrics",
				}
				result, err := LoadWithEnv(t.Context(), yamlConfig)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedValue, result.Enabled)
			})
		}
	})
	t.Run("Should give precedence to MONITORING_PATH environment variable", func(t *testing.T) {
		// Set environment variable
		t.Setenv("MONITORING_PATH", "/env/metrics")
		yamlConfig := &Config{
			Enabled: true,
			Path:    "/yaml/metrics",
		}
		result, err := LoadWithEnv(t.Context(), yamlConfig)
		require.NoError(t, err)
		assert.Equal(t, "/env/metrics", result.Path)
		assert.True(t, result.Enabled)
	})
	t.Run("Should handle both environment variables together", func(t *testing.T) {
		// Set both environment variables
		t.Setenv("MONITORING_ENABLED", "false")
		t.Setenv("MONITORING_PATH", "/custom/path")
		yamlConfig := &Config{
			Enabled: true,
			Path:    "/metrics",
		}
		result, err := LoadWithEnv(t.Context(), yamlConfig)
		require.NoError(t, err)
		assert.False(t, result.Enabled)              // Env overrides YAML
		assert.Equal(t, "/custom/path", result.Path) // Env overrides YAML
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("Should accept valid configuration", func(t *testing.T) {
		cfg := &Config{
			Enabled: true,
			Path:    "/metrics",
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should accept custom path", func(t *testing.T) {
		cfg := &Config{
			Enabled: false,
			Path:    "/custom/metrics",
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should reject empty path", func(t *testing.T) {
		cfg := &Config{
			Enabled: true,
			Path:    "",
		}
		err := cfg.Validate()
		assert.ErrorContains(t, err, "monitoring path cannot be empty")
	})
	t.Run("Should reject path not starting with slash", func(t *testing.T) {
		cfg := &Config{
			Enabled: true,
			Path:    "metrics",
		}
		err := cfg.Validate()
		assert.ErrorContains(t, err, "monitoring path must start with '/'")
	})
	t.Run("Should reject path under /api/", func(t *testing.T) {
		cfg := &Config{
			Enabled: true,
			Path:    "/api/metrics",
		}
		err := cfg.Validate()
		assert.ErrorContains(t, err, "monitoring path cannot be under /api/")
	})
	t.Run("Should reject path with query parameters", func(t *testing.T) {
		cfg := &Config{
			Enabled: true,
			Path:    "/metrics?format=json",
		}
		err := cfg.Validate()
		assert.ErrorContains(t, err, "monitoring path cannot contain query parameters")
	})
	t.Run("Should accept various valid paths", func(t *testing.T) {
		validPaths := []string{
			"/metrics",
			"/monitoring/metrics",
			"/custom-metrics",
			"/m",
			"/health/metrics",
		}
		for _, path := range validPaths {
			t.Run("path "+path, func(t *testing.T) {
				cfg := &Config{
					Enabled: true,
					Path:    path,
				}
				err := cfg.Validate()
				assert.NoError(t, err)
			})
		}
	})
}
