package autoload

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
)

// Mock configurations for testing without import cycles
type mockWorkflowConfig struct {
	Resource string
	ID       string
}

type mockAgentConfig struct {
	Resource string
	ID       string
}

func TestConfigRegistrySimple_Register(t *testing.T) {
	t.Run("Should register config with Resource field", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := &mockWorkflowConfig{
			Resource: "Workflow",
			ID:       "test-workflow",
		}
		err := registry.Register(config, "autoload")
		assert.NoError(t, err)
		assert.Equal(t, 1, registry.Count())
	})
	t.Run("Should detect duplicate configurations", func(t *testing.T) {
		registry := NewConfigRegistry()
		config1 := &mockWorkflowConfig{
			Resource: "Workflow",
			ID:       "duplicate-id",
		}
		config2 := &mockWorkflowConfig{
			Resource: "Workflow",
			ID:       "duplicate-id",
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
	t.Run("Should allow same ID across different resource types", func(t *testing.T) {
		registry := NewConfigRegistry()
		config1 := &mockWorkflowConfig{
			Resource: "Workflow",
			ID:       "same-id",
		}
		config2 := &mockAgentConfig{
			Resource: "Agent",
			ID:       "same-id",
		}
		err := registry.Register(config1, "autoload")
		assert.NoError(t, err)
		err = registry.Register(config2, "autoload")
		assert.NoError(t, err)
		assert.Equal(t, 2, registry.Count())
	})
	t.Run("Should handle empty ID", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := &mockWorkflowConfig{
			Resource: "Workflow",
			ID:       "",
		}
		err := registry.Register(config, "autoload")
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "EMPTY_ID", coreErr.Code)
	})
	t.Run("Should normalize resource type case", func(t *testing.T) {
		registry := NewConfigRegistry()
		config1 := &mockWorkflowConfig{
			Resource: "Workflow",
			ID:       "test-id",
		}
		config2 := &mockWorkflowConfig{
			Resource: "WORKFLOW",
			ID:       "test-id",
		}
		err := registry.Register(config1, "manual")
		assert.NoError(t, err)
		err = registry.Register(config2, "autoload")
		require.Error(t, err, "Should detect duplicate with different case")
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "DUPLICATE_CONFIG", coreErr.Code)
		// Verify retrieval works with any case
		_, err = registry.Get("workflow", "test-id")
		assert.NoError(t, err)
		_, err = registry.Get("WORKFLOW", "test-id")
		assert.NoError(t, err)
		_, err = registry.Get("Workflow", "test-id")
		assert.NoError(t, err)
	})
	t.Run("Should trim whitespace from resource type and ID", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := &mockWorkflowConfig{
			Resource: "  Workflow  ",
			ID:       "  test-id  ",
		}
		err := registry.Register(config, "autoload")
		assert.NoError(t, err)
		// Should be able to retrieve without whitespace
		_, err = registry.Get("Workflow", "test-id")
		assert.NoError(t, err)
	})
}

func TestConfigRegistrySimple_Get(t *testing.T) {
	t.Run("Should retrieve registered configuration", func(t *testing.T) {
		registry := NewConfigRegistry()
		config := &mockWorkflowConfig{
			Resource: "Workflow",
			ID:       "test-workflow",
		}
		err := registry.Register(config, "autoload")
		require.NoError(t, err)
		retrieved, err := registry.Get("Workflow", "test-workflow")
		assert.NoError(t, err)
		assert.Equal(t, config, retrieved)
	})
	t.Run("Should return error for non-existent configuration", func(t *testing.T) {
		registry := NewConfigRegistry()
		_, err := registry.Get("Workflow", "non-existent")
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "RESOURCE_NOT_FOUND", coreErr.Code)
		assert.Equal(t, "workflow", coreErr.Details["type"]) // normalized to lowercase
		assert.Equal(t, "non-existent", coreErr.Details["id"])
	})
}

func TestConfigRegistrySimple_GetAll(t *testing.T) {
	t.Run("Should retrieve all configurations of a type", func(t *testing.T) {
		registry := NewConfigRegistry()
		config1 := &mockWorkflowConfig{Resource: "Workflow", ID: "workflow-1"}
		config2 := &mockWorkflowConfig{Resource: "Workflow", ID: "workflow-2"}
		config3 := &mockAgentConfig{Resource: "Agent", ID: "agent-1"}
		err := registry.Register(config1, "autoload")
		require.NoError(t, err)
		err = registry.Register(config2, "autoload")
		require.NoError(t, err)
		err = registry.Register(config3, "autoload")
		require.NoError(t, err)
		workflows := registry.GetAll("Workflow")
		assert.Len(t, workflows, 2)
		agents := registry.GetAll("Agent")
		assert.Len(t, agents, 1)
	})
	t.Run("Should return empty slice for non-existent type", func(t *testing.T) {
		registry := NewConfigRegistry()
		configs := registry.GetAll("NonExistentType")
		assert.NotNil(t, configs)
		assert.Empty(t, configs)
	})
}

func TestConfigRegistrySimple_Clear(t *testing.T) {
	t.Run("Should clear all configurations", func(t *testing.T) {
		registry := NewConfigRegistry()
		config1 := &mockWorkflowConfig{Resource: "Workflow", ID: "workflow-1"}
		config2 := &mockAgentConfig{Resource: "Agent", ID: "agent-1"}
		err := registry.Register(config1, "autoload")
		require.NoError(t, err)
		err = registry.Register(config2, "autoload")
		require.NoError(t, err)
		assert.Equal(t, 2, registry.Count())
		registry.Clear()
		assert.Equal(t, 0, registry.Count())
		// Verify configs are actually gone
		_, err = registry.Get("Workflow", "workflow-1")
		require.Error(t, err)
		_, err = registry.Get("Agent", "agent-1")
		require.Error(t, err)
	})
}

func TestConfigRegistrySimple_Concurrency(t *testing.T) {
	t.Run("Should handle concurrent registrations safely", func(t *testing.T) {
		registry := NewConfigRegistry()
		var wg sync.WaitGroup
		errCh := make(chan error, 10)
		// Spawn 10 goroutines to register configs concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				config := &mockWorkflowConfig{
					Resource: "Workflow",
					ID:       fmt.Sprintf("workflow-%d", id),
				}
				errCh <- registry.Register(config, "autoload")
			}(i)
		}
		wg.Wait()
		close(errCh)
		// Check all registrations succeeded
		for err := range errCh {
			assert.NoError(t, err)
		}
		assert.Equal(t, 10, registry.Count())
	})
}

func TestConfigRegistrySimple_CountByType(t *testing.T) {
	t.Run("Should count configurations by type", func(t *testing.T) {
		registry := NewConfigRegistry()
		// Register different types of configurations
		workflow1 := &mockWorkflowConfig{Resource: "Workflow", ID: "workflow-1"}
		workflow2 := &mockWorkflowConfig{Resource: "Workflow", ID: "workflow-2"}
		agent := &mockAgentConfig{Resource: "Agent", ID: "agent-1"}
		err := registry.Register(workflow1, "test")
		require.NoError(t, err)
		err = registry.Register(workflow2, "test")
		require.NoError(t, err)
		err = registry.Register(agent, "test")
		require.NoError(t, err)
		// Test counts
		assert.Equal(t, 2, registry.CountByType("workflow"))
		assert.Equal(t, 1, registry.CountByType("agent"))
		assert.Equal(t, 0, registry.CountByType("tool"))
		assert.Equal(t, 0, registry.CountByType("nonexistent"))
	})

	t.Run("Should handle case-insensitive type names", func(t *testing.T) {
		registry := NewConfigRegistry()
		workflow := &mockWorkflowConfig{Resource: "WORKFLOW", ID: "test-workflow"}
		err := registry.Register(workflow, "test")
		require.NoError(t, err)
		assert.Equal(t, 1, registry.CountByType("workflow"))
		assert.Equal(t, 1, registry.CountByType("WORKFLOW"))
		assert.Equal(t, 1, registry.CountByType("Workflow"))
	})

	t.Run("Should return zero for empty registry", func(t *testing.T) {
		registry := NewConfigRegistry()
		assert.Equal(t, 0, registry.CountByType("workflow"))
		assert.Equal(t, 0, registry.CountByType("agent"))
	})
}

func TestExtractResourceInfoSimple(t *testing.T) {
	t.Run("Should extract info from config with Resource field", func(t *testing.T) {
		config := &mockWorkflowConfig{
			Resource: "CustomWorkflow",
			ID:       "test-workflow",
		}
		resourceType, id, err := extractResourceInfo(config)
		assert.NoError(t, err)
		assert.Equal(t, "CustomWorkflow", resourceType)
		assert.Equal(t, "test-workflow", id)
	})
	t.Run("Should handle unknown config type", func(t *testing.T) {
		unknownConfig := struct{ SomeField string }{SomeField: "value"}
		_, _, err := extractResourceInfo(&unknownConfig)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "UNKNOWN_CONFIG_TYPE", coreErr.Code)
	})
	t.Run("Should handle non-struct config", func(t *testing.T) {
		stringConfig := "not a struct"
		_, _, err := extractResourceInfo(stringConfig)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_CONFIG_TYPE", coreErr.Code)
	})
}
