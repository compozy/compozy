package autoload

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceResolver_ResolveResource(t *testing.T) {
	t.Run("Should resolve resource by ID selector", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := map[string]any{
			"resource": "workflow",
			"id":       "test-workflow",
			"name":     "Test Workflow",
			"steps":    []string{"step1", "step2"},
		}
		err := registry.Register(config, "autoload")
		require.NoError(t, err)
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("workflow", "#(id=='test-workflow')")
		require.NoError(t, err)
		assert.Equal(t, config, result)
	})

	t.Run("Should resolve resource field by ID selector", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := map[string]any{
			"resource": "task",
			"id":       "email-task",
			"name":     "Email Task",
			"outputs":  map[string]any{"email": "sent", "status": "complete"},
		}
		err := registry.Register(config, "autoload")
		require.NoError(t, err)
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("task", "#(id=='email-task').outputs")
		require.NoError(t, err)
		expected := map[string]any{"email": "sent", "status": "complete"}
		assert.Equal(t, expected, result)
	})

	t.Run("Should resolve nested field by ID selector", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := map[string]any{
			"resource": "tool",
			"id":       "api-tool",
			"config": map[string]any{
				"endpoint": "https://api.example.com",
				"auth":     map[string]any{"type": "bearer", "token": "secret"},
			},
		}
		err := registry.Register(config, "autoload")
		require.NoError(t, err)
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("tool", "#(id=='api-tool').config.auth.type")
		require.NoError(t, err)
		assert.Equal(t, "bearer", result)
	})

	t.Run("Should resolve resource by simple ID", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := map[string]any{
			"resource": "agent",
			"id":       "worker-agent",
			"type":     "background",
		}
		err := registry.Register(config, "autoload")
		require.NoError(t, err)
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("agent", "worker-agent")
		require.NoError(t, err)
		assert.Equal(t, config, result)
	})

	t.Run("Should resolve field with simple ID", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := map[string]any{
			"resource": "agent",
			"id":       "worker-agent",
			"type":     "background",
			"replicas": 3,
		}
		err := registry.Register(config, "autoload")
		require.NoError(t, err)
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("agent", "worker-agent.type")
		require.NoError(t, err)
		assert.Equal(t, "background", result)
	})

	t.Run("Should handle case-insensitive resource types and IDs", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := map[string]any{
			"resource": "Workflow",
			"id":       "Test-Workflow",
			"name":     "Test",
		}
		err := registry.Register(config, "autoload")
		require.NoError(t, err)
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("workflow", "#(id=='test-workflow')")
		require.NoError(t, err)
		assert.Equal(t, config, result)
	})

	t.Run("Should return error for non-existent resource", func(t *testing.T) {
		registry := NewConfigRegistry()
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("workflow", "#(id=='missing')")
		assert.Error(t, err)
		assert.Nil(t, result)
		// Check if it's a core.Error with the expected code
		if coreErr, ok := err.(*core.Error); ok {
			assert.Equal(t, "RESOURCE_NOT_FOUND", coreErr.Code)
		} else {
			t.Fatalf("Expected core.Error, got %T", err)
		}
	})

	t.Run("Should return error for non-existent field", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := map[string]any{
			"resource": "task",
			"id":       "test-task",
			"name":     "Test Task",
		}
		err := registry.Register(config, "autoload")
		require.NoError(t, err)
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("task", "#(id=='test-task').missing_field")
		assert.Error(t, err)
		assert.Nil(t, result)
		// Check if it's a core.Error with the expected code
		if coreErr, ok := err.(*core.Error); ok {
			assert.Equal(t, "FIELD_NOT_FOUND", coreErr.Code)
		} else {
			t.Fatalf("Expected core.Error, got %T", err)
		}
	})

	t.Run("Should return error for invalid selector format", func(t *testing.T) {
		registry := NewConfigRegistry()
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("workflow", "#(invalid)")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid resource selector")
	})

	t.Run("Should return error for empty selector", func(t *testing.T) {
		registry := NewConfigRegistry()
		resolver := NewResourceResolver(registry)
		result, err := resolver.ResolveResource("workflow", "")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "selector cannot be empty")
	})
}

func TestParseResourceSelector(t *testing.T) {
	t.Run("Should parse ID selector with field", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector("#(id=='test-workflow').name")
		require.NoError(t, err)
		assert.Equal(t, "test-workflow", id)
		assert.Equal(t, "name", fieldPath)
	})

	t.Run("Should parse ID selector without field", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector("#(id=='test-workflow')")
		require.NoError(t, err)
		assert.Equal(t, "test-workflow", id)
		assert.Equal(t, "", fieldPath)
	})

	t.Run("Should parse simple ID with field", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector("test-workflow.name")
		require.NoError(t, err)
		assert.Equal(t, "test-workflow", id)
		assert.Equal(t, "name", fieldPath)
	})

	t.Run("Should parse simple ID without field", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector("test-workflow")
		require.NoError(t, err)
		assert.Equal(t, "test-workflow", id)
		assert.Equal(t, "", fieldPath)
	})

	t.Run("Should parse ID selector with nested field", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector("#(id=='api-tool').config.endpoint")
		require.NoError(t, err)
		assert.Equal(t, "api-tool", id)
		assert.Equal(t, "config.endpoint", fieldPath)
	})

	t.Run("Should handle single quotes in selector", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector("#(id=='my-task').output")
		require.NoError(t, err)
		assert.Equal(t, "my-task", id)
		assert.Equal(t, "output", fieldPath)
	})

	t.Run("Should handle double quotes in selector", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector(`#(id=="my-task").output`)
		require.NoError(t, err)
		assert.Equal(t, "my-task", id)
		assert.Equal(t, "output", fieldPath)
	})

	t.Run("Should return error for empty selector", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector("")
		assert.Error(t, err)
		assert.Equal(t, "", id)
		assert.Equal(t, "", fieldPath)
		assert.Contains(t, err.Error(), "selector cannot be empty")
	})

	t.Run("Should return error for invalid selector format", func(t *testing.T) {
		id, fieldPath, err := ParseResourceSelector("#(invalid==test)")
		assert.Error(t, err)
		assert.Equal(t, "", id)
		assert.Equal(t, "", fieldPath)
	})
}

func TestApplyFieldPath(t *testing.T) {
	t.Run("Should apply simple field path", func(t *testing.T) {
		config := map[string]any{
			"name": "Test Config",
			"type": "workflow",
		}
		result, err := ApplyFieldPath(config, "name")
		require.NoError(t, err)
		assert.Equal(t, "Test Config", result)
	})

	t.Run("Should apply nested field path", func(t *testing.T) {
		config := map[string]any{
			"config": map[string]any{
				"auth": map[string]any{
					"type":  "bearer",
					"token": "secret",
				},
			},
		}
		result, err := ApplyFieldPath(config, "config.auth.type")
		require.NoError(t, err)
		assert.Equal(t, "bearer", result)
	})

	t.Run("Should apply array index path", func(t *testing.T) {
		config := map[string]any{
			"steps": []any{"step1", "step2", "step3"},
		}
		result, err := ApplyFieldPath(config, "steps.1")
		require.NoError(t, err)
		assert.Equal(t, "step2", result)
	})

	t.Run("Should return error for non-existent field", func(t *testing.T) {
		config := map[string]any{
			"name": "Test Config",
		}
		result, err := ApplyFieldPath(config, "missing")
		assert.Error(t, err)
		assert.Nil(t, result)
		// Check if it's a core.Error with the expected code
		if coreErr, ok := err.(*core.Error); ok {
			assert.Equal(t, "FIELD_NOT_FOUND", coreErr.Code)
		} else {
			t.Fatalf("Expected core.Error, got %T", err)
		}
	})

	t.Run("Should handle complex structures", func(t *testing.T) {
		config := map[string]any{
			"workflows": []any{
				map[string]any{"id": "wf1", "name": "Workflow 1"},
				map[string]any{"id": "wf2", "name": "Workflow 2"},
			},
		}
		result, err := ApplyFieldPath(config, "workflows.0.name")
		require.NoError(t, err)
		assert.Equal(t, "Workflow 1", result)
	})

	t.Run("Should handle nil config", func(t *testing.T) {
		result, err := ApplyFieldPath(nil, "field")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
