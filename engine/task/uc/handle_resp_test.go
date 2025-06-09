package uc

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleResponse_ShouldUpdateParentStatus(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus core.StatusType
		newStatus     core.StatusType
		expected      bool
	}{
		{
			name:          "same status should not update",
			currentStatus: core.StatusRunning,
			newStatus:     core.StatusRunning,
			expected:      false,
		},
		{
			name:          "transition to success should update",
			currentStatus: core.StatusRunning,
			newStatus:     core.StatusSuccess,
			expected:      true,
		},
		{
			name:          "transition to failed should update",
			currentStatus: core.StatusRunning,
			newStatus:     core.StatusFailed,
			expected:      true,
		},
		{
			name:          "from pending to running should update",
			currentStatus: core.StatusPending,
			newStatus:     core.StatusRunning,
			expected:      true,
		},
		{
			name:          "from success to failed should update (terminal to terminal)",
			currentStatus: core.StatusSuccess,
			newStatus:     core.StatusFailed,
			expected:      true,
		},
		{
			name:          "from success to running should not update (terminal to active)",
			currentStatus: core.StatusSuccess,
			newStatus:     core.StatusRunning,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repositories - we don't need them for this pure function test
			mockWorkflowRepo := &mockWorkflowRepo{}
			mockTaskRepo := &mockTaskRepo{}

			handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)

			result := handleResponse.parentStatusUpdater.ShouldUpdateParentStatus(tt.currentStatus, tt.newStatus)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleResponse_UpdateParentStatusIfNeeded(t *testing.T) {
	tests := []struct {
		name           string
		childState     *task.State
		parentState    *task.State
		progressInfo   *task.ProgressInfo
		expectedStatus core.StatusType
		expectUpdate   bool
	}{
		{
			name: "child task with no parent should not update anything",
			childState: &task.State{
				TaskExecID:    core.ID("child-123"),
				TaskID:        "child-task",
				Status:        core.StatusSuccess,
				ParentStateID: nil,
			},
			expectUpdate: false,
		},
		{
			name: "non-parallel parent task should not update",
			childState: &task.State{
				TaskExecID:    core.ID("child-123"),
				TaskID:        "child-task",
				Status:        core.StatusSuccess,
				ParentStateID: &[]core.ID{core.ID("parent-456")}[0],
			},
			parentState: &task.State{
				TaskExecID:    core.ID("parent-456"),
				TaskID:        "parent-task",
				Status:        core.StatusRunning,
				ExecutionType: task.ExecutionBasic,
			},
			expectUpdate: false,
		},
		{
			name: "wait_all strategy with all children complete should succeed",
			childState: &task.State{
				TaskExecID:    core.ID("child-123"),
				TaskID:        "child-task",
				Status:        core.StatusSuccess,
				ParentStateID: &[]core.ID{core.ID("parent-456")}[0],
			},
			parentState: &task.State{
				TaskExecID:    core.ID("parent-456"),
				TaskID:        "parent-task",
				Status:        core.StatusRunning,
				ExecutionType: task.ExecutionParallel,
				Input: &core.Input{
					"_parallel_config": map[string]any{
						"strategy": string(task.StrategyWaitAll),
					},
				},
			},
			progressInfo: &task.ProgressInfo{
				TotalChildren:  2,
				CompletedCount: 2,
				FailedCount:    0,
				RunningCount:   0,
				PendingCount:   0,
			},
			expectedStatus: core.StatusSuccess,
			expectUpdate:   true,
		},
		{
			name: "fail_fast strategy with one failure should fail",
			childState: &task.State{
				TaskExecID:    core.ID("child-123"),
				TaskID:        "child-task",
				Status:        core.StatusFailed,
				ParentStateID: &[]core.ID{core.ID("parent-456")}[0],
			},
			parentState: &task.State{
				TaskExecID:    core.ID("parent-456"),
				TaskID:        "parent-task",
				Status:        core.StatusRunning,
				ExecutionType: task.ExecutionParallel,
				Input: &core.Input{
					"_parallel_config": map[string]any{
						"strategy": string(task.StrategyFailFast),
					},
				},
			},
			progressInfo: &task.ProgressInfo{
				TotalChildren:  2,
				CompletedCount: 0,
				FailedCount:    1,
				RunningCount:   1,
				PendingCount:   0,
			},
			expectedStatus: core.StatusFailed,
			expectUpdate:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorkflowRepo := &mockWorkflowRepo{}
			mockTaskRepo := &mockTaskRepo{}

			// Set up expectations based on test case
			if tt.childState.ParentStateID != nil {
				mockTaskRepo.getStateFunc = func(_ context.Context, taskExecID core.ID) (*task.State, error) {
					if taskExecID == *tt.childState.ParentStateID {
						return tt.parentState, nil
					}
					return nil, nil
				}

				if tt.expectUpdate && tt.parentState.ExecutionType == task.ExecutionParallel {
					mockTaskRepo.getProgressInfoFunc = func(_ context.Context, _ core.ID) (*task.ProgressInfo, error) {
						return tt.progressInfo, nil
					}

					mockTaskRepo.upsertStateFunc = func(_ context.Context, state *task.State) error {
						if state.TaskExecID == tt.parentState.TaskExecID {
							require.Equal(t, tt.expectedStatus, state.Status)
						}
						return nil
					}
				}
			}

			handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)
			ctx := context.Background()

			err := handleResponse.updateParentStatusIfNeeded(ctx, tt.childState)
			assert.NoError(t, err)
		})
	}
}

// mockTaskRepo is a simple mock implementation for testing
type mockTaskRepo struct {
	getStateFunc        func(ctx context.Context, taskExecID core.ID) (*task.State, error)
	upsertStateFunc     func(ctx context.Context, state *task.State) error
	getProgressInfoFunc func(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error)
}

func (m *mockTaskRepo) ListStates(_ context.Context, _ *task.StateFilter) ([]*task.State, error) {
	return nil, nil
}

func (m *mockTaskRepo) UpsertState(ctx context.Context, state *task.State) error {
	if m.upsertStateFunc != nil {
		return m.upsertStateFunc(ctx, state)
	}
	return nil
}

func (m *mockTaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	if m.getStateFunc != nil {
		return m.getStateFunc(ctx, taskExecID)
	}
	return nil, nil
}

func (m *mockTaskRepo) ListTasksInWorkflow(
	_ context.Context,
	_ core.ID,
) (map[string]*task.State, error) {
	return nil, nil
}

func (m *mockTaskRepo) ListTasksByStatus(
	_ context.Context,
	_ core.ID,
	_ core.StatusType,
) ([]*task.State, error) {
	return nil, nil
}

func (m *mockTaskRepo) ListTasksByAgent(
	_ context.Context,
	_ core.ID,
	_ string,
) ([]*task.State, error) {
	return nil, nil
}

func (m *mockTaskRepo) ListTasksByTool(
	_ context.Context,
	_ core.ID,
	_ string,
) ([]*task.State, error) {
	return nil, nil
}

func (m *mockTaskRepo) ListChildren(_ context.Context, _ core.ID) ([]*task.State, error) {
	return nil, nil
}

func (m *mockTaskRepo) CreateChildStatesInTransaction(
	_ context.Context,
	_ core.ID,
	_ []*task.State,
) error {
	return nil
}

func (m *mockTaskRepo) GetTaskTree(_ context.Context, _ core.ID) ([]*task.State, error) {
	return nil, nil
}

func (m *mockTaskRepo) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	if m.getProgressInfoFunc != nil {
		return m.getProgressInfoFunc(ctx, parentStateID)
	}
	return nil, nil
}

// mockWorkflowRepo is a simple mock implementation for testing
type mockWorkflowRepo struct{}

func (m *mockWorkflowRepo) ListStates(_ context.Context, _ *workflow.StateFilter) ([]*workflow.State, error) {
	return nil, nil
}

func (m *mockWorkflowRepo) UpsertState(_ context.Context, _ *workflow.State) error {
	return nil
}

func (m *mockWorkflowRepo) UpdateStatus(_ context.Context, _ string, _ core.StatusType) error {
	return nil
}

func (m *mockWorkflowRepo) GetState(_ context.Context, _ core.ID) (*workflow.State, error) {
	return nil, nil
}

func (m *mockWorkflowRepo) GetStateByID(_ context.Context, _ string) (*workflow.State, error) {
	return nil, nil
}

func (m *mockWorkflowRepo) GetStateByTaskID(_ context.Context, _ string, _ string) (*workflow.State, error) {
	return nil, nil
}

func (m *mockWorkflowRepo) GetStateByAgentID(_ context.Context, _ string, _ string) (*workflow.State, error) {
	return nil, nil
}

func (m *mockWorkflowRepo) GetStateByToolID(_ context.Context, _ string, _ string) (*workflow.State, error) {
	return nil, nil
}

func (m *mockWorkflowRepo) CompleteWorkflow(_ context.Context, _ core.ID) (*workflow.State, error) {
	return nil, nil
}
