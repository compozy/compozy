package memory

import (
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestManager creates a Manager with minimal required fields for testing
func createTestManager(tplEngine *tplengine.TemplateEngine, fallbackProjectID string) *Manager {
	return &Manager{
		tplEngine:              tplEngine,
		projectContextResolver: NewProjectContextResolver(fallbackProjectID),
	}
}

func TestManager_loadMemoryConfig(t *testing.T) {
	t.Run("Should successfully load valid memory resource from registry", func(t *testing.T) {
		// Create a valid memory config that follows autoload pattern
		testConfig := &Config{
			Resource:    "memory",
			ID:          "test-memory",
			Description: "Test memory resource",
			Type:        memcore.TokenBasedMemory,
			MaxTokens:   1000,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
		}

		// Create real registry and register the config
		registry := autoload.NewConfigRegistry()
		err := registry.Register(testConfig, "manual")
		require.NoError(t, err)

		// Create manager with registry
		manager := &Manager{
			resourceRegistry: registry,
		}

		// Call loadMemoryConfig
		result, err := manager.loadMemoryConfig(t.Context(), "test-memory")

		// Verify results
		require.NoError(t, err)
		assert.Equal(t, "test-memory", result.ID)
		assert.Equal(t, "Test memory resource", result.Description)
		assert.Equal(t, memcore.TokenBasedMemory, result.Type)
		assert.Equal(t, 1000, result.MaxTokens)
		assert.Equal(t, memcore.RedisPersistence, result.Persistence.Type)
	})

	t.Run("Should return ConfigError when resource not found in registry", func(t *testing.T) {
		// Create empty registry
		registry := autoload.NewConfigRegistry()

		// Create manager with empty registry
		manager := &Manager{
			resourceRegistry: registry,
		}

		// Call loadMemoryConfig with non-existent resource
		result, err := manager.loadMemoryConfig(t.Context(), "nonexistent-memory")

		// Verify error
		require.Error(t, err)
		assert.Nil(t, result)

		// Check error type and message
		var memErr *Error
		require.ErrorAs(t, err, &memErr)
		assert.Equal(t, ErrorTypeConfig, memErr.Type)
		assert.Equal(t, "load", memErr.Operation)
		assert.Equal(t, "nonexistent-memory", memErr.ResourceID)
		assert.Contains(t, err.Error(), "memory configuration error for resource 'nonexistent-memory' during load")
	})

	t.Run("Should return ConfigError when config has wrong type", func(t *testing.T) {
		// Create a config with wrong type (not a *Config)
		wrongTypeConfig := &struct {
			Resource string
			ID       string
			Value    string
		}{
			Resource: "memory",
			ID:       "wrong-type",
			Value:    "not a memory config",
		}

		// Create registry and register wrong type config
		registry := autoload.NewConfigRegistry()
		err := registry.Register(wrongTypeConfig, "manual")
		require.NoError(t, err)

		// Create manager with registry
		manager := &Manager{
			resourceRegistry: registry,
		}

		// Call loadMemoryConfig
		result, err := manager.loadMemoryConfig(t.Context(), "wrong-type")

		// Verify error
		require.Error(t, err)
		assert.Nil(t, result)

		// Check error type and message
		var memErr *Error
		require.ErrorAs(t, err, &memErr)
		assert.Equal(t, ErrorTypeConfig, memErr.Type)
		assert.Equal(t, "convert", memErr.Operation)
		assert.Equal(t, "wrong-type", memErr.ResourceID)
		assert.Contains(t, err.Error(), "memory configuration error for resource 'wrong-type' during convert")
		assert.Contains(t, err.Error(), "expected map[string]any")
	})

	t.Run("Should handle different memory resource types", func(t *testing.T) {
		testCases := []struct {
			name   string
			config *Config
		}{
			{
				name: "token-based memory",
				config: &Config{
					Resource:  "memory",
					ID:        "token-memory",
					Type:      memcore.TokenBasedMemory,
					MaxTokens: 2000,
					Persistence: memcore.PersistenceConfig{
						Type: memcore.RedisPersistence,
						TTL:  "24h",
					},
				},
			},
			{
				name: "message-count-based memory",
				config: &Config{
					Resource:    "memory",
					ID:          "message-memory",
					Type:        memcore.MessageCountBasedMemory,
					MaxMessages: 50,
					Persistence: memcore.PersistenceConfig{
						Type: memcore.RedisPersistence,
						TTL:  "24h",
					},
				},
			},
			{
				name: "buffer memory",
				config: &Config{
					Resource: "memory",
					ID:       "buffer-memory",
					Type:     memcore.BufferMemory,
					Persistence: memcore.PersistenceConfig{
						Type: memcore.RedisPersistence,
						TTL:  "24h",
					},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create registry and register test config
				registry := autoload.NewConfigRegistry()
				err := registry.Register(tc.config, "manual")
				require.NoError(t, err)

				// Create manager
				manager := &Manager{
					resourceRegistry: registry,
				}

				// Load config
				result, err := manager.loadMemoryConfig(t.Context(), tc.config.ID)

				// Verify success
				require.NoError(t, err)
				assert.Equal(t, tc.config.ID, result.ID)
				assert.Equal(t, tc.config.Type, result.Type)
				assert.Equal(t, tc.config.MaxTokens, result.MaxTokens)
				assert.Equal(t, tc.config.MaxMessages, result.MaxMessages)
				assert.Equal(t, tc.config.Persistence.Type, result.Persistence.Type)
			})
		}
	})

	t.Run("Should handle case insensitive resource ID lookup", func(t *testing.T) {
		// Create memory config with lowercase ID
		testConfig := &Config{
			Resource:  "memory",
			ID:        "case-test-memory",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
		}

		// Create registry and register the config
		registry := autoload.NewConfigRegistry()
		err := registry.Register(testConfig, "manual")
		require.NoError(t, err)

		// Create manager
		manager := &Manager{
			resourceRegistry: registry,
		}

		// Test different case variations
		testCases := []string{
			"case-test-memory",   // exact match
			"CASE-TEST-MEMORY",   // uppercase
			"Case-Test-Memory",   // title case
			" case-test-memory ", // with whitespace
		}

		for _, testID := range testCases {
			result, err := manager.loadMemoryConfig(t.Context(), testID)
			require.NoError(t, err, "Failed for ID: %s", testID)
			assert.Equal(t, testConfig.ID, result.ID)
			assert.Equal(t, testConfig.Type, result.Type)
			assert.Equal(t, testConfig.MaxTokens, result.MaxTokens)
		}
	})

	t.Run("Integration: Should work with complex memory configuration", func(t *testing.T) {
		// Create a complex memory configuration with all optional fields
		complexConfig := &Config{
			Resource:        "memory",
			ID:              "complex-memory",
			Description:     "Complex memory configuration for integration testing",
			Version:         "1.0.0",
			Type:            memcore.TokenBasedMemory,
			MaxTokens:       4000,
			MaxMessages:     100,
			MaxContextRatio: 0.8,
			TokenAllocation: &memcore.TokenAllocation{
				ShortTerm: 0.6,
				LongTerm:  0.3,
				System:    0.1,
			},
			Flushing: &memcore.FlushingStrategyConfig{
				Type:                   memcore.HybridSummaryFlushing,
				SummarizeThreshold:     0.8,
				SummaryTokens:          200,
				SummarizeOldestPercent: 0.3,
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "7d",
			},
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns:         []string{`\b\d{4}-\d{4}-\d{4}-\d{4}\b`}, // Credit card pattern
				DefaultRedactionString: "[REDACTED]",
			},
		}

		// Create registry and register complex config
		registry := autoload.NewConfigRegistry()
		err := registry.Register(complexConfig, "manual")
		require.NoError(t, err)

		// Create manager
		manager := &Manager{
			resourceRegistry: registry,
		}

		// Load and verify complex config
		result, err := manager.loadMemoryConfig(t.Context(), "complex-memory")
		require.NoError(t, err)

		// Verify all fields are properly converted
		assert.Equal(t, "complex-memory", result.ID)
		assert.Equal(t, "Complex memory configuration for integration testing", result.Description)
		assert.Equal(t, memcore.TokenBasedMemory, result.Type)
		assert.Equal(t, 4000, result.MaxTokens)
		assert.Equal(t, 100, result.MaxMessages)
		assert.Equal(t, 0.8, result.MaxContextRatio)

		// Verify nested structures
		require.NotNil(t, result.TokenAllocation)
		assert.Equal(t, 0.6, result.TokenAllocation.ShortTerm)
		assert.Equal(t, 0.3, result.TokenAllocation.LongTerm)
		assert.Equal(t, 0.1, result.TokenAllocation.System)

		require.NotNil(t, result.FlushingStrategy)
		assert.Equal(t, memcore.HybridSummaryFlushing, result.FlushingStrategy.Type)
		assert.Equal(t, 0.8, result.FlushingStrategy.SummarizeThreshold)
		assert.Equal(t, 200, result.FlushingStrategy.SummaryTokens)

		assert.Equal(t, memcore.RedisPersistence, result.Persistence.Type)
		assert.Equal(t, "7d", result.Persistence.TTL)

		require.NotNil(t, result.PrivacyPolicy)
		assert.Equal(t, "[REDACTED]", result.PrivacyPolicy.DefaultRedactionString)
		assert.Len(t, result.PrivacyPolicy.RedactPatterns, 1)
	})
}

func TestManager_resolveMemoryKey(t *testing.T) {
	t.Run("Should resolve simple template with variable substitution", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager with template engine using helper
		manager := createTestManager(engine, "")

		// Test context data
		workflowContext := map[string]any{
			"project.id": "test-project-123",
			"agent": map[string]any{
				"name": "customer-support",
			},
		}

		// Test template with variables
		template := "memory-{{index . \"project.id\"}}-{{.agent.name}}"

		// Call resolveMemoryKey
		memRef := core.MemoryReference{Key: template}
		validatedKey, err := manager.resolveMemoryKey(t.Context(), memRef, workflowContext)

		// Verify results
		assert.NoError(t, err)
		assert.NotEmpty(t, validatedKey)
		projectID := manager.getProjectID(t.Context(), workflowContext)
		assert.Equal(t, "test-project-123", projectID)

		// Verify the resolved template was processed (should contain the resolved key)
		assert.Equal(
			t,
			"memory-test-project-123-customer-support",
			validatedKey,
			"Template should be resolved to the exact expected value",
		)
	})

	t.Run("Should resolve complex template with nested variables", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager using helper
		manager := createTestManager(engine, "")

		// Complex context data
		workflowContext := map[string]any{
			"project.id": "complex-project",
			"project": map[string]any{
				"name": "Complex Project",
			},
			"workflow": map[string]any{
				"id": "wf-456",
			},
			"env": "production",
			"user": map[string]any{
				"id":   "user-789",
				"role": "admin",
			},
		}

		// Complex template
		template := "{{.env}}-{{index . \"project.id\"}}-{{.workflow.id}}-{{.user.role}}"

		// Call resolveMemoryKey
		memRef := core.MemoryReference{Key: template}
		validatedKey, err := manager.resolveMemoryKey(t.Context(), memRef, workflowContext)

		// Verify results
		assert.NoError(t, err)
		assert.NotEmpty(t, validatedKey)
		projectID := manager.getProjectID(t.Context(), workflowContext)
		assert.Equal(t, "complex-project", projectID)
	})

	t.Run("Should handle template without project.id gracefully", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager with project context resolver
		manager := createTestManager(engine, "")

		// Context without project.id
		workflowContext := map[string]any{
			"agent": map[string]any{
				"name": "test-agent",
			},
			"env": "test",
		}

		// Template
		template := "memory-{{.env}}-{{.agent.name}}"

		// Call resolveMemoryKey
		memRef := core.MemoryReference{Key: template}
		validatedKey, err := manager.resolveMemoryKey(t.Context(), memRef, workflowContext)

		// Verify results
		assert.NoError(t, err)
		assert.NotEmpty(t, validatedKey)
		projectID := manager.getProjectID(t.Context(), workflowContext)
		assert.Empty(t, projectID, "Project ID should be empty when not in context")
	})

	t.Run("Should fail validation when template evaluation fails", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager with project context resolver
		manager := createTestManager(engine, "fallback-project")

		// Context with project.id
		workflowContext := map[string]any{
			"project.id": "fallback-project",
		}

		// Invalid template (missing variable)
		invalidTemplate := "memory-{{.nonexistent.variable}}"

		// Call resolveMemoryKey
		memRef := core.MemoryReference{Key: invalidTemplate}
		validatedKey, err := manager.resolveMemoryKey(t.Context(), memRef, workflowContext)

		// Verify results - should fail validation due to invalid characters
		assert.Error(t, err, "Should fail validation due to template syntax in key")
		assert.Empty(t, validatedKey, "Should not return a key when validation fails")
		assert.Contains(t, err.Error(), "memory key validation failed", "Error should indicate validation failure")
		assert.Contains(t, err.Error(), "fallback-project", "Error should include project ID for context")
	})

	t.Run("Should handle empty template gracefully", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager using helper
		manager := createTestManager(engine, "")

		// Context with project.id
		workflowContext := map[string]any{
			"project.id": "empty-test-project",
		}

		// Empty template
		emptyTemplate := ""

		// Call resolveMemoryKey
		memRef := core.MemoryReference{Key: emptyTemplate}
		validatedKey, err := manager.resolveMemoryKey(t.Context(), memRef, workflowContext)

		// Verify results - empty keys should be rejected
		assert.Empty(t, validatedKey, "Should reject empty keys")
		assert.Error(t, err, "Should return error for empty keys")
		assert.Contains(t, err.Error(), "empty-test-project", "Error should include project ID")
	})

	t.Run("Should handle template with missing project.id context", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager with project context resolver
		manager := createTestManager(engine, "")

		// Context missing project.id
		workflowContext := map[string]any{
			"agent": map[string]any{
				"name": "test-agent",
			},
		}

		// Template
		template := "memory-{{.agent.name}}"

		// Call resolveMemoryKey
		memRef := core.MemoryReference{Key: template}
		validatedKey, err := manager.resolveMemoryKey(t.Context(), memRef, workflowContext)

		// Verify results
		assert.NoError(t, err)
		assert.NotEmpty(t, validatedKey)
		projectID := manager.getProjectID(t.Context(), workflowContext)
		assert.Empty(t, projectID)
	})

	t.Run("Should handle literal string without template markers", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)

		// Create manager using helper
		manager := createTestManager(engine, "")

		// Context with project.id
		workflowContext := map[string]any{
			"project.id": "literal-project",
		}

		// Literal string (no template markers)
		literalString := "memory-literal-key"

		// Call resolveMemoryKey
		memRef := core.MemoryReference{Key: literalString}
		validatedKey, err := manager.resolveMemoryKey(t.Context(), memRef, workflowContext)

		// Verify results
		assert.NoError(t, err)
		assert.NotEmpty(t, validatedKey)
		projectID := manager.getProjectID(t.Context(), workflowContext)
		assert.Equal(t, "literal-project", projectID)

		// Should be the literal string itself (now that we don't hash)
		assert.Equal(t, literalString, validatedKey, "Literal strings should be returned as-is")
	})
}

func TestProjectContextResolver(t *testing.T) {
	t.Run("Should extract project ID from flat format", func(t *testing.T) {
		resolver := NewProjectContextResolver("fallback-id")
		workflowContext := map[string]any{
			"project.id": "test-project-id",
		}

		projectID := resolver.ResolveProjectID(t.Context(), workflowContext)
		assert.Equal(t, "test-project-id", projectID)
	})

	t.Run("Should extract project ID from nested format", func(t *testing.T) {
		resolver := NewProjectContextResolver("fallback-id")
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "nested-project-id",
			},
		}

		projectID := resolver.ResolveProjectID(t.Context(), workflowContext)
		assert.Equal(t, "nested-project-id", projectID)
	})

	t.Run("Should use fallback when project.id is missing", func(t *testing.T) {
		resolver := NewProjectContextResolver("fallback-id")
		workflowContext := map[string]any{
			"project.name": "Test Project",
		}

		projectID := resolver.ResolveProjectID(t.Context(), workflowContext)
		assert.Equal(t, "fallback-id", projectID)
	})

	t.Run("Should use fallback when project key is missing", func(t *testing.T) {
		resolver := NewProjectContextResolver("fallback-id")
		workflowContext := map[string]any{
			"agent": map[string]any{
				"name": "test-agent",
			},
		}

		projectID := resolver.ResolveProjectID(t.Context(), workflowContext)
		assert.Equal(t, "fallback-id", projectID)
	})

	t.Run("Should use fallback when project.id is not a string", func(t *testing.T) {
		resolver := NewProjectContextResolver("fallback-id")
		workflowContext := map[string]any{
			"project.id": 123, // not a string
		}

		projectID := resolver.ResolveProjectID(t.Context(), workflowContext)
		assert.Equal(t, "fallback-id", projectID)
	})

	t.Run("Should handle empty context", func(t *testing.T) {
		resolver := NewProjectContextResolver("fallback-id")
		workflowContext := map[string]any{}

		projectID := resolver.ResolveProjectID(t.Context(), workflowContext)
		assert.Equal(t, "fallback-id", projectID)
	})

	t.Run("Should handle nil context", func(t *testing.T) {
		resolver := NewProjectContextResolver("fallback-id")
		projectID := resolver.ResolveProjectID(t.Context(), nil)
		assert.Equal(t, "fallback-id", projectID)
	})
}

func TestManager_configToResource(t *testing.T) {
	t.Run("Should properly map config to resource with TTL fields", func(t *testing.T) {
		// Create config with locking TTL fields
		config := &Config{
			ID:              "test-memory",
			Description:     "Test memory configuration",
			Type:            memcore.TokenBasedMemory,
			MaxTokens:       2000,
			MaxMessages:     50,
			MaxContextRatio: 0.7,
			Locking: &memcore.LockConfig{
				AppendTTL: "15s",
				ClearTTL:  "30s",
				FlushTTL:  "2m",
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "48h",
			},
		}

		builder := &ResourceBuilder{config: config}
		result, err := builder.Build(t.Context())
		require.NoError(t, err)

		// Verify basic fields are mapped correctly
		assert.Equal(t, config.ID, result.ID)
		assert.Equal(t, config.Description, result.Description)
		assert.Equal(t, config.Type, result.Type)
		assert.Equal(t, config.MaxTokens, result.MaxTokens)
		assert.Equal(t, config.MaxMessages, result.MaxMessages)
		assert.Equal(t, config.MaxContextRatio, result.MaxContextRatio)
		assert.Equal(t, config.Persistence, result.Persistence)

		// Verify TTL fields are properly mapped from locking config
		assert.Equal(t, "15s", result.AppendTTL)
		assert.Equal(t, "30s", result.ClearTTL)
		assert.Equal(t, "2m", result.FlushTTL)

		// Verify fields that are intentionally not mapped from config have expected values
		assert.Empty(t, result.Model, "Model should be empty - not specified in memory config")
		assert.Zero(t, result.ModelContextSize, "ModelContextSize should be 0 - not specified in memory config")
		assert.Nil(
			t,
			result.EvictionPolicyConfig,
			"EvictionPolicyConfig should be nil - determined by memory type and flushing strategy",
		)
		assert.Empty(t, result.TokenCounter, "TokenCounter should be empty - determined at runtime")
		assert.Nil(t, result.Metadata, "Metadata should be nil - not stored in config")
		assert.False(t, result.DisableFlush, "DisableFlush should be false - flush enabled by default")
	})

	t.Run("Should handle nil locking config gracefully", func(t *testing.T) {
		config := &Config{
			ID:        "test-memory",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			Locking:   nil, // No locking config
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
		}

		builder := &ResourceBuilder{config: config}
		result, err := builder.Build(t.Context())
		require.NoError(t, err)

		// Verify TTL fields are empty when locking config is nil
		assert.Empty(t, result.AppendTTL)
		assert.Empty(t, result.ClearTTL)
		assert.Empty(t, result.FlushTTL)

		// Verify other fields still work
		assert.Equal(t, config.ID, result.ID)
		assert.Equal(t, config.Type, result.Type)
	})
}

func TestConfigResolverPatternIntegration(t *testing.T) {
	t.Run("Should validate and use regex patterns directly", func(t *testing.T) {
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory with privacy patterns",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns: []string{
					`\b\d{3}-\d{2}-\d{4}\b`,                               // SSN pattern
					`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email pattern
					`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{3,6}\b`,        // Credit card pattern
				},
			},
		}
		builder := &ResourceBuilder{config: config}
		resource, err := builder.Build(t.Context())
		require.NoError(t, err)
		require.NotNil(t, resource)
		require.NotNil(t, resource.PrivacyPolicy)
		// Should have all patterns
		assert.Len(t, resource.PrivacyPolicy.RedactPatterns, 3)
		// Check that patterns are valid regex
		assert.Regexp(t, resource.PrivacyPolicy.RedactPatterns[0], "123-45-6789")         // SSN
		assert.Regexp(t, resource.PrivacyPolicy.RedactPatterns[1], "test@example.com")    // Email
		assert.Regexp(t, resource.PrivacyPolicy.RedactPatterns[2], "4111 1111 1111 1111") // Credit card
	})
	t.Run("Should reject invalid regex patterns", func(t *testing.T) {
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory with invalid pattern",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns: []string{
					`\b\d{3}-\d{2}-\d{4}\b`, // Valid SSN pattern
					`[invalid(`,             // Invalid regex
				},
			},
		}
		builder := &ResourceBuilder{config: config}
		resource, err := builder.Build(t.Context())
		assert.Error(t, err, "Should return error for invalid patterns")
		assert.Nil(t, resource)
	})
	t.Run("Should reject ReDoS vulnerable patterns", func(t *testing.T) {
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory with dangerous pattern",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns: []string{
					`\b\d{3}-\d{2}-\d{4}\b`, // Valid SSN pattern
					`(a+)+`,                 // ReDoS vulnerable pattern
				},
			},
		}
		builder := &ResourceBuilder{config: config}
		resource, err := builder.Build(t.Context())
		assert.Error(t, err, "Should return error for dangerous patterns")
		assert.Nil(t, resource)
	})
	t.Run("Should preserve other privacy policy settings", func(t *testing.T) {
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory with full privacy policy",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns: []string{
					`\b\d{3}-\d{2}-\d{4}\b`,
					`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
				},
				NonPersistableMessageTypes: []string{"system", "tool"},
				DefaultRedactionString:     "[HIDDEN]",
			},
		}
		builder := &ResourceBuilder{config: config}
		resource, err := builder.Build(t.Context())
		require.NoError(t, err)
		require.NotNil(t, resource)
		require.NotNil(t, resource.PrivacyPolicy)
		// Check all settings are preserved
		assert.Equal(t, []string{"system", "tool"}, resource.PrivacyPolicy.NonPersistableMessageTypes)
		assert.Equal(t, "[HIDDEN]", resource.PrivacyPolicy.DefaultRedactionString)
		assert.Len(t, resource.PrivacyPolicy.RedactPatterns, 2)
	})
	t.Run("Should handle empty patterns list", func(t *testing.T) {
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory without patterns",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns:             []string{},
				NonPersistableMessageTypes: []string{"system"},
			},
		}
		builder := &ResourceBuilder{config: config}
		resource, err := builder.Build(t.Context())
		require.NoError(t, err)
		require.NotNil(t, resource)
		require.NotNil(t, resource.PrivacyPolicy)
		assert.Empty(t, resource.PrivacyPolicy.RedactPatterns)
		assert.Equal(t, []string{"system"}, resource.PrivacyPolicy.NonPersistableMessageTypes)
	})
}

// TestManager_validateKey tests the validateKey function
func TestManager_validateKey(t *testing.T) {
	manager := &Manager{}

	t.Run("Should accept valid keys", func(t *testing.T) {
		validKeys := []string{
			"user:123",
			"user_123",
			"user-123",
			"user@example.com",
			"user.name",
			"a", // single character
			"user:123:session:456",
			"user_2024-01-01@example.com",
			"user:123:*",             // asterisk allowed for wildcard patterns
			"user*name",              // asterisk allowed for wildcard patterns
			strings.Repeat("a", 256), // max length
		}

		for _, key := range validKeys {
			validated, err := manager.validateKey(key)
			assert.NoError(t, err, "Key should be valid: %s", key)
			assert.Equal(t, key, validated, "Key should not be modified: %s", key)
		}
	})

	t.Run("Should reject invalid keys", func(t *testing.T) {
		invalidKeys := []struct {
			key         string
			errContains string
		}{
			{"", "invalid memory key"},
			{"user name", "invalid memory key"},  // spaces not allowed
			{"user!name", "invalid memory key"},  // exclamation not allowed
			{"user#name", "invalid memory key"},  // hash not allowed
			{"user$name", "invalid memory key"},  // dollar not allowed
			{"user%name", "invalid memory key"},  // percent not allowed
			{"user&name", "invalid memory key"},  // ampersand not allowed
			{"user(name)", "invalid memory key"}, // parentheses not allowed
			{"user[name]", "invalid memory key"}, // brackets not allowed
			{"user{name}", "invalid memory key"}, // braces not allowed
			{"user<name>", "invalid memory key"}, // angle brackets not allowed
			{"user|name", "invalid memory key"},  // pipe not allowed
			{"user\\name", "invalid memory key"}, // backslash not allowed
			{"user/name", "invalid memory key"},  // forward slash not allowed
			{"__reserved", "cannot start or end with '__'"},
			{"reserved__", "cannot start or end with '__'"},
			{"__reserved__", "cannot start or end with '__'"},
			{strings.Repeat("a", 257), "invalid memory key"}, // too long
		}

		for _, tc := range invalidKeys {
			_, err := manager.validateKey(tc.key)
			assert.Error(t, err, "Key should be invalid: %s", tc.key)
			assert.Contains(
				t,
				err.Error(),
				tc.errContains,
				"Error message should contain expected text for key: %s",
				tc.key,
			)
		}
	})

	t.Run("Should handle edge cases", func(t *testing.T) {
		// Keys with multiple special characters
		validComplexKeys := []string{
			"user:123@example.com:session-456.tmp",
			"org.company.user_123-test@2024",
			"a:b:c:d:e:f:g:h:i:j:k", // many colons
			"a_b_c_d_e_f_g_h_i_j_k", // many underscores
			"a-b-c-d-e-f-g-h-i-j-k", // many hyphens
			"a.b.c.d.e.f.g.h.i.j.k", // many dots
		}

		for _, key := range validComplexKeys {
			validated, err := manager.validateKey(key)
			assert.NoError(t, err, "Complex key should be valid: %s", key)
			assert.Equal(t, key, validated, "Complex key should not be modified: %s", key)
		}
	})
}
