package activities

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	utils "github.com/compozy/compozy/test/helpers"
)

func TestNormalizeWaitProcessor_Run(t *testing.T) {
	t.Run("Should inherit CWD from parent wait task", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create activity
		activity := NewNormalizeWaitProcessor([]*workflow.Config{workflowConfig}, workflowRepo, taskRepo)

		// Test data
		parentCWD := &core.PathCWD{Path: "/parent/working/directory"}
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-wait-task",
				Type: task.TaskTypeWait,
				CWD:  parentCWD,
			},
		}

		processorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "processor-task",
				Type: task.TaskTypeBasic,
				// CWD not set, should inherit from parent
			},
			BasicTask: task.BasicTask{
				Action: "process_signal",
			},
		}

		signal := &task.SignalEnvelope{
			Payload: map[string]any{
				"status": "ready",
			},
			Metadata: task.SignalMetadata{
				SignalID:      "signal-123",
				ReceivedAtUTC: time.Now(),
				WorkflowID:    workflowID,
				Source:        "test",
			},
		}

		input := &NormalizeWaitProcessorInput{
			WorkflowID:       workflowID,
			WorkflowExecID:   workflowExecID,
			ProcessorConfig:  processorConfig,
			ParentTaskConfig: parentConfig,
			Signal:           signal,
		}

		// Act
		result, err := activity.Run(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.CWD, "Processor should have inherited CWD")
		assert.Equal(t, parentCWD.Path, result.CWD.Path, "Processor should inherit CWD from parent wait task")
	})

	t.Run("Should inherit FilePath from parent wait task", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create activity
		activity := NewNormalizeWaitProcessor([]*workflow.Config{workflowConfig}, workflowRepo, taskRepo)

		// Test data
		parentFilePath := "/parent/config.yaml"
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "parent-wait-task",
				Type:     task.TaskTypeWait,
				FilePath: parentFilePath,
			},
		}

		processorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "processor-task",
				Type: task.TaskTypeBasic,
				// FilePath not set, should inherit from parent
			},
			BasicTask: task.BasicTask{
				Action: "process_signal",
			},
		}

		signal := &task.SignalEnvelope{
			Payload: map[string]any{
				"status": "ready",
			},
			Metadata: task.SignalMetadata{
				SignalID:      "signal-123",
				ReceivedAtUTC: time.Now(),
				WorkflowID:    workflowID,
				Source:        "test",
			},
		}

		input := &NormalizeWaitProcessorInput{
			WorkflowID:       workflowID,
			WorkflowExecID:   workflowExecID,
			ProcessorConfig:  processorConfig,
			ParentTaskConfig: parentConfig,
			Signal:           signal,
		}

		// Act
		result, err := activity.Run(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, parentFilePath, result.FilePath, "Processor should inherit FilePath from parent wait task")
	})

	t.Run("Should not override explicit processor settings", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create activity
		activity := NewNormalizeWaitProcessor([]*workflow.Config{workflowConfig}, workflowRepo, taskRepo)

		// Test data
		parentCWD := &core.PathCWD{Path: "/parent/working/directory"}
		parentFilePath := "/parent/config.yaml"
		processorCWD := &core.PathCWD{Path: "/processor/working/directory"}
		processorFilePath := "/processor/config.yaml"

		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "parent-wait-task",
				Type:     task.TaskTypeWait,
				CWD:      parentCWD,
				FilePath: parentFilePath,
			},
		}

		processorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "processor-task",
				Type:     task.TaskTypeBasic,
				CWD:      processorCWD,      // Explicitly set, should not be overridden
				FilePath: processorFilePath, // Explicitly set, should not be overridden
			},
			BasicTask: task.BasicTask{
				Action: "process_signal",
			},
		}

		signal := &task.SignalEnvelope{
			Payload: map[string]any{
				"status": "ready",
			},
			Metadata: task.SignalMetadata{
				SignalID:      "signal-123",
				ReceivedAtUTC: time.Now(),
				WorkflowID:    workflowID,
				Source:        "test",
			},
		}

		input := &NormalizeWaitProcessorInput{
			WorkflowID:       workflowID,
			WorkflowExecID:   workflowExecID,
			ProcessorConfig:  processorConfig,
			ParentTaskConfig: parentConfig,
			Signal:           signal,
		}

		// Act
		result, err := activity.Run(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, processorCWD, result.CWD, "Processor should retain its explicit CWD setting")
		assert.Equal(t, processorFilePath, result.FilePath, "Processor should retain its explicit FilePath setting")
	})

	t.Run("Should handle nil parent config gracefully", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create activity
		activity := NewNormalizeWaitProcessor([]*workflow.Config{workflowConfig}, workflowRepo, taskRepo)

		// Test data
		processorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "processor-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "process_signal",
			},
		}

		signal := &task.SignalEnvelope{
			Payload: map[string]any{
				"status": "ready",
			},
			Metadata: task.SignalMetadata{
				SignalID:      "signal-123",
				ReceivedAtUTC: time.Now(),
				WorkflowID:    workflowID,
				Source:        "test",
			},
		}

		input := &NormalizeWaitProcessorInput{
			WorkflowID:       workflowID,
			WorkflowExecID:   workflowExecID,
			ProcessorConfig:  processorConfig,
			ParentTaskConfig: nil, // nil parent config
			Signal:           signal,
		}

		// Act
		result, err := activity.Run(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Nil(t, result.CWD, "CWD should remain nil with nil parent")
		assert.Empty(t, result.FilePath, "FilePath should remain empty with nil parent")
		assert.Equal(t, task.TaskTypeBasic, result.Type, "Type should remain unchanged")
	})

	t.Run("Should inherit both CWD and FilePath when both are missing from processor", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create activity
		activity := NewNormalizeWaitProcessor([]*workflow.Config{workflowConfig}, workflowRepo, taskRepo)

		// Test data
		parentCWD := &core.PathCWD{Path: "/parent/working/directory"}
		parentFilePath := "/parent/config.yaml"
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "parent-wait-task",
				Type:     task.TaskTypeWait,
				CWD:      parentCWD,
				FilePath: parentFilePath,
			},
		}

		processorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "processor-task",
				Type: task.TaskTypeBasic,
				// Neither CWD nor FilePath set, should inherit both
			},
			BasicTask: task.BasicTask{
				Action: "process_signal",
			},
		}

		signal := &task.SignalEnvelope{
			Payload: map[string]any{
				"status": "ready",
			},
			Metadata: task.SignalMetadata{
				SignalID:      "signal-123",
				ReceivedAtUTC: time.Now(),
				WorkflowID:    workflowID,
				Source:        "test",
			},
		}

		input := &NormalizeWaitProcessorInput{
			WorkflowID:       workflowID,
			WorkflowExecID:   workflowExecID,
			ProcessorConfig:  processorConfig,
			ParentTaskConfig: parentConfig,
			Signal:           signal,
		}

		// Act
		result, err := activity.Run(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.CWD, "Processor should have inherited CWD")
		assert.Equal(t, parentCWD.Path, result.CWD.Path, "Processor should inherit CWD from parent wait task")
		assert.Equal(t, parentFilePath, result.FilePath, "Processor should inherit FilePath from parent wait task")
	})

	t.Run("Should normalize processor with signal context", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create activity
		activity := NewNormalizeWaitProcessor([]*workflow.Config{workflowConfig}, workflowRepo, taskRepo)

		// Test data - processor with templates that use signal data
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-wait-task",
				Type: task.TaskTypeWait,
			},
		}

		processorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "processor-{{ .signal.payload.type }}",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "process_{{ .signal.payload.action }}",
			},
		}

		signal := &task.SignalEnvelope{
			Payload: map[string]any{
				"type":   "approval",
				"action": "validate",
			},
			Metadata: task.SignalMetadata{
				SignalID:      "signal-123",
				ReceivedAtUTC: time.Now(),
				WorkflowID:    workflowID,
				Source:        "test",
			},
		}

		input := &NormalizeWaitProcessorInput{
			WorkflowID:       workflowID,
			WorkflowExecID:   workflowExecID,
			ProcessorConfig:  processorConfig,
			ParentTaskConfig: parentConfig,
			Signal:           signal,
		}

		// Act
		result, err := activity.Run(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "processor-approval", result.ID, "Processor ID should be normalized with signal data")
		assert.Equal(t, "process_validate", result.Action, "Processor action should be normalized with signal data")
	})
}
