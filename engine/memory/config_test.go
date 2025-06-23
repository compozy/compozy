package memory

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryConfig_Validate(t *testing.T) {
	validPersistence := PersistenceConfig{Type: RedisPersistence, TTL: "24h"}
	validFlushing := &FlushingStrategyConfig{
		Type:                   HybridSummaryFlushing,
		SummarizeThreshold:     0.8,
		SummaryTokens:          500,
		SummarizeOldestPercent: 0.3,
	}

	t.Run("Valid full configuration", func(t *testing.T) {
		cfg := &Config{
			Resource:    "memory",
			ID:          "test-mem",
			Type:        TokenBasedMemory,
			MaxTokens:   2000,
			Persistence: validPersistence,
			Flushing:    validFlushing,
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid resource type", func(t *testing.T) {
		cfg := &Config{Resource: "not-memory", ID: "test-mem", Type: TokenBasedMemory, Persistence: validPersistence}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resource field must be 'memory'")
	})

	t.Run("Empty resource field", func(t *testing.T) {
		cfg := &Config{Resource: "", ID: "test-mem", Type: TokenBasedMemory, Persistence: validPersistence}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resource field must be 'memory'")
		assert.Contains(t, err.Error(), "got ''")
	})

	t.Run("Invalid persistence TTL format", func(t *testing.T) {
		cfg := &Config{
			Resource:    "memory",
			ID:          "test-mem",
			Type:        TokenBasedMemory,
			Persistence: PersistenceConfig{Type: RedisPersistence, TTL: "invalid-duration"},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid persistence.ttl duration format")
	})

	t.Run("Invalid persistence TTL value (non-positive)", func(t *testing.T) {
		cfg := &Config{
			Resource:    "memory",
			ID:          "test-mem",
			Type:        TokenBasedMemory,
			Persistence: PersistenceConfig{Type: RedisPersistence, TTL: "0s"},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "persistence.ttl must be positive")
	})

	t.Run("Missing persistence TTL for redis", func(t *testing.T) {
		cfg := &Config{
			Resource:    "memory",
			ID:          "test-mem",
			Type:        TokenBasedMemory,
			Persistence: PersistenceConfig{Type: RedisPersistence, TTL: ""}, // TTL is empty
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "persistence.ttl is required for persistence type 'redis'")
	})

	t.Run("Valid InMemory persistence with no TTL (or 0s TTL)", func(t *testing.T) {
		cfg := &Config{
			Resource:    "memory",
			ID:          "test-mem-inmem-no-ttl",
			Type:        BufferMemory,
			Persistence: PersistenceConfig{Type: InMemoryPersistence, TTL: ""}, // Empty TTL
		}
		err := cfg.Validate()
		assert.NoError(t, err) // Should be valid as TTL is not strictly required for InMemory

		cfg2 := &Config{
			Resource:    "memory",
			ID:          "test-mem-inmem-zero-ttl",
			Type:        BufferMemory,
			Persistence: PersistenceConfig{Type: InMemoryPersistence, TTL: "0s"},
		}
		err2 := cfg2.Validate()
		// The current Validate() for PersistenceConfig requires positive TTL if not InMemory.
		// If TTL is "0s", time.ParseDuration is fine, but then it hits the "must be positive" check.
		// This needs clarification: should "0s" for InMemory be allowed to mean "no expiry"?
		// Current check `if parsedTTL <= 0 && c.Persistence.Type != InMemoryPersistence` allows 0s for InMemory
		assert.NoError(t, err2, "0s TTL for InMemory should be treated as no expiry / valid by current logic")
	})

	t.Run("Invalid flushing SummarizeThreshold (too low)", func(t *testing.T) {
		cfg := &Config{
			Resource: "memory", ID: "test-mem", Type: TokenBasedMemory, Persistence: validPersistence,
			Flushing: &FlushingStrategyConfig{Type: HybridSummaryFlushing, SummarizeThreshold: 0.0},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "flushing.summarize_threshold (0.00) must be > 0 and <= 1")
	})

	t.Run("Invalid flushing SummarizeThreshold (too high)", func(t *testing.T) {
		cfg := &Config{
			Resource: "memory", ID: "test-mem", Type: TokenBasedMemory, Persistence: validPersistence,
			Flushing: &FlushingStrategyConfig{Type: HybridSummaryFlushing, SummarizeThreshold: 1.1},
		}
		err := cfg.Validate()
		require.Error(t, err)
		// Note: struct tag validate:"omitempty,gt=0,lte=1" on MaxContextRatio is similar.
		// The custom validation is more explicit for SummarizeThreshold.
		assert.Contains(t, err.Error(), "flushing.summarize_threshold (1.10) must be > 0 and <= 1")
	})

	t.Run("Invalid flushing SummaryTokens (non-positive)", func(t *testing.T) {
		cfg := &Config{
			Resource: "memory", ID: "test-mem", Type: TokenBasedMemory, Persistence: validPersistence,
			Flushing: &FlushingStrategyConfig{Type: HybridSummaryFlushing, SummarizeThreshold: 0.8, SummaryTokens: 0},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "flushing.summary_tokens (0) must be > 0")
	})

	t.Run("TokenBasedMemory with no limits should error", func(t *testing.T) {
		cfg := &Config{
			Resource:    "memory",
			ID:          "test-mem-no-token-limit",
			Type:        TokenBasedMemory,
			Persistence: validPersistence,
			// MaxTokens: 0, MaxContextRatio: 0, MaxMessages: 0 by default
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token_based memory must have at least one limit configured")
	})

	t.Run("TokenBasedMemory with MaxTokens only should be valid", func(t *testing.T) {
		cfg := &Config{
			Resource:    "memory",
			ID:          "test-mem-max-tokens",
			Type:        TokenBasedMemory,
			MaxTokens:   1000,
			Persistence: validPersistence,
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("TokenBasedMemory with MaxContextRatio only should be valid", func(t *testing.T) {
		cfg := &Config{
			Resource:        "memory",
			ID:              "test-mem-max-context-ratio",
			Type:            TokenBasedMemory,
			MaxContextRatio: 0.8,
			Persistence:     validPersistence,
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("TokenBasedMemory with MaxMessages only should be valid", func(t *testing.T) {
		cfg := &Config{
			Resource:    "memory",
			ID:          "test-mem-max-messages",
			Type:        TokenBasedMemory,
			MaxMessages: 100,
			Persistence: validPersistence,
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("TokenBasedMemory with multiple limits should be valid", func(t *testing.T) {
		cfg := &Config{
			Resource:        "memory",
			ID:              "test-mem-multiple-limits",
			Type:            TokenBasedMemory,
			MaxTokens:       1000,
			MaxContextRatio: 0.8,
			MaxMessages:     100,
			Persistence:     validPersistence,
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("Non-TokenBasedMemory types should not require token limits", func(t *testing.T) {
		// MessageCountBasedMemory without token limits should be valid
		cfg1 := &Config{
			Resource:    "memory",
			ID:          "test-mem-message-count",
			Type:        MessageCountBasedMemory,
			MaxMessages: 50,
			Persistence: validPersistence,
		}
		err1 := cfg1.Validate()
		assert.NoError(t, err1)

		// BufferMemory without any limits should be valid
		cfg2 := &Config{
			Resource:    "memory",
			ID:          "test-mem-buffer",
			Type:        BufferMemory,
			Persistence: validPersistence,
		}
		err2 := cfg2.Validate()
		assert.NoError(t, err2)
	})

	// Test core.Configurable methods
	t.Run("Implements core.Configurable methods", func(t *testing.T) {
		cfg := &Config{Resource: "memory", ID: "mem-id"}
		assert.Equal(t, "memory", cfg.GetResource())
		assert.Equal(t, "mem-id", cfg.GetID())
		assert.Equal(t, core.ConfigMemory, cfg.Component())

		err := cfg.SetCWD(".")
		require.NoError(t, err)
		assert.NotNil(t, cfg.GetCWD())
		assert.NotEmpty(t, cfg.GetCWD().PathStr())

		cfg.SetFilePath("./memories/test.yaml")
		assert.Equal(t, "./memories/test.yaml", cfg.GetFilePath())
	})
}

func TestPersistenceConfig_ParsedTTL(t *testing.T) {
	// This test is more conceptual as ParsedTTL is not set by Validate() in memory.Config
	// but would be set by a consumer of the config.
	// However, we can test time.ParseDuration here.
	pc := PersistenceConfig{TTL: "30m"}
	parsed, err := time.ParseDuration(pc.TTL)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, parsed)

	pcInvalid := PersistenceConfig{TTL: "invalid"}
	_, errInvalid := time.ParseDuration(pcInvalid.TTL)
	assert.Error(t, errInvalid)
}
