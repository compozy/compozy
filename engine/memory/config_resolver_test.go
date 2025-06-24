package memory

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/autoload"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		result, err := manager.loadMemoryConfig("test-memory")

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
		result, err := manager.loadMemoryConfig("nonexistent-memory")

		// Verify error
		require.Error(t, err)
		assert.Nil(t, result)

		// Check error type and message
		var configErr *memcore.ConfigError
		require.ErrorAs(t, err, &configErr)
		assert.Contains(t, err.Error(), "memory resource 'nonexistent-memory' not found in registry")
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
		result, err := manager.loadMemoryConfig("wrong-type")

		// Verify error
		require.Error(t, err)
		assert.Nil(t, result)

		// Check error type and message
		var configErr *memcore.ConfigError
		require.ErrorAs(t, err, &configErr)
		assert.Contains(t, err.Error(), "invalid config type for memory resource 'wrong-type'")
		assert.Contains(t, err.Error(), "expected *memory.Config")
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
				result, err := manager.loadMemoryConfig(tc.config.ID)

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
			result, err := manager.loadMemoryConfig(testID)
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
		result, err := manager.loadMemoryConfig("complex-memory")
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
		log := logger.FromContext(context.Background())

		// Create manager with template engine
		manager := &Manager{
			tplEngine: engine,
			log:       log,
		}

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
		sanitizedKey, projectID := manager.resolveMemoryKey(context.Background(), template, workflowContext)

		// Verify results
		assert.NotEmpty(t, sanitizedKey)
		assert.Equal(t, "test-project-123", projectID)

		// Verify the resolved template was processed (should be different from template hash)
		templateOnlyHash := manager.sanitizeKey(template)
		assert.NotEqual(t, templateOnlyHash, sanitizedKey, "Template should be resolved before sanitization")
	})

	t.Run("Should resolve complex template with nested variables", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)
		log := logger.FromContext(context.Background())

		// Create manager
		manager := &Manager{
			tplEngine: engine,
			log:       log,
		}

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
		sanitizedKey, projectID := manager.resolveMemoryKey(context.Background(), template, workflowContext)

		// Verify results
		assert.NotEmpty(t, sanitizedKey)
		assert.Equal(t, "complex-project", projectID)
	})

	t.Run("Should handle template without project.id gracefully", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)
		log := logger.FromContext(context.Background())

		// Create manager
		manager := &Manager{
			tplEngine: engine,
			log:       log,
		}

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
		sanitizedKey, projectID := manager.resolveMemoryKey(context.Background(), template, workflowContext)

		// Verify results
		assert.NotEmpty(t, sanitizedKey)
		assert.Empty(t, projectID, "Project ID should be empty when not in context")
	})

	t.Run("Should fallback to sanitization when template evaluation fails", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)
		log := logger.FromContext(context.Background())

		// Create manager
		manager := &Manager{
			tplEngine: engine,
			log:       log,
		}

		// Context with project.id
		workflowContext := map[string]any{
			"project.id": "fallback-project",
		}

		// Invalid template (missing variable)
		invalidTemplate := "memory-{{.nonexistent.variable}}"

		// Call resolveMemoryKey
		sanitizedKey, projectID := manager.resolveMemoryKey(context.Background(), invalidTemplate, workflowContext)

		// Verify results - should fallback to sanitizing the template string
		assert.NotEmpty(t, sanitizedKey)
		assert.Equal(t, "fallback-project", projectID)

		// Verify it's the hash of the original template
		expectedHash := manager.sanitizeKey(invalidTemplate)
		assert.Equal(t, expectedHash, sanitizedKey)
	})

	t.Run("Should handle empty template gracefully", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)
		log := logger.FromContext(context.Background())

		// Create manager
		manager := &Manager{
			tplEngine: engine,
			log:       log,
		}

		// Context with project.id
		workflowContext := map[string]any{
			"project.id": "empty-test-project",
		}

		// Empty template
		emptyTemplate := ""

		// Call resolveMemoryKey
		sanitizedKey, projectID := manager.resolveMemoryKey(context.Background(), emptyTemplate, workflowContext)

		// Verify results
		assert.NotEmpty(t, sanitizedKey) // Should be hash of empty string
		assert.Equal(t, "empty-test-project", projectID)
	})

	t.Run("Should handle template with missing project.id context", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)
		log := logger.FromContext(context.Background())

		// Create manager
		manager := &Manager{
			tplEngine: engine,
			log:       log,
		}

		// Context missing project.id
		workflowContext := map[string]any{
			"agent": map[string]any{
				"name": "test-agent",
			},
		}

		// Template
		template := "memory-{{.agent.name}}"

		// Call resolveMemoryKey
		sanitizedKey, projectID := manager.resolveMemoryKey(context.Background(), template, workflowContext)

		// Verify results
		assert.NotEmpty(t, sanitizedKey)
		assert.Empty(t, projectID)
	})

	t.Run("Should handle literal string without template markers", func(t *testing.T) {
		// Create template engine
		engine := tplengine.NewEngine(tplengine.FormatText)
		log := logger.FromContext(context.Background())

		// Create manager
		manager := &Manager{
			tplEngine: engine,
			log:       log,
		}

		// Context with project.id
		workflowContext := map[string]any{
			"project.id": "literal-project",
		}

		// Literal string (no template markers)
		literalString := "memory-literal-key"

		// Call resolveMemoryKey
		sanitizedKey, projectID := manager.resolveMemoryKey(context.Background(), literalString, workflowContext)

		// Verify results
		assert.NotEmpty(t, sanitizedKey)
		assert.Equal(t, "literal-project", projectID)

		// Should be hash of the literal string
		expectedHash := manager.sanitizeKey(literalString)
		assert.Equal(t, expectedHash, sanitizedKey)
	})
}

func TestExtractProjectID(t *testing.T) {
	t.Run("Should extract project ID from valid context", func(t *testing.T) {
		workflowContext := map[string]any{
			"project.id": "test-project-id",
		}

		projectID := extractProjectID(workflowContext)
		assert.Equal(t, "test-project-id", projectID)
	})

	t.Run("Should return empty string when project.id is missing", func(t *testing.T) {
		workflowContext := map[string]any{
			"project.name": "Test Project",
		}

		projectID := extractProjectID(workflowContext)
		assert.Empty(t, projectID)
	})

	t.Run("Should return empty string when project key is missing", func(t *testing.T) {
		workflowContext := map[string]any{
			"agent": map[string]any{
				"name": "test-agent",
			},
		}

		projectID := extractProjectID(workflowContext)
		assert.Empty(t, projectID)
	})

	t.Run("Should return empty string when project.id is not a string", func(t *testing.T) {
		workflowContext := map[string]any{
			"project.id": 123, // not a string
		}

		projectID := extractProjectID(workflowContext)
		assert.Empty(t, projectID)
	})

	t.Run("Should handle empty context", func(t *testing.T) {
		workflowContext := map[string]any{}

		projectID := extractProjectID(workflowContext)
		assert.Empty(t, projectID)
	})

	t.Run("Should handle nil context", func(t *testing.T) {
		projectID := extractProjectID(nil)
		assert.Empty(t, projectID)
	})
}

func TestManager_configToResource(t *testing.T) {
	t.Run("Should properly map config to resource with TTL fields", func(t *testing.T) {
		manager := &Manager{}

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

		result := manager.configToResource(config)

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
		assert.Empty(
			t,
			result.EvictionPolicy,
			"EvictionPolicy should be empty - determined by memory type and flushing strategy",
		)
		assert.Empty(t, result.TokenCounter, "TokenCounter should be empty - determined at runtime")
		assert.Nil(t, result.Metadata, "Metadata should be nil - not stored in config")
		assert.False(t, result.DisableFlush, "DisableFlush should be false - flush enabled by default")
	})

	t.Run("Should handle nil locking config gracefully", func(t *testing.T) {
		manager := &Manager{}

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

		result := manager.configToResource(config)

		// Verify TTL fields are empty when locking config is nil
		assert.Empty(t, result.AppendTTL)
		assert.Empty(t, result.ClearTTL)
		assert.Empty(t, result.FlushTTL)

		// Verify other fields still work
		assert.Equal(t, config.ID, result.ID)
		assert.Equal(t, config.Type, result.Type)
	})
}
