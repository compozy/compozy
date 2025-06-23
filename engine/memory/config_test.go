package memory

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Basic(t *testing.T) {
	t.Run("Valid configuration", func(t *testing.T) {
		cfg := &Config{
			Resource:  "memory",
			ID:        "test-memory",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
		}

		assert.Equal(t, "memory", cfg.GetResource())
		assert.Equal(t, "test-memory", cfg.GetID())
		assert.Equal(t, core.ConfigMemory, cfg.Component())
	})

	t.Run("SetCWD works", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.SetCWD(".")
		require.NoError(t, err)
		assert.NotNil(t, cfg.GetCWD())
	})
}
