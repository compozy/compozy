package services

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a task config with proper initialization
func createTaskConfig(id string, taskType task.Type) task.Config {
	config := task.Config{}
	config.ID = id
	config.Type = taskType
	return config
}

func TestConfigManager_PrepareParallelConfigs(t *testing.T) {
	t.Run("Should store parallel metadata successfully", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)

		parentStateID := core.MustNewID()
		child1 := createTaskConfig("child1", task.TaskTypeBasic)
		child2 := createTaskConfig("child2", task.TaskTypeBasic)

		taskConfig := &task.Config{}
		taskConfig.Type = task.TaskTypeParallel
		taskConfig.ParallelTask = task.ParallelTask{
			Tasks:      []task.Config{child1, child2},
			Strategy:   task.StrategyWaitAll,
			MaxWorkers: 2,
		}

		// Act
		err := cm.PrepareParallelConfigs(context.Background(), parentStateID, taskConfig)

		// Assert
		require.NoError(t, err)

		// Verify metadata was stored
		key := cm.buildParallelMetadataKey(parentStateID)
		storedMetadata, err := configStore.GetMetadata(context.Background(), key)
		require.NoError(t, err)

		// Verify metadata content
		var metadata ParallelTaskMetadata
		err = json.Unmarshal(storedMetadata, &metadata)
		require.NoError(t, err)
		assert.Equal(t, parentStateID, metadata.ParentStateID)
		assert.Len(t, metadata.ChildConfigs, 2)
	})

	t.Run("Should fail with empty parent state ID", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		taskConfig := &task.Config{}
		taskConfig.Type = task.TaskTypeParallel

		// Act
		err := cm.PrepareParallelConfigs(context.Background(), "", taskConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should fail with nil task config", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()

		// Act
		err := cm.PrepareParallelConfigs(context.Background(), parentStateID, nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config cannot be nil")
	})

	t.Run("Should fail with wrong task type", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()
		taskConfig := &task.Config{}
		taskConfig.Type = task.TaskTypeBasic

		// Act
		err := cm.PrepareParallelConfigs(context.Background(), parentStateID, taskConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config must be parallel type")
	})

	t.Run("Should fail with no child tasks", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Tasks: []task.Config{}, // Empty tasks
			},
		}

		// Act
		err := cm.PrepareParallelConfigs(context.Background(), parentStateID, taskConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel task must have at least one child task")
	})

	t.Run("Should fail with child config missing ID", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Tasks: []task.Config{
					{BaseConfig: task.BaseConfig{ID: "child1", Type: task.TaskTypeBasic}},
					{BaseConfig: task.BaseConfig{ID: "", Type: task.TaskTypeBasic}}, // Missing ID
				},
			},
		}

		// Act
		err := cm.PrepareParallelConfigs(context.Background(), parentStateID, taskConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child config at index 1 missing required ID field")
	})
}

func TestConfigManager_PrepareCollectionConfigs(t *testing.T) {
	t.Run("Should store collection metadata successfully", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)

		parentStateID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: `["item1", "item2", "item3"]`,
				Mode:  task.CollectionModeParallel,
			},
			ParallelTask: task.ParallelTask{
				Task: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "template-task",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}

		// Act
		metadata, err := cm.PrepareCollectionConfigs(context.Background(), parentStateID, taskConfig, workflowState)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, metadata)
		assert.Equal(t, 3, metadata.ItemCount)
		assert.Equal(t, 0, metadata.SkippedCount)
		assert.Equal(t, "parallel", metadata.Mode)

		// Verify metadata was stored
		key := cm.buildCollectionMetadataKey(parentStateID)
		storedMetadata, err := configStore.GetMetadata(context.Background(), key)
		require.NoError(t, err)

		// Verify metadata content
		var storedCollectionMetadata CollectionTaskMetadata
		err = json.Unmarshal(storedMetadata, &storedCollectionMetadata)
		require.NoError(t, err)
		assert.Equal(t, parentStateID, storedCollectionMetadata.ParentStateID)
		assert.Equal(t, 3, storedCollectionMetadata.ItemCount)
	})

	t.Run("Should fail with empty parent state ID", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		workflowState := &workflow.State{}
		taskConfig := &task.Config{}
		taskConfig.Type = task.TaskTypeCollection

		// Act
		_, err := cm.PrepareCollectionConfigs(context.Background(), "", taskConfig, workflowState)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should fail with nil task config", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()
		workflowState := &workflow.State{}

		// Act
		_, err := cm.PrepareCollectionConfigs(context.Background(), parentStateID, nil, workflowState)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config cannot be nil")
	})

	t.Run("Should fail with wrong task type", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()
		workflowState := &workflow.State{}
		taskConfig := &task.Config{}
		taskConfig.Type = task.TaskTypeBasic

		// Act
		_, err := cm.PrepareCollectionConfigs(context.Background(), parentStateID, taskConfig, workflowState)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config must be collection type")
	})

	t.Run("Should fail with nil workflow state", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()
		taskConfig := &task.Config{}
		taskConfig.Type = task.TaskTypeCollection

		// Act
		_, err := cm.PrepareCollectionConfigs(context.Background(), parentStateID, taskConfig, nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state cannot be nil")
	})
}

func TestConfigManager_LoadParallelTaskMetadata(t *testing.T) {
	t.Run("Should load parallel metadata successfully", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)

		parentStateID := core.MustNewID()
		originalTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Tasks: []task.Config{
					{BaseConfig: task.BaseConfig{ID: "child1", Type: task.TaskTypeBasic}},
					{BaseConfig: task.BaseConfig{ID: "child2", Type: task.TaskTypeBasic}},
				},
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 2,
			},
		}

		// Store the metadata first
		err := cm.PrepareParallelConfigs(context.Background(), parentStateID, originalTaskConfig)
		require.NoError(t, err)

		// Act
		metadata, err := cm.LoadParallelTaskMetadata(context.Background(), parentStateID)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, parentStateID, metadata.ParentStateID)
		assert.Len(t, metadata.ChildConfigs, 2)
		assert.Equal(t, "child1", metadata.ChildConfigs[0].ID)
		assert.Equal(t, "child2", metadata.ChildConfigs[1].ID)
		assert.Equal(t, string(task.StrategyWaitAll), metadata.Strategy)
		assert.Equal(t, 2, metadata.MaxWorkers)
	})

	t.Run("Should fail when metadata not found", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()

		// Act
		_, err := cm.LoadParallelTaskMetadata(context.Background(), parentStateID)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get parallel task metadata")
	})
}

func TestConfigManager_LoadCollectionTaskMetadata(t *testing.T) {
	t.Run("Should load collection metadata successfully", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)

		parentStateID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		originalTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: `["item1", "item2"]`,
				Mode:  task.CollectionModeSequential,
			},
			ParallelTask: task.ParallelTask{
				Task: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "template-task",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}

		// Store the metadata first
		_, err := cm.PrepareCollectionConfigs(context.Background(), parentStateID, originalTaskConfig, workflowState)
		require.NoError(t, err)

		// Act
		metadata, err := cm.LoadCollectionTaskMetadata(context.Background(), parentStateID)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, parentStateID, metadata.ParentStateID)
		assert.Len(t, metadata.ChildConfigs, 2)
		assert.Equal(t, 2, metadata.ItemCount)
		assert.Equal(t, 0, metadata.SkippedCount)
		assert.Equal(t, "sequential", metadata.Mode)
		assert.Equal(t, 1, metadata.MaxWorkers) // Sequential mode should set MaxWorkers to 1
	})

	t.Run("Should fail when metadata not found", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)
		parentStateID := core.MustNewID()

		// Act
		_, err := cm.LoadCollectionTaskMetadata(context.Background(), parentStateID)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get collection task metadata")
	})
}

func TestConfigManager_EdgeCases(t *testing.T) {
	t.Run("Should handle collection with filter that removes all items", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)

		parentStateID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items:  `[]`, // Empty items array as JSON string
				Mode:   task.CollectionModeParallel,
				Filter: "false", // Filter that removes everything
			},
			ParallelTask: task.ParallelTask{
				Task: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "template-task",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}

		// Act
		_, err := cm.PrepareCollectionConfigs(context.Background(), parentStateID, taskConfig, workflowState)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no child configs generated")
	})

	t.Run("Should handle parallel task with batch size in collection mode", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		cm := NewConfigManager(configStore)

		parentStateID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: `["item1", "item2", "item3"]`,
				Mode:  task.CollectionModeSequential,
				Batch: 2,
			},
			ParallelTask: task.ParallelTask{
				Task: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "template-task",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}

		// Act
		metadata, err := cm.PrepareCollectionConfigs(context.Background(), parentStateID, taskConfig, workflowState)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, metadata.BatchSize)

		// Load the stored metadata to verify MaxWorkers
		storedMetadata, err := cm.LoadCollectionTaskMetadata(context.Background(), parentStateID)
		require.NoError(t, err)
		assert.Equal(t, 2, storedMetadata.MaxWorkers) // Should use batch size as MaxWorkers
	})
}

// NewMockConfigStore creates a simple in-memory mock config store for testing
func NewMockConfigStore() *MockConfigStore {
	return &MockConfigStore{
		store: make(map[string]*task.Config),
	}
}

type MockConfigStore struct {
	store    map[string]*task.Config
	metadata map[string][]byte
}

func (m *MockConfigStore) Save(_ context.Context, taskExecID string, config *task.Config) error {
	m.store[taskExecID] = config
	return nil
}

func (m *MockConfigStore) Get(_ context.Context, taskExecID string) (*task.Config, error) {
	config, exists := m.store[taskExecID]
	if !exists {
		return nil, fmt.Errorf("configuration not found for task ID: %s", taskExecID)
	}
	return config, nil
}

func (m *MockConfigStore) Delete(_ context.Context, taskExecID string) error {
	delete(m.store, taskExecID)
	return nil
}

func (m *MockConfigStore) SaveMetadata(_ context.Context, key string, data []byte) error {
	if m.metadata == nil {
		m.metadata = make(map[string][]byte)
	}
	m.metadata[key] = data
	return nil
}

func (m *MockConfigStore) GetMetadata(_ context.Context, key string) ([]byte, error) {
	data, exists := m.metadata[key]
	if !exists {
		return nil, fmt.Errorf("metadata not found for key: %s", key)
	}
	return data, nil
}

func (m *MockConfigStore) DeleteMetadata(_ context.Context, key string) error {
	delete(m.metadata, key)
	return nil
}

func (m *MockConfigStore) Close() error {
	return nil
}
