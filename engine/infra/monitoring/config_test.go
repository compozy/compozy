package monitoring

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("Should return config with default values", func(t *testing.T) {
		cfg := DefaultConfig()
		assert.NotNil(t, cfg)
		assert.False(t, cfg.Enabled)
		assert.Equal(t, "/metrics", cfg.Path)
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
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "monitoring path cannot be empty")
	})
	t.Run("Should reject path not starting with slash", func(t *testing.T) {
		cfg := &Config{
			Enabled: true,
			Path:    "metrics",
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "monitoring path must start with '/'")
	})
}
