package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreTypes "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/tokens"
	"github.com/compozy/compozy/pkg/logger"
)

func TestUnifiedTokenCounter_Integration(t *testing.T) {
	t.Run("Should create UnifiedTokenCounter with provider config", func(t *testing.T) {
		// Test the UnifiedTokenCounter directly
		log := logger.NewForTests()

		// Create provider config
		providerConfig := &tokens.ProviderConfig{
			Provider: "openai",
			Model:    "gpt-4",
			// No API key - should fallback to tiktoken
		}

		// Create fallback counter
		fallbackCounter, err := tokens.NewTiktokenCounter("gpt-4")
		require.NoError(t, err)

		// Create unified counter
		unifiedCounter, err := tokens.NewUnifiedTokenCounter(providerConfig, fallbackCounter, log)
		require.NoError(t, err)
		require.NotNil(t, unifiedCounter)

		// Test token counting (should use fallback since no API key)
		ctx := context.Background()
		text := "Hello, this is a test message for token counting."
		count, err := unifiedCounter.CountTokens(ctx, text)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "Token count should be positive")

		// Test encoding
		encoding := unifiedCounter.GetEncoding()
		assert.Equal(t, "openai-gpt-4", encoding)
	})

	t.Run("Should integrate UnifiedTokenCounter in memory system", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create a standard memory configuration
		testConfig := &memory.Config{
			Resource:    "memory",
			ID:          "test-unified-counter",
			Type:        memcore.TokenBasedMemory,
			Description: "Test memory with unified token counter",
			MaxTokens:   4000,
			MaxMessages: 100,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "1h",
			},
		}

		// Validate and register config
		err := testConfig.Validate()
		require.NoError(t, err)
		err = env.configRegistry.Register(testConfig, "test")
		require.NoError(t, err)

		// Get memory instance (will use tiktoken by default)
		memRef := coreTypes.MemoryReference{
			ID:  "test-unified-counter",
			Key: "unified-counter-test-1",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
		}

		manager, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Add messages and verify token counting works
		messages := []llm.Message{
			{Role: llm.MessageRoleSystem, Content: "You are a helpful assistant."},
			{Role: llm.MessageRoleUser, Content: "Hello, how are you today?"},
			{
				Role:    llm.MessageRoleAssistant,
				Content: "I'm doing well, thank you for asking! How can I help you today?",
			},
		}

		var tokenCounts []int
		for _, msg := range messages {
			err := manager.Append(ctx, msg)
			require.NoError(t, err)

			count, err := manager.GetTokenCount(ctx)
			require.NoError(t, err)
			tokenCounts = append(tokenCounts, count)
		}

		// Verify token counts are reasonable and increasing
		assert.Greater(t, tokenCounts[0], 0, "First message should have tokens")
		assert.Greater(t, tokenCounts[1], tokenCounts[0], "Token count should increase")
		assert.Greater(t, tokenCounts[2], tokenCounts[1], "Token count should keep increasing")

		// Read back messages
		readMessages, err := manager.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, readMessages, 3)
	})

	t.Run("Should fallback to tiktoken when no TokenProvider configured", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Use existing customer-support config (no TokenProvider)
		memRef := coreTypes.MemoryReference{
			ID:  "customer-support",
			Key: "tiktoken-fallback-test",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
		}

		manager, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Add a message
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "This message will be counted using tiktoken",
		}
		err = manager.Append(ctx, msg)
		require.NoError(t, err)

		// Verify token count
		count, err := manager.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "Should have positive token count")
	})

	t.Run("Should handle different providers in UnifiedTokenCounter", func(t *testing.T) {
		log := logger.NewForTests()
		ctx := context.Background()

		providers := []struct {
			name     string
			provider string
			model    string
		}{
			{"anthropic", "anthropic", "claude-3-haiku"},
			{"google", "google", "gemini-pro"},
			{"cohere", "cohere", "command"},
			{"deepseek", "deepseek", "deepseek-v2"},
		}

		for _, tc := range providers {
			t.Run(tc.name, func(t *testing.T) {
				// Create provider config
				providerConfig := &tokens.ProviderConfig{
					Provider: tc.provider,
					Model:    tc.model,
					// No API key - will use fallback
				}

				// Create fallback counter
				fallbackCounter, err := tokens.NewTiktokenCounter("gpt-4")
				require.NoError(t, err)

				// Create unified counter
				unifiedCounter, err := tokens.NewUnifiedTokenCounter(providerConfig, fallbackCounter, log)
				require.NoError(t, err)
				require.NotNil(t, unifiedCounter)

				// Test token counting
				text := "Testing " + tc.name + " provider token counting with a sample message."
				count, err := unifiedCounter.CountTokens(ctx, text)
				require.NoError(t, err)
				assert.Greater(t, count, 0, "Should have positive token count for "+tc.name)

				// Test encoding
				encoding := unifiedCounter.GetEncoding()
				expected := tc.provider + "-" + tc.model
				assert.Equal(t, expected, encoding)
			})
		}
	})

	t.Run("Should use API key from environment when configured", func(t *testing.T) {
		log := logger.NewForTests()
		ctx := context.Background()

		// Set test API key in environment
		t.Setenv("TEST_OPENAI_API_KEY", "test-key-12345")

		// Create provider config with API key from env
		providerConfig := &memcore.TokenProviderConfig{
			Provider:  "openai",
			Model:     "gpt-3.5-turbo",
			APIKeyEnv: "TEST_OPENAI_API_KEY",
		}

		// Create API key resolver
		keyResolver := tokens.NewAPIKeyResolver(log)

		// Resolve provider configuration
		resolvedConfig := keyResolver.ResolveProviderConfig(providerConfig)
		require.NotNil(t, resolvedConfig)
		assert.Equal(t, "test-key-12345", resolvedConfig.APIKey)
		assert.Equal(t, "openai", resolvedConfig.Provider)
		assert.Equal(t, "gpt-3.5-turbo", resolvedConfig.Model)

		// Create fallback counter
		fallbackCounter, err := tokens.NewTiktokenCounter("gpt-4")
		require.NoError(t, err)

		// Create unified counter with resolved config
		unifiedCounter, err := tokens.NewUnifiedTokenCounter(resolvedConfig, fallbackCounter, log)
		require.NoError(t, err)
		require.NotNil(t, unifiedCounter)

		// Test token counting (will fallback to tiktoken since API key is fake)
		text := "This should use the API key from environment for token counting."
		count, err := unifiedCounter.CountTokens(ctx, text)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "Should have positive token count")
	})
}
