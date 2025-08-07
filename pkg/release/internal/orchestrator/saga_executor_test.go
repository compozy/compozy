package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Define error for testing
var errStateNotFound = fmt.Errorf("state not found")

// MockStateRepository is a mock implementation of StateRepository
type MockStateRepository struct {
	mock.Mock
}

func (m *MockStateRepository) Save(ctx context.Context, state *domain.RollbackState) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockStateRepository) Load(ctx context.Context, sessionID string) (*domain.RollbackState, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RollbackState), args.Error(1)
}

func (m *MockStateRepository) LoadLatest(ctx context.Context) (*domain.RollbackState, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RollbackState), args.Error(1)
}

func (m *MockStateRepository) Delete(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockStateRepository) Exists(ctx context.Context, sessionID string) (bool, error) {
	args := m.Called(ctx, sessionID)
	return args.Bool(0), args.Error(1)
}

func TestSagaExecutor_Execute(t *testing.T) {
	t.Run("Should execute all steps successfully", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, false) // Persistence disabled

		// No Save calls expected when persistence is disabled

		step1Executed := false
		step2Executed := false

		saga.AddStep(SagaStep{
			Name: "Step 1",
			Type: domain.OperationTypeCheckChanges,
			Execute: func(_ context.Context) (map[string]any, error) {
				step1Executed = true
				return map[string]any{"result": "step1"}, nil
			},
			Compensate: func(_ context.Context, _ map[string]any) error {
				return nil
			},
		})

		saga.AddStep(SagaStep{
			Name: "Step 2",
			Type: domain.OperationTypeCalculateVersion,
			Execute: func(_ context.Context) (map[string]any, error) {
				step2Executed = true
				return map[string]any{"result": "step2"}, nil
			},
			Compensate: func(_ context.Context, _ map[string]any) error {
				return nil
			},
		})

		// Act
		err := saga.Execute(context.Background())

		// Assert
		assert.NoError(t, err)
		assert.True(t, step1Executed)
		assert.True(t, step2Executed)
		assert.Equal(t, domain.WorkflowStatusCompleted, saga.GetState().Status)
		// No expectations to assert when persistence is disabled
	})

	t.Run("Should rollback on failure", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, true) // Enable rollback

		// When enableRollback is true, Save is called multiple times
		mockRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Maybe()

		step1Compensated := false
		step2Compensated := false

		saga.AddStep(SagaStep{
			Name: "Step 1",
			Type: domain.OperationTypeCheckChanges,
			Execute: func(_ context.Context) (map[string]any, error) {
				return map[string]any{"result": "step1"}, nil
			},
			Compensate: func(_ context.Context, _ map[string]any) error {
				step1Compensated = true
				return nil
			},
		})

		saga.AddStep(SagaStep{
			Name: "Step 2",
			Type: domain.OperationTypeCalculateVersion,
			Execute: func(_ context.Context) (map[string]any, error) {
				return nil, errors.New("step 2 failed")
			},
			Compensate: func(_ context.Context, _ map[string]any) error {
				step2Compensated = true
				return nil
			},
		})

		// Act
		err := saga.Execute(context.Background())

		// Assert
		assert.Error(t, err)
		assert.True(t, step1Compensated)
		assert.False(t, step2Compensated) // Step 2 never succeeded
		assert.Equal(t, domain.WorkflowStatusRolledBack, saga.GetState().Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle compensate errors", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, true) // Enable rollback

		// When enableRollback is true, Save is called multiple times
		mockRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Maybe()

		saga.AddStep(SagaStep{
			Name: "Step 1",
			Type: domain.OperationTypeCheckChanges,
			Execute: func(_ context.Context) (map[string]any, error) {
				return map[string]any{"result": "step1"}, nil
			},
			Compensate: func(_ context.Context, _ map[string]any) error {
				return errors.New("compensate failed")
			},
		})

		saga.AddStep(SagaStep{
			Name: "Step 2",
			Type: domain.OperationTypeCalculateVersion,
			Execute: func(_ context.Context) (map[string]any, error) {
				return nil, errors.New("step 2 failed")
			},
			Compensate: func(_ context.Context, _ map[string]any) error {
				return nil
			},
		})

		// Act
		err := saga.Execute(context.Background())

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "step 2 failed")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should persist state when enabled", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, true) // Enable persistence

		// With persistence enabled: 1 initial + (2 per step * 1 step) + 1 final = 4 saves
		mockRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Times(4)

		saga.AddStep(SagaStep{
			Name: "Step 1",
			Type: domain.OperationTypeCheckChanges,
			Execute: func(_ context.Context) (map[string]any, error) {
				return map[string]any{"result": "step1"}, nil
			},
			Compensate: func(_ context.Context, _ map[string]any) error {
				return nil
			},
		})

		// Act
		err := saga.Execute(context.Background())

		// Assert
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestSagaExecutor_Rollback(t *testing.T) {
	t.Run("Should rollback completed steps in reverse order", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, false) // Persistence disabled

		// No Save calls expected when persistence is disabled

		rollbackOrder := []string{}

		// Set up initial state
		saga.state.Operations = []domain.OperationRecord{
			{
				Type:         domain.OperationTypeCheckChanges,
				Status:       domain.OperationStatusCompleted,
				RollbackData: map[string]any{"step": "1"},
			},
			{
				Type:         domain.OperationTypeCalculateVersion,
				Status:       domain.OperationStatusCompleted,
				RollbackData: map[string]any{"step": "2"},
			},
			{
				Type:         domain.OperationTypeCreateBranch,
				Status:       domain.OperationStatusCompleted,
				RollbackData: map[string]any{"step": "3"},
			},
		}

		// Add compensating steps
		saga.AddStep(SagaStep{
			Name: "Step 1",
			Type: domain.OperationTypeCheckChanges,
			Compensate: func(_ context.Context, _ map[string]any) error {
				rollbackOrder = append(rollbackOrder, "step1")
				return nil
			},
		})

		saga.AddStep(SagaStep{
			Name: "Step 2",
			Type: domain.OperationTypeCalculateVersion,
			Compensate: func(_ context.Context, _ map[string]any) error {
				rollbackOrder = append(rollbackOrder, "step2")
				return nil
			},
		})

		saga.AddStep(SagaStep{
			Name: "Step 3",
			Type: domain.OperationTypeCreateBranch,
			Compensate: func(_ context.Context, _ map[string]any) error {
				rollbackOrder = append(rollbackOrder, "step3")
				return nil
			},
		})

		// Act
		err := saga.Rollback(context.Background())

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []string{"step3", "step2", "step1"}, rollbackOrder)
		assert.Equal(t, domain.WorkflowStatusRolledBack, saga.GetState().Status)
		// No expectations to assert when persistence is disabled
	})

	t.Run("Should skip failed and pending operations", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, false) // Persistence disabled

		// No Save calls expected when persistence is disabled

		rollbackCalled := false

		// Set up initial state with mixed statuses
		saga.state.Operations = []domain.OperationRecord{
			{
				Type:         domain.OperationTypeCheckChanges,
				Status:       domain.OperationStatusCompleted,
				RollbackData: map[string]any{"step": "1"},
			},
			{
				Type:   domain.OperationTypeCalculateVersion,
				Status: domain.OperationStatusFailed,
			},
			{
				Type:   domain.OperationTypeCreateBranch,
				Status: domain.OperationStatusPending,
			},
		}

		// Add compensating step for completed operation
		saga.AddStep(SagaStep{
			Name: "Step 1",
			Type: domain.OperationTypeCheckChanges,
			Compensate: func(_ context.Context, _ map[string]any) error {
				rollbackCalled = true
				return nil
			},
		})

		// Act
		err := saga.Rollback(context.Background())

		// Assert
		assert.NoError(t, err)
		assert.True(t, rollbackCalled)
		// No expectations to assert when persistence is disabled
	})
}

func TestLoadExistingSaga(t *testing.T) {
	t.Run("Should load existing saga from repository", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		sessionID := "test-session-123"
		expectedState := &domain.RollbackState{
			SessionID:      sessionID,
			Version:        "v1.2.3",
			BranchName:     "release/v1.2.3",
			OriginalBranch: "main",
			Status:         domain.WorkflowStatusFailed,
			Operations: []domain.OperationRecord{
				{
					Type:   domain.OperationTypeCheckChanges,
					Status: domain.OperationStatusCompleted,
				},
			},
		}

		mockRepo.On("Load", mock.Anything, sessionID).Return(expectedState, nil)

		// Act
		saga, err := LoadExistingSaga(mockRepo, sessionID)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, saga)
		assert.Equal(t, sessionID, saga.GetState().SessionID)
		assert.Equal(t, "v1.2.3", saga.GetState().Version)
		assert.Equal(t, "release/v1.2.3", saga.GetState().BranchName)
		assert.Equal(t, "main", saga.GetState().OriginalBranch)
		assert.Equal(t, domain.WorkflowStatusFailed, saga.GetState().Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return error when saga not found", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		sessionID := "non-existent"

		mockRepo.On("Load", mock.Anything, sessionID).Return(nil, errStateNotFound)

		// Act
		saga, err := LoadExistingSaga(mockRepo, sessionID)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, saga)
		assert.Contains(t, err.Error(), "failed to load saga state")
		mockRepo.AssertExpectations(t)
	})
}

func TestSagaExecutor_SettersAndGetters(t *testing.T) {
	t.Run("Should set and get version", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, false)

		// Act
		saga.SetVersion("v1.2.3")

		// Assert
		assert.Equal(t, "v1.2.3", saga.GetState().Version)
	})

	t.Run("Should set and get branch name", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, false)

		// Act
		saga.SetBranchName("release/v1.2.3")

		// Assert
		assert.Equal(t, "release/v1.2.3", saga.GetState().BranchName)
	})

	t.Run("Should set and get original branch", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockStateRepository)
		saga := NewSagaExecutor(mockRepo, false)

		// Act
		saga.SetOriginalBranch("main")

		// Assert
		assert.Equal(t, "main", saga.GetState().OriginalBranch)
	})
}
