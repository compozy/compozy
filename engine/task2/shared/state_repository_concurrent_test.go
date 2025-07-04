package shared

import (
	"context"
	"sync"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTaskRepository is a mock implementation of task.Repository
type MockTaskRepository struct {
	mock.Mock
	mu     sync.RWMutex
	states map[string]*task.State
}

func NewMockTaskRepository() *MockTaskRepository {
	return &MockTaskRepository{
		states: make(map[string]*task.State),
	}
}

func (m *MockTaskRepository) AddState(state *task.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[state.TaskExecID.String()] = state
}

func (m *MockTaskRepository) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	args := m.Called(ctx, taskExecID)
	// Check if we have a stored state
	if state, ok := m.states[taskExecID.String()]; ok {
		return state, nil
	}
	return args.Get(0).(*task.State), args.Error(1)
}

func (m *MockTaskRepository) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepository) UpsertState(ctx context.Context, state *task.State) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockTaskRepository) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockTaskRepository) GetStateForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	taskExecID core.ID,
) (*task.State, error) {
	args := m.Called(ctx, tx, taskExecID)
	return args.Get(0).(*task.State), args.Error(1)
}

func (m *MockTaskRepository) UpsertStateWithTx(ctx context.Context, tx pgx.Tx, state *task.State) error {
	args := m.Called(ctx, tx, state)
	return args.Error(0)
}

func (m *MockTaskRepository) ListTasksInWorkflow(
	ctx context.Context,
	workflowExecID core.ID,
) (map[string]*task.State, error) {
	args := m.Called(ctx, workflowExecID)
	return args.Get(0).(map[string]*task.State), args.Error(1)
}

func (m *MockTaskRepository) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	args := m.Called(ctx, workflowExecID, status)
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepository) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	args := m.Called(ctx, workflowExecID, agentID)
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepository) ListTasksByTool(
	ctx context.Context,
	workflowExecID core.ID,
	toolID string,
) ([]*task.State, error) {
	args := m.Called(ctx, workflowExecID, toolID)
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepository) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	args := m.Called(ctx, parentStateID)
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepository) GetChildByTaskID(
	ctx context.Context,
	parentStateID core.ID,
	taskID string,
) (*task.State, error) {
	args := m.Called(ctx, parentStateID, taskID)
	return args.Get(0).(*task.State), args.Error(1)
}

func (m *MockTaskRepository) CreateChildStatesInTransaction(
	ctx context.Context,
	parentStateID core.ID,
	childStates []*task.State,
) error {
	args := m.Called(ctx, parentStateID, childStates)
	return args.Error(0)
}

func (m *MockTaskRepository) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	args := m.Called(ctx, rootStateID)
	return args.Get(0).([]*task.State), args.Error(1)
}

func (m *MockTaskRepository) ListChildrenOutputs(
	ctx context.Context,
	parentStateID core.ID,
) (map[string]*core.Output, error) {
	args := m.Called(ctx, parentStateID)
	return args.Get(0).(map[string]*core.Output), args.Error(1)
}

func (m *MockTaskRepository) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	args := m.Called(ctx, parentStateID)
	return args.Get(0).(*task.ProgressInfo), args.Error(1)
}

func TestDefaultStateRepository_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent reads safely", func(t *testing.T) {
		// This test should be run with -race flag to detect race conditions
		// go test -race -run TestDefaultStateRepository_ConcurrentAccess
		mockRepo := NewMockTaskRepository()
		mockRepo.On("GetState", mock.Anything, mock.Anything).Return((*task.State)(nil), nil)
		repo := NewStateRepository(mockRepo)

		ctx := context.Background()
		parentID, _ := core.NewID()
		parentState := &task.State{
			TaskExecID: parentID,
			Status:     core.StatusSuccess,
		}

		// Add state to stub repository
		mockRepo.AddState(parentState)

		// Launch multiple goroutines to read concurrently
		var wg sync.WaitGroup
		numReaders := 10
		wg.Add(numReaders)

		for i := 0; i < numReaders; i++ {
			go func() {
				defer wg.Done()
				state, err := repo.GetParentState(ctx, parentID)
				assert.NoError(t, err)
				assert.NotNil(t, state)
				assert.Equal(t, parentID, state.TaskExecID)
			}()
		}

		wg.Wait()
	})

	t.Run("Should handle concurrent writes safely", func(t *testing.T) {
		// This test verifies that cache writes are protected by mutex
		mockRepo := NewMockTaskRepository()
		mockRepo.On("GetState", mock.Anything, mock.Anything).Return((*task.State)(nil), nil)
		repo := NewStateRepository(mockRepo).(*DefaultStateRepository)

		ctx := context.Background()
		numWriters := 10
		var wg sync.WaitGroup
		wg.Add(numWriters)

		// Create states for concurrent access
		states := make([]*task.State, numWriters)
		for i := 0; i < numWriters; i++ {
			id, _ := core.NewID()
			states[i] = &task.State{
				TaskExecID: id,
				TaskID:     "task-" + id.String(),
				Status:     core.StatusSuccess,
			}
			mockRepo.AddState(states[i])
		}

		// Launch multiple goroutines to write to cache concurrently
		for i := 0; i < numWriters; i++ {
			go func(index int) {
				defer wg.Done()
				_, err := repo.GetParentState(ctx, states[index].TaskExecID)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
	})

	t.Run("Should handle mixed read/write operations safely", func(t *testing.T) {
		mockRepo := NewMockTaskRepository()
		mockRepo.On("GetState", mock.Anything, mock.Anything).Return((*task.State)(nil), nil)
		repo := NewStateRepository(mockRepo).(*DefaultStateRepository)

		ctx := context.Background()
		sharedID, _ := core.NewID()
		sharedState := &task.State{
			TaskExecID: sharedID,
			Status:     core.StatusSuccess,
		}

		// Add to stub repository
		mockRepo.AddState(sharedState)

		var wg sync.WaitGroup
		numOperations := 20

		// Mix of readers and writers
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			// All operations are effectively the same - GetParentState reads from cache if present,
			// or fetches from repository and caches the result
			go func() {
				defer wg.Done()
				state, err := repo.GetParentState(ctx, sharedID)
				assert.NoError(t, err)
				assert.NotNil(t, state)
			}()
		}

		wg.Wait()
	})

	t.Run("Should not have race conditions with validateID", func(t *testing.T) {
		mockRepo := NewMockTaskRepository()
		mockRepo.On("GetState", mock.Anything, mock.Anything).Return((*task.State)(nil), nil)
		repo := NewStateRepository(mockRepo).(*DefaultStateRepository)

		ctx := context.Background()
		var wg sync.WaitGroup
		numValidations := 10

		wg.Add(numValidations)
		for i := 0; i < numValidations; i++ {
			go func() {
				defer wg.Done()
				// Test with invalid IDs that should fail validation
				_, err := repo.GetParentState(ctx, "")
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid parent task reference")
			}()
		}

		wg.Wait()
	})

	t.Run("Should handle cache size correctly under concurrent access", func(t *testing.T) {
		mockRepo := NewMockTaskRepository()
		mockRepo.On("GetState", mock.Anything, mock.Anything).Return((*task.State)(nil), nil)
		repo := NewStateRepository(mockRepo).(*DefaultStateRepository)

		ctx := context.Background()
		numStates := 5
		states := make([]*task.State, numStates)

		// Create states and add to stub
		for i := 0; i < numStates; i++ {
			id, _ := core.NewID()
			states[i] = &task.State{
				TaskExecID: id,
				TaskID:     "task-" + id.String(),
				Status:     core.StatusSuccess,
			}
			mockRepo.AddState(states[i])
		}

		var wg sync.WaitGroup
		wg.Add(numStates)

		// Concurrently add all states to cache
		for i := 0; i < numStates; i++ {
			go func(index int) {
				defer wg.Done()
				_, err := repo.GetParentState(ctx, states[index].TaskExecID)
				require.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Verify cache size (need to lock for safe access)
		repo.cacheMutex.RLock()
		cacheSize := len(repo.parentCache)
		repo.cacheMutex.RUnlock()

		assert.Equal(t, numStates, cacheSize)
	})
}
