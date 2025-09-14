package activities

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
)

// Mock for task.Repository
type mockTaskRepository struct {
	mock.Mock
}

// Basic CRUD operations
func (m *mockTaskRepository) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *mockTaskRepository) UpsertState(ctx context.Context, state *task.State) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *mockTaskRepository) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	args := m.Called(ctx, taskExecID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.State), args.Error(1)
}

// Transaction operations
func (m *mockTaskRepository) WithTransaction(ctx context.Context, fn func(task.Repository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *mockTaskRepository) GetStateForUpdate(
	ctx context.Context,
	taskExecID core.ID,
) (*task.State, error) {
	args := m.Called(ctx, taskExecID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.State), args.Error(1)
}

// Workflow-level operations
func (m *mockTaskRepository) ListTasksInWorkflow(
	ctx context.Context,
	workflowExecID core.ID,
) (map[string]*task.State, error) {
	args := m.Called(ctx, workflowExecID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*task.State), args.Error(1)
}

func (m *mockTaskRepository) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	args := m.Called(ctx, workflowExecID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *mockTaskRepository) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	args := m.Called(ctx, workflowExecID, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *mockTaskRepository) ListTasksByTool(
	ctx context.Context,
	workflowExecID core.ID,
	toolID string,
) ([]*task.State, error) {
	args := m.Called(ctx, workflowExecID, toolID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

// Parent-child relationship operations
func (m *mockTaskRepository) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	args := m.Called(ctx, parentStateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *mockTaskRepository) GetChildByTaskID(
	ctx context.Context,
	parentStateID core.ID,
	taskID string,
) (*task.State, error) {
	args := m.Called(ctx, parentStateID, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.State), args.Error(1)
}

func (m *mockTaskRepository) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	args := m.Called(ctx, rootStateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *mockTaskRepository) ListChildrenOutputs(
	ctx context.Context,
	parentStateID core.ID,
) (map[string]*core.Output, error) {
	args := m.Called(ctx, parentStateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*core.Output), args.Error(1)
}

// Progress aggregation operations
func (m *mockTaskRepository) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	args := m.Called(ctx, parentStateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.ProgressInfo), args.Error(1)
}

func TestProcessParentTask(t *testing.T) {
	// Create a logger and add it to context
	log := logger.NewForTests()
	ctx := logger.ContextWithLogger(context.Background(), log)

	t.Run("Should process parallel task successfully with all children succeeded", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:         "parent-task",
			TaskExecID:     parentExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeParallel,
			},
		}

		progressInfo := &task.ProgressInfo{
			TotalChildren: 3,
			SuccessCount:  3,
			FailedCount:   0,
			TerminalCount: 3,
			StatusCounts: map[core.StatusType]int{
				core.StatusSuccess: 3,
			},
		}

		// Mock expectations
		mockRepo.On("GetProgressInfo", ctx, parentExecID).Return(progressInfo, nil)
		mockRepo.On("ListChildrenOutputs", ctx, parentExecID).Return(map[string]*core.Output{
			"child1": {"result": "success1"},
			"child2": {"result": "success2"},
			"child3": {"result": "success3"},
		}, nil)

		// Act
		err := processParentTask(ctx, mockRepo, parentState, taskConfig, task.TaskTypeParallel)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, parentState.Status)
		assert.NotNil(t, parentState.Output)
		assert.Contains(t, *parentState.Output, "progress_info")
		assert.Contains(t, *parentState.Output, "outputs")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should process collection task with mixed results", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:         "parent-task",
			TaskExecID:     parentExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
		}

		progressInfo := &task.ProgressInfo{
			TotalChildren: 3,
			SuccessCount:  2,
			FailedCount:   1,
			TerminalCount: 3,
			StatusCounts: map[core.StatusType]int{
				core.StatusSuccess: 2,
				core.StatusFailed:  1,
			},
		}

		// Mock expectations
		mockRepo.On("GetProgressInfo", ctx, parentExecID).Return(progressInfo, nil)
		mockRepo.On("ListChildrenOutputs", ctx, parentExecID).Return(map[string]*core.Output{
			"child1": {"result": "success1"},
			"child2": {"result": "success2"},
			// child3 failed, no output
		}, nil)
		// For collection tasks with failed children, we need to mock ListChildren
		failedChild := &task.State{
			TaskID: "child3",
			Status: core.StatusFailed,
			Error:  &core.Error{Code: "TASK_FAILED", Message: "Collection item failed"},
		}
		mockRepo.On("ListChildren", ctx, parentExecID).Return([]*task.State{failedChild}, nil)

		// Act
		err := processParentTask(ctx, mockRepo, parentState, taskConfig, task.TaskTypeCollection)

		// Assert
		require.Error(t, err)
		// Collection tasks fail when there are failed children
		assert.Equal(t, core.StatusFailed, parentState.Status)
		assert.Contains(t, err.Error(), "collection task failed")
		assert.NotNil(t, parentState.Output)
		assert.Contains(t, *parentState.Output, "progress_info")
		assert.Contains(t, *parentState.Output, "outputs")
		outputs := (*parentState.Output)["outputs"].(map[string]any)
		assert.Len(t, outputs, 2) // Only successful children have outputs
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle task type mismatch error", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: core.MustNewID(),
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeBasic, // Wrong type
			},
		}

		// Act
		err := processParentTask(ctx, mockRepo, parentState, taskConfig, task.TaskTypeParallel)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected parallel task type, got: basic")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle progress info fetch error", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:         "parent-task",
			TaskExecID:     parentExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeParallel,
			},
		}

		// Mock expectations
		mockRepo.On("GetProgressInfo", ctx, parentExecID).Return(nil, assert.AnError)

		// Act
		err := processParentTask(ctx, mockRepo, parentState, taskConfig, task.TaskTypeParallel)

		// Assert
		require.Error(t, err)
		assert.IsType(t, &core.Error{}, err)
		coreErr := err.(*core.Error)
		assert.Equal(t, "PROGRESS_INFO_FETCH_FAILED", coreErr.Code)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should retry when no children have reached terminal state", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:         "parent-task",
			TaskExecID:     parentExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeParallel,
			},
		}

		progressInfo := &task.ProgressInfo{
			TotalChildren: 3,
			SuccessCount:  0,
			FailedCount:   0,
			TerminalCount: 0, // No children in terminal state yet
			RunningCount:  3,
			StatusCounts: map[core.StatusType]int{
				core.StatusRunning: 3,
			},
		}

		// Mock expectations
		mockRepo.On("GetProgressInfo", ctx, parentExecID).Return(progressInfo, nil)

		// Act
		err := processParentTask(ctx, mockRepo, parentState, taskConfig, task.TaskTypeParallel)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "progress not yet visible")
		assert.Contains(t, err.Error(), "retrying")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle failed parent task with detailed error information", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:         "parent-task",
			TaskExecID:     parentExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeParallel,
			},
		}

		progressInfo := &task.ProgressInfo{
			TotalChildren: 3,
			SuccessCount:  1,
			FailedCount:   2,
			TerminalCount: 3,
			StatusCounts: map[core.StatusType]int{
				core.StatusSuccess: 1,
				core.StatusFailed:  2,
			},
		}

		failedChildren := []*task.State{
			{
				TaskID: "child2",
				Status: core.StatusFailed,
				Error:  &core.Error{Message: "Connection timeout"},
			},
			{
				TaskID: "child3",
				Status: core.StatusFailed,
				Error:  &core.Error{Code: "VALIDATION_ERROR"},
			},
		}

		// Mock expectations
		mockRepo.On("GetProgressInfo", ctx, parentExecID).Return(progressInfo, nil)
		mockRepo.On("ListChildrenOutputs", ctx, parentExecID).Return(map[string]*core.Output{
			"child1": {"result": "success"},
		}, nil)
		mockRepo.On("ListChildren", ctx, parentExecID).Return(failedChildren, nil)

		// Act
		err := processParentTask(ctx, mockRepo, parentState, taskConfig, task.TaskTypeParallel)

		// Assert
		require.Error(t, err)
		assert.Equal(t, core.StatusFailed, parentState.Status)
		assert.Contains(t, err.Error(), "parallel task failed")
		assert.Contains(t, err.Error(), "task[child2]: Connection timeout")
		assert.Contains(t, err.Error(), "task[child3]: VALIDATION_ERROR")
		mockRepo.AssertExpectations(t)
	})
}

func TestAggregateChildOutputs(t *testing.T) {
	log := logger.NewForTests()
	ctx := logger.ContextWithLogger(context.Background(), log)

	t.Run("Should aggregate all child outputs successfully", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: parentExecID,
			Output:     &core.Output{},
		}

		progressInfo := &task.ProgressInfo{
			SuccessCount: 3,
		}

		childOutputs := map[string]*core.Output{
			"child1": {"value": 1},
			"child2": {"value": 2},
			"child3": {"value": 3},
		}

		// Mock expectations
		mockRepo.On("ListChildrenOutputs", ctx, parentExecID).Return(childOutputs, nil)

		// Act
		err := aggregateChildOutputs(ctx, mockRepo, parentState, progressInfo, task.TaskTypeCollection)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, *parentState.Output, "outputs")
		outputs := (*parentState.Output)["outputs"].(map[string]any)
		assert.Len(t, outputs, 3)
		// outputs is a map[string]any where values are core.Output (not *core.Output)
		assert.Equal(t, core.Output{"value": 1}, outputs["child1"])
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should retry when not all successful children have outputs yet", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: parentExecID,
			Output:     &core.Output{},
		}

		progressInfo := &task.ProgressInfo{
			SuccessCount:  3, // 3 successful children
			TerminalCount: 3,
		}

		// Only 2 outputs available (race condition)
		childOutputs := map[string]*core.Output{
			"child1": {"value": 1},
			"child2": {"value": 2},
		}

		// Mock expectations
		mockRepo.On("ListChildrenOutputs", ctx, parentExecID).Return(childOutputs, nil)

		// Act
		err := aggregateChildOutputs(ctx, mockRepo, parentState, progressInfo, task.TaskTypeParallel)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child outputs not yet visible")
		assert.Contains(t, err.Error(), "have 2 outputs but 3 successful tasks")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle nil outputs from children", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: parentExecID,
			Output:     &core.Output{},
		}

		progressInfo := &task.ProgressInfo{
			SuccessCount: 2,
		}

		childOutputs := map[string]*core.Output{
			"child1": {"value": 1},
			"child2": nil, // Nil output
			"child3": {"value": 3},
		}

		// Mock expectations
		mockRepo.On("ListChildrenOutputs", ctx, parentExecID).Return(childOutputs, nil)

		// Act
		err := aggregateChildOutputs(ctx, mockRepo, parentState, progressInfo, task.TaskTypeCollection)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, *parentState.Output, "outputs")
		outputs := (*parentState.Output)["outputs"].(map[string]any)
		assert.Len(t, outputs, 2) // Only non-nil outputs
		assert.Contains(t, outputs, "child1")
		assert.Contains(t, outputs, "child3")
		assert.NotContains(t, outputs, "child2")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle error when listing child outputs", func(t *testing.T) {
		// Arrange
		mockRepo := new(mockTaskRepository)
		parentExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: parentExecID,
			Output:     &core.Output{},
		}

		progressInfo := &task.ProgressInfo{
			SuccessCount: 2,
		}

		// Mock expectations
		mockRepo.On("ListChildrenOutputs", ctx, parentExecID).Return(nil, assert.AnError)

		// Act
		err := aggregateChildOutputs(ctx, mockRepo, parentState, progressInfo, task.TaskTypeParallel)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to aggregate child outputs")
		mockRepo.AssertExpectations(t)
	})
}
