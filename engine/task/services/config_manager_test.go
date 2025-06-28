package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/engine/agent"
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

		configStore := NewTestConfigStore(t)
		defer configStore.Close()

		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)

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
		err = cm.PrepareParallelConfigs(context.Background(), parentStateID, taskConfig)

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
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
		taskConfig.Type = task.TaskTypeParallel

		// Act
		err = cm.PrepareParallelConfigs(context.Background(), "", taskConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should fail with nil task config", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		parentStateID := core.MustNewID()

		// Act
		err = cm.PrepareParallelConfigs(context.Background(), parentStateID, nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config cannot be nil")
	})

	t.Run("Should fail with wrong task type", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		parentStateID := core.MustNewID()
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
		taskConfig.Type = task.TaskTypeBasic

		// Act
		err = cm.PrepareParallelConfigs(context.Background(), parentStateID, taskConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config must be parallel type")
	})

	t.Run("Should fail with no child tasks", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
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
		err = cm.PrepareParallelConfigs(context.Background(), parentStateID, taskConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel task must have at least one child task")
	})

	t.Run("Should fail with child config missing ID", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
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
		err = cm.PrepareParallelConfigs(context.Background(), parentStateID, taskConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child config at index 1 missing required ID field")
	})
}

func TestConfigManager_PrepareCollectionConfigs(t *testing.T) {
	t.Run("Should store collection metadata successfully", func(t *testing.T) {
		// Arrange
		parentStateID := core.MustNewID()
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		workflowState := &workflow.State{
			WorkflowExecID: core.MustNewID(),
			WorkflowID:     "test-workflow",
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
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
		metadata, err := cm.PrepareCollectionConfigs(
			context.Background(),
			parentStateID,
			taskConfig,
			workflowState,
			workflowConfig,
		)

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
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		workflowState := &workflow.State{}
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
		taskConfig.Type = task.TaskTypeCollection

		// Act
		_, err = cm.PrepareCollectionConfigs(
			context.Background(),
			"",
			taskConfig,
			workflowState,
			&workflow.Config{ID: "test-workflow"},
		)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent state ID cannot be empty")
	})

	t.Run("Should fail with nil task config", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		parentStateID := core.MustNewID()
		workflowState := &workflow.State{}

		// Act
		_, err = cm.PrepareCollectionConfigs(
			context.Background(),
			parentStateID,
			nil,
			workflowState,
			&workflow.Config{ID: "test-workflow"},
		)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config cannot be nil")
	})

	t.Run("Should fail with wrong task type", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
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
		_, err = cm.PrepareCollectionConfigs(
			context.Background(),
			parentStateID,
			taskConfig,
			workflowState,
			&workflow.Config{ID: "test-workflow"},
		)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config must be collection type")
	})

	t.Run("Should fail with nil workflow state", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		parentStateID := core.MustNewID()
		CWD, _ := core.CWDFromPath("/tmp")
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD: CWD,
			},
		}
		taskConfig.Type = task.TaskTypeCollection

		// Act
		_, err = cm.PrepareCollectionConfigs(
			context.Background(),
			parentStateID,
			taskConfig,
			nil,
			&workflow.Config{ID: "test-workflow"},
		)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state cannot be nil")
	})
}

func TestConfigManager_LoadParallelTaskMetadata(t *testing.T) {
	t.Run("Should load parallel metadata successfully", func(t *testing.T) {
		// Arrange
		parentStateID := core.MustNewID()

		configStore := NewTestConfigStore(t)
		defer configStore.Close()

		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
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
		err = cm.PrepareParallelConfigs(context.Background(), parentStateID, originalTaskConfig)
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
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		parentStateID := core.MustNewID()

		// Act
		_, err = cm.LoadParallelTaskMetadata(context.Background(), parentStateID)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get parallel task metadata")
	})
}

func TestConfigManager_LoadCollectionTaskMetadata(t *testing.T) {
	t.Run("Should load collection metadata successfully", func(t *testing.T) {
		// Arrange
		parentStateID := core.MustNewID()

		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
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
		_, err = cm.PrepareCollectionConfigs(
			context.Background(),
			parentStateID,
			originalTaskConfig,
			workflowState,
			&workflow.Config{ID: "test-workflow"},
		)
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
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		parentStateID := core.MustNewID()

		// Act
		_, err = cm.LoadCollectionTaskMetadata(context.Background(), parentStateID)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get collection task metadata")
	})
}

func TestConfigManager_EdgeCases(t *testing.T) {
	t.Run("Should handle collection with filter that removes all items", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)

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
		metadata, err := cm.PrepareCollectionConfigs(
			context.Background(),
			parentStateID,
			taskConfig,
			workflowState,
			&workflow.Config{ID: "test-workflow"},
		)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, metadata)
		assert.Equal(t, 0, metadata.ItemCount)
		assert.Equal(t, 0, metadata.SkippedCount)
		assert.Equal(t, string(task.CollectionModeParallel), metadata.Mode)
	})

	t.Run("Should handle parallel task with batch size in collection mode", func(t *testing.T) {
		// Arrange
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)

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
		metadata, err := cm.PrepareCollectionConfigs(
			context.Background(),
			parentStateID,
			taskConfig,
			workflowState,
			&workflow.Config{ID: "test-workflow"},
		)

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
		configStore := NewTestConfigStore(t)
		defer configStore.Close()

		cm, err := NewConfigManager(configStore, nil)
		require.NoError(t, err)
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskConfig := createTaskConfig("test-task", task.TaskTypeBasic)

		// Save the config
		err = cm.SaveTaskConfig(ctx, taskExecID, &taskConfig)
		require.NoError(t, err)

		// Load it back
		savedConfig, err := configStore.Get(ctx, string(taskExecID))
		require.NoError(t, err)
		assert.Equal(t, "test-task", savedConfig.ID)

		// Delete it
		err = cm.DeleteTaskConfig(ctx, taskExecID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = configStore.Get(ctx, string(taskExecID))
		assert.Error(t, err)
	})
}

func TestConfigManager_CollectionWithAgentIntegration(t *testing.T) {
	t.Run("Should process agent prompts with item context during collection child creation", func(t *testing.T) {
		// Arrange
		cwd := &core.PathCWD{Path: "/test"}
		configStore := NewTestConfigStore(t)
		defer configStore.Close()
		configManager, err := NewConfigManager(configStore, cwd)
		require.NoError(t, err)

		// Create a collection task with an agent that uses {{ .item }}
		collectionTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "analyze-activities",
				Type: task.TaskTypeCollection,
				CWD:  cwd,
			},
			CollectionConfig: task.CollectionConfig{
				Items: `["running", "swimming", "cycling"]`,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "analyze-activity-{{ .index }}",
					Type: task.TaskTypeBasic,
					Agent: &agent.Config{
						ID:           "activity-analyzer",
						Instructions: "Analyze fitness activities",
						Actions: []*agent.ActionConfig{
							{
								ID:       "analyze",
								JSONMode: true,
								Prompt:   `Analyze the activity "{{ .item }}" and provide health benefits`,
							},
						},
					},
					Outputs: &core.Input{
						"activity": "{{ .item }}",
						"analysis": "{{ .output }}",
					},
				},
				BasicTask: task.BasicTask{
					Action: "analyze",
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Tasks:      make(map[string]*task.State),
		}

		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}

		// Act
		metadata, err := configManager.PrepareCollectionConfigs(
			context.Background(),
			core.ID("parent-state-123"),
			collectionTask,
			workflowState,
			workflowConfig,
		)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 3, metadata.ItemCount)
		assert.Equal(t, 0, metadata.SkippedCount)

		// Load the stored metadata
		storedMetadata, err := configManager.LoadCollectionTaskMetadata(
			context.Background(),
			core.ID("parent-state-123"),
		)
		require.NoError(t, err)
		require.NotNil(t, storedMetadata)
		assert.Len(t, storedMetadata.ChildConfigs, 3)

		// Verify that agent prompts have been processed with item context
		for i, childConfig := range storedMetadata.ChildConfigs {
			expectedActivity := []string{"running", "swimming", "cycling"}[i]

			// Check that the child task ID was processed
			assert.Equal(t, "analyze-activity-"+string(rune('0'+i)), childConfig.ID)

			// Check that the agent prompt was processed with the item context
			require.NotNil(t, childConfig.Agent)
			require.Len(t, childConfig.Agent.Actions, 1)

			expectedPrompt := `Analyze the activity "` + expectedActivity + `" and provide health benefits`
			assert.Equal(t, expectedPrompt, childConfig.Agent.Actions[0].Prompt)
			assert.NotContains(t, childConfig.Agent.Actions[0].Prompt, "{{ .item }}")

			// Check that the outputs field was NOT processed during child creation
			// Output transformation happens AFTER task execution
			require.NotNil(t, childConfig.Outputs)
			activityOutput, ok := (*childConfig.Outputs)["activity"]
			require.True(t, ok)
			assert.Equal(t, "{{ .item }}", activityOutput, "outputs should not be processed during child creation")

			// The analysis field should also remain unprocessed
			analysisOutput, ok := (*childConfig.Outputs)["analysis"]
			require.True(t, ok)
			assert.Equal(t, "{{ .output }}", analysisOutput)
		}
	})
}
