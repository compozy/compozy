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
)

func TestTokenProviderConfig_Integration(t *testing.T) {
	t.Run("Should use TokenProvider configuration from memory config", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create a memory configuration with TokenProvider
		testConfig := &memory.Config{
			Resource:    "memory",
			ID:          "test-token-provider-config",
			Type:        memcore.TokenBasedMemory,
			Description: "Test memory with TokenProvider configuration",
			MaxTokens:   4000,
			MaxMessages: 100,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "1h",
			},
			TokenProvider: &memcore.TokenProviderConfig{
				Provider: "openai",
				Model:    "gpt-4",
				// No API key - should fallback to tiktoken
			},
		}

		// Validate and register config
		err := testConfig.Validate()
		require.NoError(t, err)
		err = env.configRegistry.Register(testConfig, "test")
		require.NoError(t, err)

		// Get memory instance
		memRef := coreTypes.MemoryReference{
			ID:  "test-token-provider-config",
			Key: "token-provider-config-test",
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

		for _, msg := range messages {
			err := manager.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Verify token count
		count, err := manager.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "Should have positive token count")

		// Read back messages
		readMessages, err := manager.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, readMessages, 3)
	})

	t.Run("Should use API key from environment variable in TokenProvider", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Set test API key in environment
		t.Setenv("TEST_ANTHROPIC_API_KEY", "test-anthropic-key-123")

		// Create a memory configuration with TokenProvider using env var
		testConfig := &memory.Config{
			Resource:    "memory",
			ID:          "test-token-provider-env",
			Type:        memcore.TokenBasedMemory,
			Description: "Test memory with TokenProvider using env var",
			MaxTokens:   3000,
			MaxMessages: 50,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "30m",
			},
			TokenProvider: &memcore.TokenProviderConfig{
				Provider:  "anthropic",
				Model:     "claude-3-haiku",
				APIKeyEnv: "TEST_ANTHROPIC_API_KEY",
			},
		}

		// Validate and register config
		err := testConfig.Validate()
		require.NoError(t, err)
		err = env.configRegistry.Register(testConfig, "test")
		require.NoError(t, err)

		// Get memory instance
		memRef := coreTypes.MemoryReference{
			ID:  "test-token-provider-env",
			Key: "token-provider-env-test",
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
			Content: "Testing Anthropic provider with API key from environment",
		}
		err = manager.Append(ctx, msg)
		require.NoError(t, err)

		// Verify token count (will use fallback since API key is fake)
		count, err := manager.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "Should have positive token count")
	})

	t.Run("Should handle multiple providers in different memory configs", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		providers := []struct {
			id       string
			provider string
			model    string
		}{
			{"test-google-provider", "google", "gemini-pro"},
			{"test-cohere-provider", "cohere", "command"},
			{"test-deepseek-provider", "deepseek", "deepseek-v2"},
		}

		// Register all configs
		for _, tc := range providers {
			config := &memory.Config{
				Resource:    "memory",
				ID:          tc.id,
				Type:        memcore.TokenBasedMemory,
				Description: "Test memory with " + tc.provider + " provider",
				MaxTokens:   2000,
				MaxMessages: 40,
				Persistence: memcore.PersistenceConfig{
					Type: memcore.RedisPersistence,
					TTL:  "20m",
				},
				TokenProvider: &memcore.TokenProviderConfig{
					Provider: tc.provider,
					Model:    tc.model,
				},
			}

			err := config.Validate()
			require.NoError(t, err)
			err = env.configRegistry.Register(config, "test")
			require.NoError(t, err)
		}

		// Test each provider
		for _, tc := range providers {
			t.Run(tc.provider, func(t *testing.T) {
				memRef := coreTypes.MemoryReference{
					ID:  tc.id,
					Key: tc.provider + "-test-key",
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
					Content: "Testing " + tc.provider + " provider token counting",
				}
				err = manager.Append(ctx, msg)
				require.NoError(t, err)

				// Verify token count
				count, err := manager.GetTokenCount(ctx)
				require.NoError(t, err)
				assert.Greater(t, count, 0, "Should have positive token count for "+tc.provider)
			})
		}
	})
}
