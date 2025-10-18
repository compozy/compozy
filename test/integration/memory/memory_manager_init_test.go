package memory

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryManager_ResourceRegistryIntegration(t *testing.T) {
	t.Run("Should validate memory configuration can be retrieved from registry", func(t *testing.T) {
		// Arrange
		registry := createTestRegistry(t)

		// Register a test memory configuration
		memoryConfig := map[string]any{
			"resource":   "memory",
			"id":         "test_instance",
			"key":        "test:{{.id}}",
			"type":       "token_based",
			"max_tokens": 1000,
		}
		err := registry.Register(memoryConfig, "test.yaml")
		require.NoError(t, err)

		// Act - Verify configuration can be retrieved
		retrieved, err := registry.Get("memory", "test_instance")

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)

		configMap, ok := retrieved.(map[string]any)
		require.True(t, ok, "Retrieved config should be a map[string]any")
		assert.Equal(t, "test_instance", configMap["id"])
		assert.Equal(t, "memory", configMap["resource"])
		assert.Equal(t, "token_based", configMap["type"])
		assert.Equal(t, 1000, configMap["max_tokens"])
	})

	t.Run("Should load memory configurations from autoload", func(t *testing.T) {
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

		// Act - Load memory configurations
		err = autoloader.Load(context.Background())
		require.NoError(t, err)

		// Assert - Verify configurations are available
		memoryConfigs := registry.GetAll("memory")
		assert.GreaterOrEqual(t, len(memoryConfigs), 2, "Should have loaded at least 2 memory configurations")

		// Verify specific configurations exist
		testMemory, err := registry.Get("memory", "test_memory")
		assert.NoError(t, err)
		assert.NotNil(t, testMemory)

		convMemory, err := registry.Get("memory", "conversation_memory")
		assert.NoError(t, err)
		assert.NotNil(t, convMemory)

		// Verify configuration structure
		testConfig, ok := testMemory.(map[string]any)
		require.True(t, ok, "Test memory config should be a map[string]any")
		assert.Equal(t, "test_memory", testConfig["id"])
		assert.Equal(t, "memory", testConfig["resource"])
		assert.Equal(t, "token_based", testConfig["type"])
	})
}

// Helper functions

func createTestRegistry(t *testing.T) *autoload.ConfigRegistry {
	t.Helper()
	registry := autoload.NewConfigRegistry()

	// Add a test memory configuration
	testMemoryConfig := &memory.Config{
		Resource:    "memory",
		ID:          "test_memory",
		Type:        memcore.TokenBasedMemory,
		Description: "Test memory for integration tests",
		MaxTokens:   4000,
		MaxMessages: 100,
		Persistence: memcore.PersistenceConfig{
			Type: memcore.RedisPersistence,
			TTL:  "24h",
		},
		Flushing: &memcore.FlushingStrategyConfig{
			Type:               memcore.SimpleFIFOFlushing,
			SummarizeThreshold: 0.8,
		},
	}

	err := testMemoryConfig.Validate(t.Context())
	require.NoError(t, err)
	err = registry.Register(testMemoryConfig, "test")
	require.NoError(t, err)

	return registry
}

// Note: createTestMemoryManager removed as we're focusing on configuration validation
// rather than full memory manager instantiation in these tests
