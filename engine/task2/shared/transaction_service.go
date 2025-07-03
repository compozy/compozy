package shared

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/jackc/pgx/v5"
)

// TransactionService handles all transaction-related operations for task state management
//
// This service implements the transactional interface pattern by:
// 1. Using type assertion to check if the repository supports transactions
// 2. Gracefully degrading to non-transactional operations when transactions aren't available
// 3. Providing consistent API regardless of underlying repository capabilities
//
// The pattern is implemented through the TransactionalRepository interface:
//
//	type TransactionalRepository interface {
//	    WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
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
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
	GetStateForUpdate(ctx context.Context, tx pgx.Tx, taskExecID core.ID) (*task.State, error)
	UpsertStateWithTx(ctx context.Context, tx pgx.Tx, state *task.State) error
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
	return txRepo.WithTx(ctx, func(tx pgx.Tx) error {
		// Get latest state with row-level lock
		latestState, err := txRepo.GetStateForUpdate(ctx, tx, state.TaskExecID)
		if err != nil {
			return fmt.Errorf("unable to acquire task lock for update: %w", err)
		}

		// Merge changes into latest state
		s.mergeStateChanges(latestState, state)

		// Copy state back to original to ensure caller sees the merged state
		*state = *latestState

		// Save with transaction safety
		if err := txRepo.UpsertStateWithTx(ctx, tx, latestState); err != nil {
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
	if err := s.taskRepo.UpsertState(ctx, state); err != nil {
		return fmt.Errorf("unable to save task changes: %w", err)
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
	return txRepo.WithTx(ctx, func(tx pgx.Tx) error {
		// Get latest state with lock
		state, err := txRepo.GetStateForUpdate(ctx, tx, taskExecID)
		if err != nil {
			return fmt.Errorf("unable to retrieve task for update: %w", err)
		}

		// Apply transformation
		if err := transformer(state); err != nil {
			return fmt.Errorf("task processing failed: %w", err)
		}

		// Save transformed state
		if err := txRepo.UpsertStateWithTx(ctx, tx, state); err != nil {
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

// mergeStateChanges merges changes from source state into target state
// Only Status, Output, and Error fields are merged because:
// - These are the only fields that change during task execution
// - Other fields like TaskID, TaskExecID, WorkflowID are immutable identifiers
// - Input and Metadata are set at task creation and should not be modified
// - ParentStateID and hierarchy fields are structural and must remain unchanged
func (s *TransactionService) mergeStateChanges(target, source *task.State) {
	target.Status = source.Status
	target.Output = source.Output
	target.Error = source.Error
	// CRITICAL FIX: Also merge Input if target doesn't have it but source does
	// This handles cases where the initial save didn't include Input properly
	if target.Input == nil && source.Input != nil {
		target.Input = source.Input
	}
}
