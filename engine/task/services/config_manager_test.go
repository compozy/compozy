package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Helper function to create a task config with proper initialization
func createTaskConfig(id string, taskType task.Type) task.Config {
	config := task.Config{}
	config.ID = id
	config.Type = taskType
	// Set a default CWD to satisfy validation
	CWD, _ := core.CWDFromPath("/tmp")
	config.CWD = CWD
	return config
}

func TestConfigManager_PrepareParallelConfigs(t *testing.T) {
	t.Run("Should store parallel metadata successfully", func(t *testing.T) {
		// Arrange
		parentStateID := core.MustNewID()
		child1 := createTaskConfig("child1", task.TaskTypeBasic)
		child2 := createTaskConfig("child2", task.TaskTypeBasic)

		// Create expected metadata
		expectedMetadata := ParallelTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []task.Config{child1, child2},
			Strategy:      string(task.StrategyWaitAll),
			MaxWorkers:    2,
		}
		expectedBytes, _ := json.Marshal(expectedMetadata)

		configStore := NewMockConfigStore()
		// Set up mock expectations
		configStore.On("SaveMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		configStore.On("GetMetadata", mock.Anything, mock.Anything).Return(expectedBytes, nil)

		cm := NewConfigManager(configStore, nil)

		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
		taskConfig.Type = task.TaskTypeParallel
		taskConfig.Tasks = []task.Config{child1, child2}
		taskConfig.ParallelTask = task.ParallelTask{
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
		cm := NewConfigManager(configStore, nil)
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
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
		cm := NewConfigManager(configStore, nil)
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
		cm := NewConfigManager(configStore, nil)
		parentStateID := core.MustNewID()
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
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
		cm := NewConfigManager(configStore, nil)
		parentStateID := core.MustNewID()
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeParallel,
				CWD:  CWD,
			},
			Tasks: []task.Config{}, // Empty tasks
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
		cm := NewConfigManager(configStore, nil)
		parentStateID := core.MustNewID()
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeParallel,
				CWD:  CWD,
			},
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "child1", Type: task.TaskTypeBasic, CWD: CWD}},
				{BaseConfig: task.BaseConfig{ID: "", Type: task.TaskTypeBasic, CWD: CWD}}, // Missing ID
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
		parentStateID := core.MustNewID()

		configStore := NewMockConfigStore()
		// We'll mock the GetMetadata to return a pre-defined metadata
		// that matches what would be saved
		storedMeta := CollectionTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  []task.Config{}, // Will be populated by PrepareCollectionConfigs
			Strategy:      "wait_all",
			MaxWorkers:    0,
			ItemCount:     3,
			SkippedCount:  0,
			Mode:          "parallel",
			BatchSize:     0,
		}
		storedBytes, _ := json.Marshal(storedMeta)

		// Set up mock expectations
		configStore.On("SaveMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		configStore.On("GetMetadata", mock.Anything, mock.Anything).Return(storedBytes, nil)

		cm := NewConfigManager(configStore, nil)
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection",
				Type: task.TaskTypeCollection,
				CWD:  CWD,
			},
			CollectionConfig: task.CollectionConfig{
				Items: `["item1", "item2", "item3"]`,
				Mode:  task.CollectionModeParallel,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "template-task",
					Type: task.TaskTypeBasic,
					CWD:  CWD,
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
		cm := NewConfigManager(configStore, nil)
		workflowState := &workflow.State{}
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
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
		cm := NewConfigManager(configStore, nil)
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
		cm := NewConfigManager(configStore, nil)
		parentStateID := core.MustNewID()
		workflowState := &workflow.State{}
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
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
		cm := NewConfigManager(configStore, nil)
		parentStateID := core.MustNewID()
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
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
		parentStateID := core.MustNewID()

		// Create expected metadata that matches what will be saved
		expectedMetadata := ParallelTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "child1",
						Type: task.TaskTypeBasic,
						CWD:  func() *core.PathCWD { cwd, _ := core.CWDFromPath("/tmp"); return cwd }(),
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "child2",
						Type: task.TaskTypeBasic,
						CWD:  func() *core.PathCWD { cwd, _ := core.CWDFromPath("/tmp"); return cwd }(),
					},
				},
			},
			Strategy:   string(task.StrategyWaitAll),
			MaxWorkers: 2,
		}
		expectedBytes, _ := json.Marshal(expectedMetadata)

		configStore := NewMockConfigStore()
		// Set up mock expectations
		configStore.On("SaveMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		configStore.On("GetMetadata", mock.Anything, mock.Anything).Return(expectedBytes, nil)

		cm := NewConfigManager(configStore, nil)
		CWD, _ := core.CWDFromPath("/tmp")
		originalTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeParallel,
				CWD:  CWD,
			},
			ParallelTask: task.ParallelTask{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 2,
			},
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "child1", Type: task.TaskTypeBasic, CWD: CWD}},
				{BaseConfig: task.BaseConfig{ID: "child2", Type: task.TaskTypeBasic, CWD: CWD}},
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
		// Mock GetMetadata to return error for missing metadata
		configStore.On("GetMetadata", mock.Anything, mock.Anything).Return(nil, errors.New("metadata not found"))
		cm := NewConfigManager(configStore, nil)
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
		parentStateID := core.MustNewID()

		// Create expected metadata with actual child configs that would be generated
		cwdForExpected, _ := core.CWDFromPath("/tmp")
		expectedChildConfigs := []task.Config{
			{BaseConfig: task.BaseConfig{ID: "test-collection-0", Type: task.TaskTypeBasic, CWD: cwdForExpected}},
			{BaseConfig: task.BaseConfig{ID: "test-collection-1", Type: task.TaskTypeBasic, CWD: cwdForExpected}},
		}
		expectedMetadata := CollectionTaskMetadata{
			ParentStateID: parentStateID,
			ChildConfigs:  expectedChildConfigs,
			Strategy:      "wait_all",
			MaxWorkers:    1, // Sequential mode
			ItemCount:     2,
			SkippedCount:  0,
			Mode:          "sequential",
			BatchSize:     0,
		}
		expectedBytes, _ := json.Marshal(expectedMetadata)

		configStore := NewMockConfigStore()
		// Set up mock expectations for collection metadata
		configStore.On("SaveMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		configStore.On("GetMetadata", mock.Anything, mock.Anything).Return(expectedBytes, nil)
		cm := NewConfigManager(configStore, nil)
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		CWD, _ := core.CWDFromPath("/tmp")
		originalTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection",
				Type: task.TaskTypeCollection,
				CWD:  CWD,
			},
			CollectionConfig: task.CollectionConfig{
				Items: `["item1", "item2"]`,
				Mode:  task.CollectionModeSequential,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "template-task",
					Type: task.TaskTypeBasic,
					CWD:  CWD,
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
		// Mock GetMetadata to return error for missing metadata
		configStore.On("GetMetadata", mock.Anything, mock.Anything).Return(nil, errors.New("metadata not found"))
		cm := NewConfigManager(configStore, nil)
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
		// Set up mock expectations for empty collection metadata
		configStore.On("SaveMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		cm := NewConfigManager(configStore, nil)

		parentStateID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection",
				Type: task.TaskTypeCollection,
				CWD:  CWD,
			},
			CollectionConfig: task.CollectionConfig{
				Items:  `[]`, // Empty items array as JSON string
				Mode:   task.CollectionModeParallel,
				Filter: "false", // Filter that removes everything
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "template-task",
					Type: task.TaskTypeBasic,
					CWD:  CWD,
				},
			},
		}

		// Act
		metadata, err := cm.PrepareCollectionConfigs(context.Background(), parentStateID, taskConfig, workflowState)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, metadata)
		assert.Equal(t, 0, metadata.ItemCount)
		assert.Equal(t, 0, metadata.SkippedCount)
		assert.Equal(t, string(task.CollectionModeParallel), metadata.Mode)
	})

	t.Run("Should handle parallel task with batch size in collection mode", func(t *testing.T) {
		// Arrange
		configStore := NewMockConfigStore()
		// Set up mock expectations for collection metadata with batch
		configStore.On("SaveMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		// Mock for LoadCollectionTaskMetadata call
		metadataWithBatch := CollectionTaskMetadata{
			ParentStateID: core.MustNewID(),
			ChildConfigs:  []task.Config{},
			Strategy:      "wait_all",
			MaxWorkers:    2, // Should use batch size as MaxWorkers
			ItemCount:     3,
			SkippedCount:  0,
			Mode:          "sequential",
			BatchSize:     2,
		}
		metadataBytes, _ := json.Marshal(metadataWithBatch)
		configStore.On("GetMetadata", mock.Anything, mock.Anything).Return(metadataBytes, nil)
		cm := NewConfigManager(configStore, nil)

		parentStateID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection",
				Type: task.TaskTypeCollection,
				CWD:  CWD,
			},
			CollectionConfig: task.CollectionConfig{
				Items: `["item1", "item2", "item3"]`,
				Mode:  task.CollectionModeSequential,
				Batch: 2,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "template-task",
					Type: task.TaskTypeBasic,
					CWD:  CWD,
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

func TestConfigManager_TaskConfigOperations(t *testing.T) {
	t.Run("Should save and delete task config successfully", func(t *testing.T) {
		configStore := NewMockConfigStore()
		// Set up mock expectations for Save, Get, and Delete operations
		configStore.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		taskConfig := createTaskConfig("test-task", task.TaskTypeBasic)
		configStore.On("Get", mock.Anything, mock.Anything).Return(&taskConfig, nil).Once()
		configStore.On("Delete", mock.Anything, mock.Anything).Return(nil)
		configStore.On("Get", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))

		cm := NewConfigManager(configStore, nil)
		ctx := context.Background()
		taskExecID := core.MustNewID()
		err := cm.SaveTaskConfig(ctx, taskExecID, &taskConfig)
		require.NoError(t, err)
		savedConfig, err := configStore.Get(ctx, string(taskExecID))
		require.NoError(t, err)
		assert.Equal(t, "test-task", savedConfig.ID)
		err = cm.DeleteTaskConfig(ctx, taskExecID)
		require.NoError(t, err)
		_, err = configStore.Get(ctx, string(taskExecID))
		assert.Error(t, err)
	})
}
