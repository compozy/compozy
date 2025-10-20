package uc

import (
	"testing"

	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/service"
	"github.com/compozy/compozy/engine/memory/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsMemory_TokenLimitFromConfig(t *testing.T) {
	t.Run("Should use MaxTokens from memory configuration", func(t *testing.T) {
		// Setup real Redis environment
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		// Create a memory configuration with specific MaxTokens
		testConfig := &memory.Config{
			Resource:    "memory",
			ID:          "test_memory",
			Type:        "token_based",
			Description: "Test memory with custom token limit",
			MaxTokens:   2048, // Custom token limit
			MaxMessages: 100,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
		}

		// Validate and register config
		err := testConfig.Validate(t.Context())
		require.NoError(t, err)
		err = setup.ConfigRegistry.Register(testConfig, "test")
		require.NoError(t, err)

		// Create stats memory use case
		statsUC, err := NewStatsMemory(setup.Manager, nil, nil)
		require.NoError(t, err)

		// Execute stats operation
		ctx := t.Context()
		input := StatsMemoryInput{
			MemoryRef: "test_memory",
			Key:       "test_key",
		}

		result, err := statsUC.Execute(ctx, input)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify that token limit comes from configuration
		assert.Equal(t, 2048, result.TokenLimit, "Should use MaxTokens from configuration")
		assert.Equal(t, 0.0, result.TokenUtilization, "Token utilization should be 0 for empty memory")
	})

	t.Run("Should use default token limit when MaxTokens not configured", func(t *testing.T) {
		// Setup real Redis environment
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		// Create a memory configuration without MaxTokens
		testConfig := &memory.Config{
			Resource:    "memory",
			ID:          "test_memory_no_limit",
			Type:        "message_count_based",
			Description: "Test memory without token limit",
			MaxMessages: 100,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
		}

		// Validate and register config
		err := testConfig.Validate(t.Context())
		require.NoError(t, err)
		err = setup.ConfigRegistry.Register(testConfig, "test")
		require.NoError(t, err)

		// Create stats memory use case
		statsUC, err := NewStatsMemory(setup.Manager, nil, nil)
		require.NoError(t, err)

		// Execute stats operation
		ctx := t.Context()
		input := StatsMemoryInput{
			MemoryRef: "test_memory_no_limit",
			Key:       "test_key",
		}

		result, err := statsUC.Execute(ctx, input)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify that default token limit is used
		assert.Equal(t, 128000, result.TokenLimit, "Should use default token limit when not configured")
	})

	t.Run("Should calculate token utilization correctly", func(t *testing.T) {
		// Setup real Redis environment
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		// Create a memory configuration with specific MaxTokens
		testConfig := &memory.Config{
			Resource:    "memory",
			ID:          "test_memory_utilization",
			Type:        "token_based",
			Description: "Test memory for utilization calculation",
			MaxTokens:   1000, // Small limit for easy calculation
			MaxMessages: 100,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
		}

		// Validate and register config
		err := testConfig.Validate(t.Context())
		require.NoError(t, err)
		err = setup.ConfigRegistry.Register(testConfig, "test")
		require.NoError(t, err)

		// Create memory service and append some messages
		memService, err := service.NewMemoryOperationsService(setup.Manager, nil, nil, nil, nil)
		require.NoError(t, err)
		ctx := t.Context()

		// Append a message to generate some token count
		appendReq := &service.AppendRequest{
			BaseRequest: service.BaseRequest{
				MemoryRef: "test_memory_utilization",
				Key:       "test_key",
			},
			Payload: map[string]any{
				"role":    "user",
				"content": "This is a test message with some content to generate token count",
			},
		}
		_, err = memService.Append(ctx, appendReq)
		require.NoError(t, err)

		// Create stats memory use case
		statsUC, err := NewStatsMemory(setup.Manager, nil, nil)
		require.NoError(t, err)

		// Execute stats operation
		input := StatsMemoryInput{
			MemoryRef: "test_memory_utilization",
			Key:       "test_key",
		}

		result, err := statsUC.Execute(ctx, input)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify token limit and utilization
		assert.Equal(t, 1000, result.TokenLimit, "Should use MaxTokens from configuration")
		assert.Greater(t, result.TokenCount, 0, "Should have non_zero token count")
		assert.Greater(t, result.TokenUtilization, 0.0, "Should have non_zero utilization")
		assert.LessOrEqual(t, result.TokenUtilization, 1.0, "Utilization should not exceed 100%")

		// Verify utilization calculation
		expectedUtilization := float64(result.TokenCount) / float64(result.TokenLimit)
		assert.Equal(
			t,
			expectedUtilization,
			result.TokenUtilization,
			"Token utilization should be correctly calculated",
		)
	})
}
