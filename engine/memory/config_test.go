package memory

import (
	"testing"
	"time"

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

func TestConfig_LockTTLMethods(t *testing.T) {
	t.Run("GetAppendLockTTL - Should return parsed TTL when valid", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				AppendTTL: "45s",
			},
		}
		result := cfg.GetAppendLockTTL()
		assert.Equal(t, 45*time.Second, result)
	})

	t.Run("GetAppendLockTTL - Should return default when no locking config", func(t *testing.T) {
		cfg := &Config{ID: "test-memory"}
		result := cfg.GetAppendLockTTL()
		assert.Equal(t, 30*time.Second, result)
	})

	t.Run("GetAppendLockTTL - Should return default when AppendTTL is empty", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				AppendTTL: "",
			},
		}
		result := cfg.GetAppendLockTTL()
		assert.Equal(t, 30*time.Second, result)
	})

	t.Run("GetAppendLockTTL - Should return default when TTL is invalid", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				AppendTTL: "invalid-duration",
			},
		}

		result := cfg.GetAppendLockTTL()

		// Should return default value even when parsing fails
		assert.Equal(t, 30*time.Second, result)
	})

	t.Run("GetClearLockTTL - Should return parsed TTL when valid", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				ClearTTL: "15s",
			},
		}
		result := cfg.GetClearLockTTL()
		assert.Equal(t, 15*time.Second, result)
	})

	t.Run("GetClearLockTTL - Should return default when no locking config", func(t *testing.T) {
		cfg := &Config{ID: "test-memory"}
		result := cfg.GetClearLockTTL()
		assert.Equal(t, 10*time.Second, result)
	})

	t.Run("GetClearLockTTL - Should return default when TTL is invalid", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				ClearTTL: "bad-time",
			},
		}

		result := cfg.GetClearLockTTL()

		// Should return default value even when parsing fails
		assert.Equal(t, 10*time.Second, result)
	})

	t.Run("GetFlushLockTTL - Should return parsed TTL when valid", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				FlushTTL: "10m",
			},
		}
		result := cfg.GetFlushLockTTL()
		assert.Equal(t, 10*time.Minute, result)
	})

	t.Run("GetFlushLockTTL - Should return default when no locking config", func(t *testing.T) {
		cfg := &Config{ID: "test-memory"}
		result := cfg.GetFlushLockTTL()
		assert.Equal(t, 5*time.Minute, result)
	})

	t.Run("GetFlushLockTTL - Should return default when TTL is invalid", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				FlushTTL: "not-a-duration",
			},
		}

		result := cfg.GetFlushLockTTL()

		// Should return default value even when parsing fails
		assert.Equal(t, 5*time.Minute, result)
	})
}
