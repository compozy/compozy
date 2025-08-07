package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/google/uuid"
	"github.com/sethvargo/go-retry"
)

// SagaStep represents a single step in the saga workflow
type SagaStep struct {
	Name       string
	Type       domain.OperationType
	Execute    func(ctx context.Context) (rollbackData map[string]any, err error)
	Compensate func(ctx context.Context, rollbackData map[string]any) error
}

// SagaExecutor manages the execution of saga workflows with rollback support
type SagaExecutor struct {
	sessionID      string
	stateRepo      repository.StateRepository
	state          *domain.RollbackState
	steps          []SagaStep
	enableRollback bool
}

// NewSagaExecutor creates a new saga executor
func NewSagaExecutor(stateRepo repository.StateRepository, enableRollback bool) *SagaExecutor {
	sessionID := uuid.New().String()
	return &SagaExecutor{
		sessionID:      sessionID,
		stateRepo:      stateRepo,
		state:          domain.NewRollbackState(sessionID),
		steps:          []SagaStep{},
		enableRollback: enableRollback,
	}
}

// LoadExistingSaga loads an existing saga from state
func LoadExistingSaga(stateRepo repository.StateRepository, sessionID string) (*SagaExecutor, error) {
	state, err := stateRepo.Load(context.Background(), sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load saga state: %w", err)
	}
	return &SagaExecutor{
		sessionID:      sessionID,
		stateRepo:      stateRepo,
		state:          state,
		steps:          []SagaStep{},
		enableRollback: true,
	}, nil
}

// AddStep adds a step to the saga
func (s *SagaExecutor) AddStep(step SagaStep) {
	s.steps = append(s.steps, step)
	s.state.AddOperation(step.Type)
}

// Execute runs the saga workflow with automatic rollback on failure
func (s *SagaExecutor) Execute(ctx context.Context) error {
	if s.enableRollback {
		if err := s.saveState(ctx); err != nil {
			return fmt.Errorf("failed to save initial state: %w", err)
		}
	}
	s.state.Status = domain.WorkflowStatusRunning
	for _, step := range s.steps {
		if err := s.executeStep(ctx, step); err != nil {
			s.state.MarkOperationFailed(step.Type, err)
			if s.enableRollback {
				if saveErr := s.saveState(ctx); saveErr != nil {
					// Log but don't fail - best effort save
					fmt.Printf("Warning: failed to save state before rollback: %v\n", saveErr)
				}
				// Create separate context for rollback to ensure it completes
				rollbackCtx, cancel := context.WithTimeout(context.Background(), RollbackTimeout)
				rollbackErr := s.rollback(rollbackCtx)
				cancel() // Call cancel immediately after rollback
				if rollbackErr != nil {
					return fmt.Errorf("step '%s' failed: %w, rollback also failed: %v",
						step.Name, err, rollbackErr)
				}
			}
			return fmt.Errorf("step '%s' failed: %w", step.Name, err)
		}
	}
	s.state.Status = domain.WorkflowStatusCompleted
	if s.enableRollback {
		if saveErr := s.saveState(ctx); saveErr != nil {
			// Log but don't fail - best effort save at completion
			fmt.Printf("Warning: failed to save final state: %v\n", saveErr)
		}
	}
	return nil
}

// executeStep executes a single saga step with retry logic
func (s *SagaExecutor) executeStep(ctx context.Context, step SagaStep) error {
	s.state.MarkOperationStarted(step.Type)
	if s.enableRollback {
		if saveErr := s.saveState(ctx); saveErr != nil {
			// Log but don't fail - best effort save
			fmt.Printf("Warning: failed to save state after marking operation started: %v\n", saveErr)
		}
	}
	var rollbackData map[string]any
	var err error
	retryStrategy := retry.WithMaxRetries(DefaultRetryCount, retry.NewExponential(DefaultRetryDelay))
	err = retry.Do(ctx, retryStrategy, func(retryCtx context.Context) error {
		// Check if context is canceled before executing
		select {
		case <-retryCtx.Done():
			return retryCtx.Err()
		default:
		}
		rollbackData, err = step.Execute(retryCtx)
		if err != nil {
			return retry.RetryableError(err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.state.MarkOperationCompleted(step.Type, rollbackData)
	if s.enableRollback {
		if saveErr := s.saveState(ctx); saveErr != nil {
			// Log but don't fail - best effort save
			fmt.Printf("Warning: failed to save state after marking operation completed: %v\n", saveErr)
		}
	}
	return nil
}

// Rollback executes compensating actions for completed operations
func (s *SagaExecutor) Rollback(ctx context.Context) error {
	return s.rollback(ctx)
}

// rollback internal implementation
func (s *SagaExecutor) rollback(ctx context.Context) error {
	fmt.Println("🔄 Starting rollback process...")
	completedOps := s.state.GetCompletedOperations()
	if len(completedOps) == 0 {
		fmt.Println("No operations to rollback")
		return nil
	}
	for _, op := range completedOps {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("rollback canceled: %w", ctx.Err())
		default:
		}
		step := s.findStepByType(op.Type)
		if step == nil || step.Compensate == nil {
			continue
		}
		fmt.Printf("Rolling back: %s\n", step.Name)
		if err := s.executeCompensation(ctx, step, op.RollbackData); err != nil {
			fmt.Printf("Failed to rollback %s: %v\n", step.Name, err)
			return fmt.Errorf("rollback failed for %s: %w", step.Name, err)
		}
		op.Status = domain.OperationStatusRolledBack
		if s.enableRollback {
			if saveErr := s.saveState(ctx); saveErr != nil {
				// Log but don't fail - best effort save during rollback
				fmt.Printf("Warning: failed to save state during rollback: %v\n", saveErr)
			}
		}
	}
	s.state.Status = domain.WorkflowStatusRolledBack
	if s.enableRollback {
		if saveErr := s.saveState(ctx); saveErr != nil {
			// Log but don't fail - best effort save after rollback
			fmt.Printf("Warning: failed to save state after rollback: %v\n", saveErr)
		}
	}
	fmt.Println("✅ Rollback completed successfully")
	return nil
}

// executeCompensation executes a compensating action with retry
func (s *SagaExecutor) executeCompensation(ctx context.Context, step *SagaStep, rollbackData map[string]any) error {
	retryStrategy := retry.WithMaxRetries(DefaultRetryCount, retry.NewExponential(DefaultRetryDelay))
	return retry.Do(ctx, retryStrategy, func(retryCtx context.Context) error {
		// Check if context is canceled
		select {
		case <-retryCtx.Done():
			return retryCtx.Err()
		default:
		}
		if err := step.Compensate(retryCtx, rollbackData); err != nil {
			return retry.RetryableError(err)
		}
		return nil
	})
}

// findStepByType finds a saga step by operation type
func (s *SagaExecutor) findStepByType(opType domain.OperationType) *SagaStep {
	for i := range s.steps {
		if s.steps[i].Type == opType {
			return &s.steps[i]
		}
	}
	return nil
}

// saveState persists the current state
func (s *SagaExecutor) saveState(ctx context.Context) error {
	return s.stateRepo.Save(ctx, s.state)
}

// GetSessionID returns the saga session ID
func (s *SagaExecutor) GetSessionID() string {
	return s.sessionID
}

// GetState returns the current saga state
func (s *SagaExecutor) GetState() *domain.RollbackState {
	return s.state
}

// SetVersion sets the version in the state
func (s *SagaExecutor) SetVersion(version string) {
	s.state.Version = version
}

// SetBranchName sets the branch name in the state
func (s *SagaExecutor) SetBranchName(branchName string) {
	s.state.BranchName = branchName
}

// SetOriginalBranch sets the original branch in the state
func (s *SagaExecutor) SetOriginalBranch(branchName string) {
	s.state.OriginalBranch = branchName
}
