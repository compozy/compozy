package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
)

// TestResolvedKeyForRESTAPI verifies that REST API operations use ResolvedKey correctly
func TestResolvedKeyForRESTAPI(t *testing.T) {
	t.Run("Should use ResolvedKey when provided (REST API case)", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager
		manager := &Manager{
			tplEngine:              engine,
			projectContextResolver: NewProjectContextResolver("fallback-project"),
		}

		// REST API provides explicit keys in ResolvedKey field
		explicitKey := "user:api_test_user"
		memRef := core.MemoryReference{
			ID:          "user_memory",
			ResolvedKey: explicitKey, // REST API sets this
			// Key field is empty for REST API
		}

		// Empty workflow context (REST API doesn't need template variables)
		workflowContext := map[string]any{}

		// Call resolveMemoryKey
		validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, workflowContext)

		// Verify the explicit key was used directly (after validation)
		assert.NoError(t, err)
		assert.Equal(t, explicitKey, validatedKey, "Should use ResolvedKey directly without modification")
	})

	t.Run("Should resolve template when ResolvedKey is empty (workflow case)", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager
		manager := &Manager{
			tplEngine:              engine,
			projectContextResolver: NewProjectContextResolver("fallback-project"),
		}

		// Workflow provides template in Key field
		memRef := core.MemoryReference{
			ID:  "user_memory",
			Key: "user:{{.user_id}}", // Template for workflows
			// ResolvedKey is empty
		}

		// Workflow context with template variables
		workflowContext := map[string]any{
			"user_id": "workflow_user_123",
		}

		// Call resolveMemoryKey
		validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, workflowContext)

		// Verify the template was resolved
		assert.NoError(t, err)
		expectedResolvedKey := "user:workflow_user_123"
		assert.Equal(t, expectedResolvedKey, validatedKey, "Should resolve template from Key field")
	})

	t.Run("Should support any key format for REST API", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager
		manager := &Manager{
			tplEngine:              engine,
			projectContextResolver: NewProjectContextResolver("fallback-project"),
		}

		// Test various key formats that users might want (now with validation)
		testCases := []string{
			"user:123",
			"session:abc-def-ghi",
			"my-custom-key",
			"feature-flag:new-ui",
		}

		for _, explicitKey := range testCases {
			memRef := core.MemoryReference{
				ID:          "generic_memory",
				ResolvedKey: explicitKey,
			}

			// Call resolveMemoryKey
			validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, map[string]any{})

			// Verify each valid key format is preserved as-is
			assert.NoError(t, err)
			assert.Equal(t, explicitKey, validatedKey, "Should handle key format: %s", explicitKey)
		}
	})
}

// TestKeyValidationEdgeCases tests edge cases and validation failures
func TestKeyValidationEdgeCases(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatText)
	manager := &Manager{
		tplEngine:              engine,
		projectContextResolver: NewProjectContextResolver("fallback-project"),
	}

	t.Run("Should reject keys exceeding 256 characters", func(t *testing.T) {
		longKey := strings.Repeat("a", 257) // 257 characters
		memRef := core.MemoryReference{
			ID:          "test_memory",
			ResolvedKey: longKey,
		}

		validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, map[string]any{})
		assert.Error(t, err, "Should return error for keys longer than 256 characters")
		assert.Empty(t, validatedKey, "Should reject keys longer than 256 characters")
	})

	t.Run("Should accept keys at 256 character limit", func(t *testing.T) {
		maxKey := strings.Repeat("a", 256) // Exactly 256 characters
		memRef := core.MemoryReference{
			ID:          "test_memory",
			ResolvedKey: maxKey,
		}

		validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, map[string]any{})
		assert.NoError(t, err, "Should accept keys exactly at 256 character limit")
		assert.Equal(t, maxKey, validatedKey, "Should accept keys exactly at 256 character limit")
	})

	t.Run("Should reject keys with invalid characters", func(t *testing.T) {
		invalidKeys := []string{
			"user#invalid",  // Hash symbol
			"user$invalid",  // Dollar symbol
			"user%invalid",  // Percent symbol
			"user spaces",   // Spaces
			"user/invalid",  // Forward slash
			"user\\invalid", // Backslash
			"user|invalid",  // Pipe
			"user<invalid>", // Angle brackets
			"user[invalid]", // Square brackets
			"user{invalid}", // Curly braces
		}

		for _, invalidKey := range invalidKeys {
			memRef := core.MemoryReference{
				ID:          "test_memory",
				ResolvedKey: invalidKey,
			}

			validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, map[string]any{})
			assert.Error(t, err, "Should return error for key with invalid character: %s", invalidKey)
			assert.Empty(t, validatedKey, "Should reject key with invalid character: %s", invalidKey)
		}
	})

	t.Run("Should reject keys starting or ending with double underscores", func(t *testing.T) {
		invalidKeys := []string{
			"__prefix_key",
			"suffix_key__",
			"__both__",
		}

		for _, invalidKey := range invalidKeys {
			memRef := core.MemoryReference{
				ID:          "test_memory",
				ResolvedKey: invalidKey,
			}

			validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, map[string]any{})
			assert.Error(t, err, "Should return error for key with double underscores: %s", invalidKey)
			assert.Empty(t, validatedKey, "Should reject key with double underscores: %s", invalidKey)
		}
	})

	t.Run("Should handle template resolution failures gracefully", func(t *testing.T) {
		memRef := core.MemoryReference{
			ID:  "test_memory",
			Key: "user:{{.nonexistent.field}}", // Invalid template
		}

		workflowContext := map[string]any{
			"user_id": "test123",
		}

		validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, workflowContext)
		// Should reject invalid template due to curly braces not being valid characters
		assert.Error(t, err, "Should return error for templates with invalid characters after failed resolution")
		assert.Empty(t, validatedKey, "Should reject templates with invalid characters after failed resolution")
	})

	t.Run("Should handle empty keys", func(t *testing.T) {
		memRef := core.MemoryReference{
			ID:          "test_memory",
			ResolvedKey: "",
			Key:         "",
		}

		validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, map[string]any{})
		assert.Error(t, err, "Should return error for empty keys")
		assert.Empty(t, validatedKey, "Should reject empty keys")
	})
}

// TestConcurrentKeyResolution tests concurrent access patterns
func TestConcurrentKeyResolution(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatText)
	manager := &Manager{
		tplEngine:              engine,
		projectContextResolver: NewProjectContextResolver("fallback-project"),
	}

	t.Run("Should handle concurrent key resolution safely", func(t *testing.T) {
		numGoroutines := 100
		results := make(chan string, numGoroutines)

		for i := range numGoroutines {
			go func(_ int) {
				memRef := core.MemoryReference{
					ID:  "test_memory",
					Key: "user:{{.user_id}}",
				}
				workflowContext := map[string]any{
					"user_id": "concurrent_test_user",
				}

				validatedKey, err := manager.resolveMemoryKey(context.Background(), memRef, workflowContext)
				if err != nil {
					results <- "ERROR"
				} else {
					results <- validatedKey
				}
			}(i)
		}

		// Collect all results
		for range numGoroutines {
			result := <-results
			assert.Equal(
				t,
				"user:concurrent_test_user",
				result,
				"All concurrent resolutions should produce same result",
			)
		}
	})
}
