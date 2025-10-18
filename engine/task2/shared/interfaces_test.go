package shared

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func TestResponseInput_Structure(t *testing.T) {
	t.Run("Should have all required fields", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{}
		taskConfig.Type = task.TaskTypeBasic
		taskState := &task.State{TaskExecID: core.MustNewID()}
		workflowConfig := &workflow.Config{}
		workflowState := &workflow.State{}

		// Act
		input := &ResponseInput{
			TaskConfig:     taskConfig,
			TaskState:      taskState,
			WorkflowConfig: workflowConfig,
			WorkflowState:  workflowState,
		}

		// Assert
		assert.NotNil(t, input.TaskConfig)
		assert.NotNil(t, input.TaskState)
		assert.NotNil(t, input.WorkflowConfig)
		assert.NotNil(t, input.WorkflowState)
		assert.Nil(t, input.ExecutionError)
	})

	t.Run("Should handle execution error field", func(t *testing.T) {
		// Arrange
		executionError := assert.AnError

		// Act
		input := &ResponseInput{
			ExecutionError: executionError,
		}

		// Assert
		assert.Equal(t, executionError, input.ExecutionError)
	})
}

func TestResponseOutput_Structure(t *testing.T) {
	t.Run("Should contain response and state fields", func(t *testing.T) {
		// Arrange
		response := &task.MainTaskResponse{}
		state := &task.State{TaskExecID: core.MustNewID()}

		// Act
		output := &ResponseOutput{
			Response: response,
			State:    state,
		}

		// Assert
		assert.Equal(t, response, output.Response)
		assert.Equal(t, state, output.State)
	})
}

func TestExpansionResult_Structure(t *testing.T) {
	t.Run("Should track child configs and counts", func(t *testing.T) {
		// Arrange
		config1 := &task.Config{}
		config1.Type = task.TaskTypeBasic
		config2 := &task.Config{}
		config2.Type = task.TaskTypeBasic
		childConfigs := []*task.Config{config1, config2}

		// Act
		result := &ExpansionResult{
			ChildConfigs: childConfigs,
			ItemCount:    2,
			SkippedCount: 0,
		}

		// Assert
		assert.Len(t, result.ChildConfigs, 2)
		assert.Equal(t, 2, result.ItemCount)
		assert.Equal(t, 0, result.SkippedCount)
	})

	t.Run("Should handle skipped items", func(t *testing.T) {
		// Arrange & Act
		result := &ExpansionResult{
			ChildConfigs: []*task.Config{},
			ItemCount:    0,
			SkippedCount: 5,
		}

		// Assert
		assert.Empty(t, result.ChildConfigs)
		assert.Equal(t, 0, result.ItemCount)
		assert.Equal(t, 5, result.SkippedCount)
	})
}

// MockTaskResponseHandler for testing interface compliance
type MockTaskResponseHandler struct {
	handleResponseFunc func(context.Context, *ResponseInput) (*ResponseOutput, error)
	taskType           task.Type
}

func (m *MockTaskResponseHandler) HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error) {
	if m.handleResponseFunc != nil {
		return m.handleResponseFunc(ctx, input)
	}
	return &ResponseOutput{}, nil
}

func (m *MockTaskResponseHandler) Type() task.Type {
	return m.taskType
}

func TestTaskResponseHandler_Interface(t *testing.T) {
	t.Run("Should implement TaskResponseHandler interface", func(t *testing.T) {
		// Arrange
		handler := &MockTaskResponseHandler{
			taskType: task.TaskTypeBasic,
		}

		// Act & Assert - This ensures interface compliance
		var _ TaskResponseHandler = handler
		assert.Equal(t, task.TaskTypeBasic, handler.Type())
	})

	t.Run("Should handle response processing", func(t *testing.T) {
		// Arrange
		expectedOutput := &ResponseOutput{
			Response: &task.MainTaskResponse{},
		}
		handler := &MockTaskResponseHandler{
			handleResponseFunc: func(_ context.Context, _ *ResponseInput) (*ResponseOutput, error) {
				return expectedOutput, nil
			},
		}

		config := &task.Config{}
		config.Type = task.TaskTypeBasic
		input := &ResponseInput{
			TaskConfig: config,
		}

		// Act
		output, err := handler.HandleResponse(context.Background(), input)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedOutput, output)
	})
}

// MockParentStatusManager for testing interface compliance
type MockParentStatusManager struct {
	mock.Mock
}

func (m *MockParentStatusManager) UpdateParentStatus(
	ctx context.Context,
	parentStateID core.ID,
	strategy task.ParallelStrategy,
) error {
	args := m.Called(ctx, parentStateID, strategy)
	return args.Error(0)
}

func (m *MockParentStatusManager) GetAggregatedStatus(
	ctx context.Context,
	parentStateID core.ID,
	strategy task.ParallelStrategy,
) (core.StatusType, error) {
	args := m.Called(ctx, parentStateID, strategy)
	return args.Get(0).(core.StatusType), args.Error(1)
}

func TestParentStatusManager_Interface(t *testing.T) {
	t.Run("Should implement ParentStatusManager interface", func(_ *testing.T) {
		// Arrange
		manager := &MockParentStatusManager{}

		// Act & Assert - This ensures interface compliance
		var _ ParentStatusManager = manager
	})

	t.Run("Should handle status updates", func(t *testing.T) {
		// Arrange
		parentID := core.MustNewID()
		strategy := task.StrategyWaitAll

		manager := &MockParentStatusManager{}
		manager.On("UpdateParentStatus", mock.Anything, parentID, strategy).Return(nil)

		// Act
		err := manager.UpdateParentStatus(context.Background(), parentID, strategy)

		// Assert
		require.NoError(t, err)
		manager.AssertExpectations(t)
	})
}

// MockCollectionExpander for testing interface compliance
type MockCollectionExpander struct {
	expandItemsFunc       func(context.Context, *task.Config, *workflow.State, *workflow.Config) (*ExpansionResult, error)
	validateExpansionFunc func(*ExpansionResult) error
}

func (m *MockCollectionExpander) ExpandItems(
	ctx context.Context,
	config *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*ExpansionResult, error) {
	if m.expandItemsFunc != nil {
		return m.expandItemsFunc(ctx, config, workflowState, workflowConfig)
	}
	return &ExpansionResult{}, nil
}

func (m *MockCollectionExpander) ValidateExpansion(_ context.Context, result *ExpansionResult) error {
	if m.validateExpansionFunc != nil {
		return m.validateExpansionFunc(result)
	}
	return nil
}

func TestCollectionExpander_Interface(t *testing.T) {
	t.Run("Should implement CollectionExpander interface", func(_ *testing.T) {
		// Arrange
		expander := &MockCollectionExpander{}

		// Act & Assert - This ensures interface compliance
		var _ CollectionExpander = expander
	})

	t.Run("Should handle item expansion", func(t *testing.T) {
		// Arrange
		config1 := &task.Config{}
		config1.Type = task.TaskTypeBasic
		config2 := &task.Config{}
		config2.Type = task.TaskTypeBasic
		config3 := &task.Config{}
		config3.Type = task.TaskTypeBasic
		expectedResult := &ExpansionResult{
			ItemCount:    3,
			ChildConfigs: []*task.Config{config1, config2, config3},
		}

		expander := &MockCollectionExpander{
			expandItemsFunc: func(_ context.Context, _ *task.Config, _ *workflow.State, _ *workflow.Config) (*ExpansionResult, error) {
				return expectedResult, nil
			},
		}

		// Act
		result, err := expander.ExpandItems(context.Background(), &task.Config{}, &workflow.State{}, &workflow.Config{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		assert.Equal(t, 3, result.ItemCount)
		assert.Len(t, result.ChildConfigs, 3)
	})
}
