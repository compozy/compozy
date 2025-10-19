package memory

import (
	"testing"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoload_MemoryResources(t *testing.T) {
	t.Run("Should discover and load memory resource files", func(t *testing.T) {
		// Arrange
		config := &autoload.Config{
			Enabled: true,
			Include: []string{"test/fixtures/memory/*.yaml"},
			Exclude: autoload.DefaultExcludes,
		}

		cwd, err := core.CWDFromPath("../../..")
		require.NoError(t, err)

		registry := autoload.NewConfigRegistry()
		autoloader := autoload.New(cwd.Path, config, registry)

		// Act
		err = autoloader.Load(t.Context())

		// Assert
		assert.NoError(t, err)

		// Verify test_memory resource loaded
		testMemory, err := registry.Get("memory", "test_memory")
		assert.NoError(t, err)
		assert.NotNil(t, testMemory)

		// Verify conversation_memory resource loaded
		convMemory, err := registry.Get("memory", "conversation_memory")
		assert.NoError(t, err)
		assert.NotNil(t, convMemory)
	})

	t.Run("Should validate memory resource structure", func(t *testing.T) {
		// Arrange
		registry := autoload.NewConfigRegistry()
		memoryConfig := map[string]any{
			"resource":    "memory",
			"id":          "valid_memory",
			"description": "Valid memory configuration",
			"version":     "0.1.0",
			"key":         "test:{{.id}}",
			"type":        "token_based",
			"max_tokens":  1000,
			"persistence": map[string]any{
				"type": "redis",
				"ttl":  "1h",
			},
		}

		// Act
		err := registry.Register(memoryConfig, "test/valid_memory.yaml")

		// Assert
		assert.NoError(t, err)

		retrieved, err := registry.Get("memory", "valid_memory")
		assert.NoError(t, err)
		assert.Equal(t, "valid_memory", retrieved.(map[string]any)["id"])
	})
}
