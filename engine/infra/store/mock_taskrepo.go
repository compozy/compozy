package store

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
)

// MockTaskRepo is a testify mock implementation for testing
type MockTaskRepo struct {
	mock.Mock
}

func (m *MockTaskRepo) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepo) UpsertState(ctx context.Context, state *task.State) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockTaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	args := m.Called(ctx, taskExecID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.State), args.Error(1)
}

func (m *MockTaskRepo) ListTasksInWorkflow(ctx context.Context, workflowID core.ID) (map[string]*task.State, error) {
	args := m.Called(ctx, workflowID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*task.State), args.Error(1)
}

func (m *MockTaskRepo) ListTasksByStatus(
	ctx context.Context,
	workflowID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	args := m.Called(ctx, workflowID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepo) ListTasksByAgent(
	ctx context.Context,
	workflowID core.ID,
	agentID string,
) ([]*task.State, error) {
	args := m.Called(ctx, workflowID, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepo) ListTasksByTool(ctx context.Context, workflowID core.ID, toolID string) ([]*task.State, error) {
	args := m.Called(ctx, workflowID, toolID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepo) ListChildren(ctx context.Context, parentID core.ID) ([]*task.State, error) {
	args := m.Called(ctx, parentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepo) CreateChildStatesInTransaction(
	ctx context.Context,
	parentID core.ID,
	states []*task.State,
) error {
	args := m.Called(ctx, parentID, states)
	return args.Error(0)
}

func (m *MockTaskRepo) GetTaskTree(ctx context.Context, rootID core.ID) ([]*task.State, error) {
	args := m.Called(ctx, rootID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepo) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockTaskRepo) GetStateForUpdate(ctx context.Context, tx pgx.Tx, taskExecID core.ID) (*task.State, error) {
	args := m.Called(ctx, tx, taskExecID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.State), args.Error(1)
}

func (m *MockTaskRepo) UpsertStateWithTx(ctx context.Context, tx pgx.Tx, state *task.State) error {
	args := m.Called(ctx, tx, state)
	return args.Error(0)
}

func (m *MockTaskRepo) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	args := m.Called(ctx, parentStateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.ProgressInfo), args.Error(1)
}

func (m *MockTaskRepo) GetChildByTaskID(
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
