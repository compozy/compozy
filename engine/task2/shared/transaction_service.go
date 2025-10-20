package shared

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
)

// TransactionService handles all transaction-related operations for task state management
//
// This service implements the transactional interface pattern by:
// 1. Using type assertion to check if the repository supports transactions
// 2. Gracefully degrading to non-transactional operations when transactions aren't available
// 3. Providing consistent API regardless of underlying repository capabilities
//
// The pattern is implemented through the driver-neutral closure on task.Repository:
//
//	type Repository interface {
//	    WithTransaction(ctx context.Context, fn func(Repository) error) error
//	}
//
// Example usage:
//
//	service := NewTransactionService(taskRepo)
//	err := service.SaveStateWithLocking(ctx, state) // Uses transactions if available
//
// This approach allows the codebase to work with both transactional and non-transactional
// repositories without requiring different code paths at the call site.
type TransactionService struct {
	taskRepo task.Repository
}

// NewTransactionService creates a new transaction service
func NewTransactionService(taskRepo task.Repository) *TransactionService {
	return &TransactionService{
		taskRepo: taskRepo,
	}
}

// SaveStateWithLocking saves task state with transaction safety and row-level locking
func (s *TransactionService) SaveStateWithLocking(
	ctx context.Context,
	state *task.State,
) error {
	// Validate input to prevent nil pointer dereference
	if state == nil {
		return fmt.Errorf("task state cannot be nil")
	}
	// Try transaction-based save first
	if txRepo := s.getTransactionalRepo(); txRepo != nil {
		return s.saveWithTransaction(ctx, state, txRepo)
	}
	// Fallback to regular save
	return s.saveWithoutTransaction(ctx, state)
}

// ApplyTransformation applies a transformation function to task state within a transaction
func (s *TransactionService) ApplyTransformation(
	ctx context.Context,
	taskExecID core.ID,
	transformer StateTransformer,
) error {
	// Try transaction-based transformation first
	if txRepo := s.getTransactionalRepo(); txRepo != nil {
		return s.applyWithTransaction(ctx, taskExecID, transformer, txRepo)
	}
	// Fallback to direct transformation
	return s.applyWithoutTransaction(ctx, taskExecID, transformer)
}

// StateTransformer defines a function that transforms task state
type StateTransformer func(state *task.State) error

// transactionalRepo defines the interface for repositories supporting transactions
type transactionalRepo interface {
	WithTransaction(ctx context.Context, fn func(task.Repository) error) error
	GetStateForUpdate(ctx context.Context, taskExecID core.ID) (*task.State, error)
}

// getTransactionalRepo checks if the repository supports transactions
func (s *TransactionService) getTransactionalRepo() transactionalRepo {
	if txRepo, ok := s.taskRepo.(transactionalRepo); ok {
		return txRepo
	}
	return nil
}

// saveWithTransaction saves state using transaction with row-level locking
func (s *TransactionService) saveWithTransaction(
	ctx context.Context,
	state *task.State,
	txRepo transactionalRepo,
) error {
	return txRepo.WithTransaction(ctx, func(repo task.Repository) error {
		latestState, err := repo.GetStateForUpdate(ctx, state.TaskExecID)
		if err != nil {
			if errors.Is(err, store.ErrTaskNotFound) {
				if err := repo.UpsertState(ctx, state); err != nil {
					return fmt.Errorf("unable to save new task state: %w", err)
				}
				return nil
			}
			return fmt.Errorf("unable to acquire task lock for update: %w", err)
		}
		s.mergeStateChanges(latestState, state)
		state.Status = latestState.Status
		state.Output = latestState.Output
		state.Error = latestState.Error
		if state.Input == nil && latestState.Input != nil {
			state.Input = latestState.Input
		}
		if err := repo.UpsertState(ctx, latestState); err != nil {
			return fmt.Errorf("unable to save task changes: %w", err)
		}
		return nil
	})
}

// saveWithoutTransaction saves state without transaction support
func (s *TransactionService) saveWithoutTransaction(
	ctx context.Context,
	state *task.State,
) error {
	// Fetch current state to merge changes similarly to transactional flow
	latest, err := s.taskRepo.GetState(ctx, state.TaskExecID)
	if err != nil {
		// If not found, persist as new
		if errors.Is(err, store.ErrTaskNotFound) {
			if err := s.taskRepo.UpsertState(ctx, state); err != nil {
				return fmt.Errorf("unable to save new task state: %w", err)
			}
			return nil
		}
		return fmt.Errorf("unable to retrieve existing task state: %w", err)
	}
	s.mergeStateChanges(latest, state)
	if err := s.taskRepo.UpsertState(ctx, latest); err != nil {
		return fmt.Errorf("unable to save task changes: %w", err)
	}
	// Mirror merged fields back to caller for consistency with transactional path
	state.Status = latest.Status
	state.Output = latest.Output
	state.Error = latest.Error
	if state.Input == nil && latest.Input != nil {
		state.Input = latest.Input
	}
	return nil
}

// applyWithTransaction applies transformation within a transaction
func (s *TransactionService) applyWithTransaction(
	ctx context.Context,
	taskExecID core.ID,
	transformer StateTransformer,
	txRepo transactionalRepo,
) error {
	return txRepo.WithTransaction(ctx, func(repo task.Repository) error {
		state, err := repo.GetStateForUpdate(ctx, taskExecID)
		if err != nil {
			return fmt.Errorf("unable to retrieve task for update: %w", err)
		}
		if err := transformer(state); err != nil {
			return fmt.Errorf("task processing failed: %w", err)
		}
		if err := repo.UpsertState(ctx, state); err != nil {
			return fmt.Errorf("unable to save processed task: %w", err)
		}
		return nil
	})
}

// applyWithoutTransaction applies transformation without transaction support
func (s *TransactionService) applyWithoutTransaction(
	ctx context.Context,
	taskExecID core.ID,
	transformer StateTransformer,
) error {
	// Get current state
	state, err := s.taskRepo.GetState(ctx, taskExecID)
	if err != nil {
		return fmt.Errorf("unable to retrieve task: %w", err)
	}
	// Apply transformation
	if err := transformer(state); err != nil {
		return fmt.Errorf("task processing failed: %w", err)
	}
	// Save transformed state
	if err := s.taskRepo.UpsertState(ctx, state); err != nil {
		return fmt.Errorf("unable to save processed task: %w", err)
	}
	return nil
}

// mergeStateChanges merges mutable fields from source into target.
//
// - Merges: Status, Output, Error
// - Backfills Input only if target is missing it (doesn't overwrite existing Input)
// - Does not touch identity/structural fields (IDs, parent, etc.)
func (s *TransactionService) mergeStateChanges(target, source *task.State) {
	if target == nil || source == nil {
		return
	}
	// Status (only set when valid)
	if source.Status.IsValid() {
		target.Status = source.Status
	}
	// Output (deep copy to avoid aliasing)
	if source.Output != nil {
		target.Output = deepCopyOrSame(source.Output)
	} else {
		target.Output = nil
	}
	// Error (deep copy to avoid aliasing)
	if source.Error != nil {
		target.Error = deepCopyOrSame(source.Error)
	} else {
		target.Error = nil
	}
	// Backfill Input only when missing in target (deep copy to avoid aliasing)
	if target.Input == nil && source.Input != nil {
		target.Input = deepCopyOrSame(source.Input)
	}
}

// returns the copied value. If deep copy fails, it returns the original value unchanged.
func deepCopyOrSame[T any](v T) T {
	copied, err := core.DeepCopy(v)
	if err != nil {
		return v
	}
	return copied
}
