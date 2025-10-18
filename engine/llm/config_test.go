package llm

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate_ErrorPaths(t *testing.T) {
	t.Run("Should error when ProxyURL is empty", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ProxyURL = ""
		err := cfg.Validate(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "proxy URL cannot be empty")
	})
	t.Run("Should error when CacheTTL is negative", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.CacheTTL = -1 * time.Second
		err := cfg.Validate(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "cache TTL cannot be negative")
	})
	t.Run("Should error when Timeout is non-positive", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Timeout = 0
		err := cfg.Validate(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "timeout must be positive")
	})
	t.Run("Should error when MaxConcurrentTools is non-positive", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxConcurrentTools = 0
		err := cfg.Validate(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "max concurrent tools must be positive")
	})
	t.Run("Should error when ResolvedTools has empty ID", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ResolvedTools = []tool.Config{{ID: ""}}
		err := cfg.Validate(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "empty ID")
	})
	t.Run("Should error when ResolvedTools has duplicate IDs", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ResolvedTools = []tool.Config{{ID: "dup"}, {ID: "dup"}}
		err := cfg.Validate(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "duplicate tool ID")
		assert.ErrorContains(t, err, "dup")
	})
}

func TestWithResolvedTools_DeepCopy(t *testing.T) {
	t.Run("Should deep-copy ResolvedTools slice to avoid external mutation", func(t *testing.T) {
		original := []tool.Config{{ID: "a"}, {ID: "b"}}
		cfg := DefaultConfig()
		WithResolvedTools(original)(cfg)
		original[0].ID = "mutated-a"
		original = append(original, tool.Config{ID: "c"})
		require.Len(t, cfg.ResolvedTools, 2)
		assert.Equal(t, "a", cfg.ResolvedTools[0].ID)
		assert.Equal(t, "b", cfg.ResolvedTools[1].ID)
		for i := range original {
			original[i].ID = "x"
		}
		assert.Equal(t, "a", cfg.ResolvedTools[0].ID)
		assert.Equal(t, "b", cfg.ResolvedTools[1].ID)
	})
}
