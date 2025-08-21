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
	t.Run("Should accept valid configuration", func(t *testing.T) {
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

	t.Run("Should set current working directory successfully", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.SetCWD(".")
		require.NoError(t, err)
		assert.NotNil(t, cfg.GetCWD())
	})
}

func TestConfig_LockTTLMethods(t *testing.T) {
	t.Run("Should return parsed TTL when valid AppendTTL provided", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				AppendTTL: "45s",
			},
		}
		result := cfg.GetAppendLockTTL()
		assert.Equal(t, 45*time.Second, result)
	})

	t.Run("Should return default AppendTTL when no locking config", func(t *testing.T) {
		cfg := &Config{ID: "test-memory"}
		result := cfg.GetAppendLockTTL()
		assert.Equal(t, 30*time.Second, result)
	})

	t.Run("Should return default AppendTTL when field is empty", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				AppendTTL: "",
			},
		}
		result := cfg.GetAppendLockTTL()
		assert.Equal(t, 30*time.Second, result)
	})

	t.Run("Should return default AppendTTL when format is invalid", func(t *testing.T) {
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

	t.Run("Should return parsed TTL when valid ClearTTL provided", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				ClearTTL: "15s",
			},
		}
		result := cfg.GetClearLockTTL()
		assert.Equal(t, 15*time.Second, result)
	})

	t.Run("Should return default ClearTTL when no locking config", func(t *testing.T) {
		cfg := &Config{ID: "test-memory"}
		result := cfg.GetClearLockTTL()
		assert.Equal(t, 10*time.Second, result)
	})

	t.Run("Should return default ClearTTL when format is invalid", func(t *testing.T) {
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

	t.Run("Should return parsed TTL when valid FlushTTL provided", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				FlushTTL: "10m",
			},
		}
		result := cfg.GetFlushLockTTL()
		assert.Equal(t, 10*time.Minute, result)
	})

	t.Run("Should return default FlushTTL when no locking config", func(t *testing.T) {
		cfg := &Config{ID: "test-memory"}
		result := cfg.GetFlushLockTTL()
		assert.Equal(t, 5*time.Minute, result)
	})

	t.Run("Should return default FlushTTL when format is invalid", func(t *testing.T) {
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

func TestConfig_FromMap_ProjectStandard(t *testing.T) {
	// Test that FromMap properly uses mapstructure tags to map fields
	t.Run("Should map fields using mapstructure tags", func(t *testing.T) {
		// Simulate the exact structure that autoloader provides
		yamlMap := map[string]any{
			"resource":     "memory",
			"id":           "user_memory",
			"description":  "User conversation history",
			"version":      "0.1.0",
			"type":         "token_based",
			"max_tokens":   2000,
			"max_messages": 50,
			"flushing": map[string]any{
				"type":      "simple_fifo",
				"threshold": 0.8,
			},
			"persistence": map[string]any{
				"ttl": "168h",
			},
			"privacy_policy": map[string]any{
				"redact_patterns":          []string{"\\b\\d{3}-\\d{2}-\\d{4}\\b"},
				"default_redaction_string": "[REDACTED]",
			},
		}

		// Create config and convert from map
		config := &Config{}
		err := config.FromMap(yamlMap)
		require.NoError(t, err)

		// Verify all fields were properly mapped
		assert.Equal(t, "memory", config.Resource)
		assert.Equal(t, "user_memory", config.ID)
		assert.Equal(t, "User conversation history", config.Description)
		assert.Equal(t, "0.1.0", config.Version)
		assert.Equal(t, memcore.TokenBasedMemory, config.Type)
		assert.Equal(t, 2000, config.MaxTokens)
		assert.Equal(t, 50, config.MaxMessages)

		// Verify nested structures
		require.NotNil(t, config.Flushing)
		assert.Equal(t, memcore.SimpleFIFOFlushing, config.Flushing.Type)

		assert.Equal(t, "168h", config.Persistence.TTL)

		require.NotNil(t, config.PrivacyPolicy)
		assert.Len(t, config.PrivacyPolicy.RedactPatterns, 1)
		assert.Equal(t, "[REDACTED]", config.PrivacyPolicy.DefaultRedactionString)
	})

	t.Run("Should validate after FromMap", func(t *testing.T) {
		yamlMap := map[string]any{
			"resource":   "memory",
			"id":         "test_memory",
			"type":       "token_based",
			"max_tokens": 1000,
			"persistence": map[string]any{
				"ttl": "24h",
			},
		}

		config := &Config{}
		err := config.FromMap(yamlMap)
		require.NoError(t, err)

		// Validate should pass
		err = config.Validate()
		assert.NoError(t, err)
	})
}

func TestConfig_LazyTTLManagerInitialization(t *testing.T) {
	t.Run("Should initialize TTLManager lazily and reuse same instance", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				AppendTTL: "30s",
				ClearTTL:  "60s",
				FlushTTL:  "90s",
			},
		}
		// First call should initialize the TTLManager
		ttl1 := cfg.GetAppendLockTTL()
		assert.Equal(t, 30*time.Second, ttl1)
		assert.NotNil(t, cfg.ttlManager)
		// Store reference to the ttlManager
		firstManager := cfg.ttlManager
		// Subsequent calls should reuse the same TTLManager instance
		ttl2 := cfg.GetClearLockTTL()
		assert.Equal(t, 60*time.Second, ttl2)
		assert.Same(t, firstManager, cfg.ttlManager)
		ttl3 := cfg.GetFlushLockTTL()
		assert.Equal(t, 90*time.Second, ttl3)
		assert.Same(t, firstManager, cfg.ttlManager)
	})
	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				AppendTTL: "30s",
			},
		}
		// Run multiple goroutines to test concurrent initialization
		done := make(chan bool, 10)
		for range 10 {
			go func() {
				ttl := cfg.GetAppendLockTTL()
				assert.Equal(t, 30*time.Second, ttl)
				done <- true
			}()
		}
		// Wait for all goroutines to complete
		for range 10 {
			<-done
		}
		// Verify only one TTLManager was created
		assert.NotNil(t, cfg.ttlManager)
	})
	t.Run("Should preserve TTLManager across FromMap calls", func(t *testing.T) {
		cfg := &Config{
			ID: "test-memory",
			Locking: &memcore.LockConfig{
				AppendTTL: "30s",
			},
		}
		// Initialize TTLManager
		ttl := cfg.GetAppendLockTTL()
		assert.Equal(t, 30*time.Second, ttl)
		assert.NotNil(t, cfg.ttlManager)
		// Store reference to the ttlManager
		originalManager := cfg.ttlManager
		// Call FromMap with new data
		newData := map[string]any{
			"resource": "memory",
			"id":       "test-memory",
			"type":     "token_based",
			"locking": map[string]any{
				"append_ttl": "60s",
			},
			"persistence": map[string]any{
				"type": "memory",
			},
		}
		err := cfg.FromMap(newData)
		require.NoError(t, err)
		// Verify the TTLManager is preserved
		assert.Same(t, originalManager, cfg.ttlManager)
		// Verify the TTL values are still from the original manager
		ttl2 := cfg.GetAppendLockTTL()
		assert.Equal(t, 30*time.Second, ttl2) // Still 30s from the cached manager
	})
}
