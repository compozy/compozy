package core_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// Mock normalizer
type mockNormalizer struct {
	mock.Mock
}

func (m *mockNormalizer) Type() task.Type {
	args := m.Called()
	return args.Get(0).(task.Type)
}

func (m *mockNormalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	args := m.Called(config, ctx)
	return args.Error(0)
}

// Mock factory
type mockNormalizerFactory struct {
	mock.Mock
}

func (m *mockNormalizerFactory) CreateNormalizer(taskType task.Type) (contracts.TaskNormalizer, error) {
	args := m.Called(taskType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(contracts.TaskNormalizer), args.Error(1)
}

func TestConfigNormalizer_NormalizeTask(t *testing.T) {
	t.Run("Should normalize task successfully", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		mockTaskNormalizer := &mockNormalizer{}
		factory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockTaskNormalizer, nil)
		mockTaskNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: enginecore.MustNewID(),
			Tasks:          make(map[string]*task.State),
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Env: &enginecore.EnvMap{
					"TASK_VAR": "task_value",
				},
			},
		}
		// Act
		err = normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		// Assert
		assert.NoError(t, err)
		factory.AssertExpectations(t)
		mockTaskNormalizer.AssertExpectations(t)
	})

	t.Run("Should return error when factory fails to create normalizer", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		factory.On("CreateNormalizer", task.TaskTypeBasic).Return(nil, errors.New("unsupported type"))
		workflowState := &workflow.State{}
		workflowConfig := &workflow.Config{}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Act
		err = normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create normalizer for task test-task")
		factory.AssertExpectations(t)
	})

	t.Run("Should return error when normalizer fails", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		mockTaskNormalizer := &mockNormalizer{}
		factory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockTaskNormalizer, nil)
		mockTaskNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(errors.New("normalization failed"))
		workflowState := &workflow.State{}
		workflowConfig := &workflow.Config{}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Act
		err = normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize task test-task")
		factory.AssertExpectations(t)
		mockTaskNormalizer.AssertExpectations(t)
	})
}

func TestConfigNormalizer_NormalizeAllTasks(t *testing.T) {
	t.Run("Should normalize all workflow tasks", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		mockTaskNormalizer := &mockNormalizer{}
		factory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockTaskNormalizer, nil)
		mockTaskNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: enginecore.MustNewID(),
			Tasks:          make(map[string]*task.State),
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "task-1",
						Type: task.TaskTypeBasic,
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "task-2",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}
		// Act
		err = normalizer.NormalizeAllTasks(workflowState, workflowConfig)
		// Assert
		assert.NoError(t, err)
		// Should normalize both tasks
		assert.Equal(t, 2, len(mockTaskNormalizer.Calls))
		factory.AssertExpectations(t)
		mockTaskNormalizer.AssertExpectations(t)
	})

	t.Run("Should return error when first task fails", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		mockTaskNormalizer := &mockNormalizer{}
		factory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockTaskNormalizer, nil).Once()
		mockTaskNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(errors.New("task-1 failed"))
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: enginecore.MustNewID(),
			Tasks:          make(map[string]*task.State),
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "task-1",
						Type: task.TaskTypeBasic,
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "task-2",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}
		// Act
		err = normalizer.NormalizeAllTasks(workflowState, workflowConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task-1 failed")
		// Should NOT normalize task-2 because it returns early on first error
		mockTaskNormalizer.AssertExpectations(t)
		factory.AssertExpectations(t)
	})

	t.Run("Should handle nil workflow config", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		workflowState := &workflow.State{}
		// Act & Assert - Should panic with nil workflow config
		assert.Panics(t, func() {
			_ = normalizer.NormalizeAllTasks(workflowState, nil)
		})
	})
}

func TestConfigNormalizer_NormalizeSubTask(t *testing.T) {
	t.Run("Should normalize sub-task with parent context", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		mockTaskNormalizer := &mockNormalizer{}
		// Set up expectations for normalizer factory
		factory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockTaskNormalizer, nil)
		mockTaskNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)
		parentContext := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: enginecore.MustNewID(),
				Tasks:          make(map[string]*task.State),
			},
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			Variables: map[string]any{
				"parent_var": "parent_value",
			},
		}
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeComposite,
				Env: &enginecore.EnvMap{
					"PARENT_VAR": "parent_value",
				},
			},
		}
		subTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "sub-task",
				Type: task.TaskTypeBasic,
				Env: &enginecore.EnvMap{
					"SUB_VAR": "sub_value",
				},
			},
		}
		// Act
		err = normalizer.NormalizeSubTask(parentContext, parentTask, subTask)
		// Assert
		assert.NoError(t, err)
		factory.AssertExpectations(t)
		mockTaskNormalizer.AssertExpectations(t)
		// Verify that Normalize was called with proper context
		assert.Equal(t, 1, len(mockTaskNormalizer.Calls))
		normalizeCall := mockTaskNormalizer.Calls[0]
		ctx := normalizeCall.Arguments[1].(*shared.NormalizationContext)
		assert.NotNil(t, ctx.ParentTask)
		assert.Equal(t, parentTask, ctx.ParentTask)
	})

	t.Run("Should return error when normalizer creation fails", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		// Set up factory to return error
		factory.On("CreateNormalizer", task.TaskTypeBasic).Return(nil, errors.New("unsupported type"))
		parentContext := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: enginecore.MustNewID(),
				Tasks:          make(map[string]*task.State),
			},
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			Variables: map[string]any{
				"parent_var": "parent_value",
			},
		}
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeComposite,
			},
		}
		subTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "sub-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Act
		err = normalizer.NormalizeSubTask(parentContext, parentTask, subTask)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create normalizer for sub-task sub-task")
		factory.AssertExpectations(t)
	})

	t.Run("Should return error when parent context is nil", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeComposite,
			},
		}
		subTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "sub-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Act
		err = normalizer.NormalizeSubTask(nil, parentTask, subTask)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent context is nil")
	})

	t.Run("Should return error when parent task is nil", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		envMerger := core.NewEnvMerger()
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := core.NewConfigNormalizer(factory, envMerger, contextBuilder)
		parentContext := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: enginecore.MustNewID(),
			},
		}
		subTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "sub-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Act
		err = normalizer.NormalizeSubTask(parentContext, nil, subTask)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent task is nil")
	})
}
