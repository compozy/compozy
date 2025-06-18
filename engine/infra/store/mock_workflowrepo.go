package store

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/mock"
)

// MockWorkflowRepo is a testify mock implementation for testing
type MockWorkflowRepo struct {
	mock.Mock
}

func (m *MockWorkflowRepo) ListStates(ctx context.Context, filter *workflow.StateFilter) ([]*workflow.State, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*workflow.State), args.Error(1)
}

func (m *MockWorkflowRepo) UpsertState(ctx context.Context, state *workflow.State) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockWorkflowRepo) UpdateStatus(ctx context.Context, id string, status core.StatusType) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockWorkflowRepo) GetState(ctx context.Context, id core.ID) (*workflow.State, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.State), args.Error(1)
}

func (m *MockWorkflowRepo) GetStateByID(ctx context.Context, id string) (*workflow.State, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.State), args.Error(1)
}

func (m *MockWorkflowRepo) GetStateByTaskID(
	ctx context.Context,
	workflowID string,
	taskID string,
) (*workflow.State, error) {
	args := m.Called(ctx, workflowID, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.State), args.Error(1)
}

func (m *MockWorkflowRepo) GetStateByAgentID(
	ctx context.Context,
	workflowID string,
	agentID string,
) (*workflow.State, error) {
	args := m.Called(ctx, workflowID, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.State), args.Error(1)
}

func (m *MockWorkflowRepo) GetStateByToolID(
	ctx context.Context,
	workflowID string,
	toolID string,
) (*workflow.State, error) {
	args := m.Called(ctx, workflowID, toolID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.State), args.Error(1)
}

func (m *MockWorkflowRepo) CompleteWorkflow(
	ctx context.Context,
	id core.ID,
	transformer workflow.OutputTransformer,
) (*workflow.State, error) {
	args := m.Called(ctx, id, transformer)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.State), args.Error(1)
}
