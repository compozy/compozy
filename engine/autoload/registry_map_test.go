package autoload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
)

func TestConfigRegistry_MapConfigurations(t *testing.T) {
	t.Run("Should register map configuration successfully", func(t *testing.T) {
		registry := NewConfigRegistry()
		configMap := map[string]any{
			"resource": "workflow",
			"id":       "test-workflow",
			"version":  "1.0",
			"tasks":    []any{},
		}
		err := registry.Register(configMap, "autoload")
		assert.NoError(t, err)
		assert.Equal(t, 1, registry.Count())
	})

	t.Run("Should retrieve map configuration", func(t *testing.T) {
		registry := NewConfigRegistry()
		configMap := map[string]any{
			"resource":    "workflow",
			"id":          "test-workflow",
			"version":     "1.0",
			"description": "Test workflow",
		}
		err := registry.Register(configMap, "autoload")
		require.NoError(t, err)
		retrieved, err := registry.Get("workflow", "test-workflow")
		assert.NoError(t, err)
		assert.Equal(t, configMap, retrieved)
	})

	t.Run("Should handle case-insensitive resource type for maps", func(t *testing.T) {
		registry := NewConfigRegistry()
		configMap := map[string]any{
			"resource": "WORKFLOW",
			"id":       "test-workflow",
		}
		err := registry.Register(configMap, "autoload")
		require.NoError(t, err)
		// Should be able to retrieve with different cases
		_, err = registry.Get("workflow", "test-workflow")
		assert.NoError(t, err)
		_, err = registry.Get("Workflow", "test-workflow")
		assert.NoError(t, err)
		_, err = registry.Get("WORKFLOW", "test-workflow")
		assert.NoError(t, err)
	})

	t.Run("Should detect duplicate map configurations", func(t *testing.T) {
		registry := NewConfigRegistry()
		config1 := map[string]any{
			"resource": "workflow",
			"id":       "duplicate-id",
		}
		config2 := map[string]any{
			"resource": "workflow",
			"id":       "duplicate-id",
		}
		err := registry.Register(config1, "manual")
		assert.NoError(t, err)
		err = registry.Register(config2, "autoload")
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "DUPLICATE_CONFIG", coreErr.Code)
		assert.Equal(t, "manual", coreErr.Details["existing_source"])
		assert.Equal(t, "autoload", coreErr.Details["source"])
	})

	t.Run("Should allow same ID across different resource types for maps", func(t *testing.T) {
		registry := NewConfigRegistry()
		workflowConfig := map[string]any{
			"resource": "workflow",
			"id":       "same-id",
		}
		agentConfig := map[string]any{
			"resource": "agent",
			"id":       "same-id",
		}
		err := registry.Register(workflowConfig, "autoload")
		assert.NoError(t, err)
		err = registry.Register(agentConfig, "autoload")
		assert.NoError(t, err)
		assert.Equal(t, 2, registry.Count())
	})

	t.Run("Should handle whitespace in map resource type and ID", func(t *testing.T) {
		registry := NewConfigRegistry()
		configMap := map[string]any{
			"resource": "  workflow  ",
			"id":       "  test-id  ",
		}
		err := registry.Register(configMap, "autoload")
		assert.NoError(t, err)
		// Should be able to retrieve without whitespace
		_, err = registry.Get("workflow", "test-id")
		assert.NoError(t, err)
	})

	t.Run("Should handle case-insensitive IDs", func(t *testing.T) {
		registry := NewConfigRegistry()
		configMap := map[string]any{
			"resource": "workflow",
			"id":       "Test-ID",
		}
		err := registry.Register(configMap, "autoload")
		assert.NoError(t, err)
		// Should be able to retrieve with different case
		_, err = registry.Get("workflow", "test-id")
		assert.NoError(t, err)
		_, err = registry.Get("workflow", "TEST-ID")
		assert.NoError(t, err)
	})
}

func TestExtractResourceInfoFromMap(t *testing.T) {
	t.Run("Should extract valid resource info from map", func(t *testing.T) {
		configMap := map[string]any{
			"resource":    "workflow",
			"id":          "test-workflow",
			"version":     "1.0",
			"description": "Test description",
		}
		resourceType, id, err := extractResourceInfoFromMap(configMap)
		assert.NoError(t, err)
		assert.Equal(t, "workflow", resourceType)
		assert.Equal(t, "test-workflow", id)
	})

	t.Run("Should handle missing resource field", func(t *testing.T) {
		configMap := map[string]any{
			"id":      "test-workflow",
			"version": "1.0",
		}
		_, _, err := extractResourceInfoFromMap(configMap)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "MISSING_RESOURCE_FIELD", coreErr.Code)
	})

	t.Run("Should handle invalid resource field type", func(t *testing.T) {
		configMap := map[string]any{
			"resource": 123,
			"id":       "test-workflow",
		}
		_, _, err := extractResourceInfoFromMap(configMap)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_RESOURCE_FIELD", coreErr.Code)
		assert.Equal(t, 123, coreErr.Details["resource"])
	})

	t.Run("Should handle empty resource field", func(t *testing.T) {
		configMap := map[string]any{
			"resource": "",
			"id":       "test-workflow",
		}
		_, _, err := extractResourceInfoFromMap(configMap)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_RESOURCE_FIELD", coreErr.Code)
	})

	t.Run("Should handle missing ID field", func(t *testing.T) {
		configMap := map[string]any{
			"resource": "workflow",
			"version":  "1.0",
		}
		_, _, err := extractResourceInfoFromMap(configMap)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "MISSING_ID_FIELD", coreErr.Code)
	})

	t.Run("Should handle invalid ID field type", func(t *testing.T) {
		configMap := map[string]any{
			"resource": "workflow",
			"id":       123,
		}
		_, _, err := extractResourceInfoFromMap(configMap)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_ID_FIELD", coreErr.Code)
		assert.Equal(t, 123, coreErr.Details["id"])
	})

	t.Run("Should handle empty ID field", func(t *testing.T) {
		configMap := map[string]any{
			"resource": "workflow",
			"id":       "",
		}
		_, _, err := extractResourceInfoFromMap(configMap)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_ID_FIELD", coreErr.Code)
	})

	t.Run("Should handle mixed valid fields", func(t *testing.T) {
		configMap := map[string]any{
			"resource":    "Agent",
			"id":          "chatbot-agent",
			"description": "AI chatbot agent",
			"model":       "gpt-4",
			"config": map[string]any{
				"temperature": 0.7,
			},
		}
		resourceType, id, err := extractResourceInfoFromMap(configMap)
		assert.NoError(t, err)
		assert.Equal(t, "Agent", resourceType)
		assert.Equal(t, "chatbot-agent", id)
	})
}

func TestExtractResourceInfo_MixedTypes(t *testing.T) {
	t.Run("Should handle both map and struct configurations", func(t *testing.T) {
		registry := NewConfigRegistry()
		// Test with map configuration
		mapConfig := map[string]any{
			"resource": "workflow",
			"id":       "map-workflow",
		}
		err := registry.Register(mapConfig, "autoload")
		assert.NoError(t, err)
		// Test with struct configuration (mock)
		structConfig := &mockWorkflowConfig{
			Resource: "workflow",
			ID:       "struct-workflow",
		}
		err = registry.Register(structConfig, "manual")
		assert.NoError(t, err)
		assert.Equal(t, 2, registry.Count())
		// Verify both can be retrieved
		mapRetrieved, err := registry.Get("workflow", "map-workflow")
		assert.NoError(t, err)
		assert.Equal(t, mapConfig, mapRetrieved)
		structRetrieved, err := registry.Get("workflow", "struct-workflow")
		assert.NoError(t, err)
		assert.Equal(t, structConfig, structRetrieved)
	})
}
