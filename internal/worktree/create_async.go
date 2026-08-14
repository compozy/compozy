package worktree

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type createOperation struct {
	cancel   context.CancelFunc
	done     chan struct{}
	once     sync.Once
	errMu    sync.Mutex
	err      error
	canceled bool
}

// CreateAccepted durably inserts a pending worktree and continues
// materialization after the request ends. Callers observe phases through the
// catalog stream and canonical lifecycle events.
func (s *Service) CreateAccepted(
	ctx context.Context,
	workspaceID string,
	options CreateOptions,
) (*Worktree, error) {
	item, _, err := s.startAcceptedCreation(ctx, workspaceID, options)
	return item, err
}

// CreateReady inserts a visible pending worktree, waits for materialization,
// and returns only after the worktree is ready for session binding.
func (s *Service) CreateReady(
	ctx context.Context,
	workspaceID string,
	options CreateOptions,
) (*Worktree, error) {
	item, operation, err := s.startAcceptedCreation(ctx, workspaceID, options)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, errors.Join(ctx.Err(), s.cancelReadyCreation(ctx, workspaceID, item.ID, operation))
	case <-operation.done:
	}
	canceled, operationErr := operation.result()
	if operationErr != nil {
		return nil, fmt.Errorf("worktree: materialize accepted creation: %w", operationErr)
	}
	if canceled {
		return nil, context.Canceled
	}
	ready, err := s.store.Get(ctx, workspaceID, item.ID)
	if err != nil {
		return nil, fmt.Errorf("worktree: read ready creation: %w", err)
	}
	if ready == nil || ready.State != StateReady {
		return nil, fmt.Errorf("%w: %s", ErrNotReady, item.ID)
	}
	return ready, nil
}

func (s *Service) cancelReadyCreation(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
	operation *createOperation,
) error {
	operation.once.Do(operation.cancel)
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), defaultGitTimeout)
	defer cancelCleanup()
	select {
	case <-operation.done:
	case <-cleanupCtx.Done():
		return fmt.Errorf("worktree: cancel ready creation: %w", cleanupCtx.Err())
	}
	canceled, operationErr := operation.result()
	if operationErr != nil || canceled {
		return operationErr
	}
	if _, err := s.Remove(cleanupCtx, workspaceID, worktreeID, true); err != nil {
		return fmt.Errorf("worktree: remove completed creation after caller cancellation: %w", err)
	}
	return nil
}

func (s *Service) startAcceptedCreation(
	ctx context.Context,
	workspaceID string,
	options CreateOptions,
) (*Worktree, *createOperation, error) {
	workspace, item, commonDir, err := s.prepareCreation(ctx, workspaceID, options)
	if err != nil {
		return nil, nil, err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return nil, nil, err
	}
	if err := s.insertPendingUnderLock(ctx, workspace, commonDir, &item, options); err != nil {
		release()
		return nil, nil, err
	}

	// The operation owns cancel until runAcceptedCreation finishes or an explicit cancellation wins.
	operationContext, cancel := context.WithCancel(context.WithoutCancel(ctx)) // #nosec G118
	operation := &createOperation{cancel: cancel, done: make(chan struct{})}
	s.setCreateOperation(item.WorkspaceID, item.ID, operation)
	go s.runAcceptedCreation(
		operationContext, operation, release, workspace, commonDir, item, options,
	)
	return &item, operation, nil
}

func (s *Service) runAcceptedCreation(
	ctx context.Context,
	operation *createOperation,
	release func(),
	workspace Workspace,
	commonDir string,
	item Worktree,
	options CreateOptions,
) {
	defer release()
	defer close(operation.done)
	defer s.deleteCreateOperation(item.WorkspaceID, item.ID, operation)

	err := s.materializePending(ctx, workspace, &item, options)
	if err != nil {
		rollbackErr := s.rollbackCreate(context.WithoutCancel(ctx), workspace, commonDir, item)
		deleteErr := s.store.DeletePending(context.WithoutCancel(ctx), item.WorkspaceID, item.ID)
		if errors.Is(ctx.Err(), context.Canceled) {
			operation.setResult(errors.Join(rollbackErr, deleteErr), rollbackErr == nil && deleteErr == nil)
		} else {
			operation.setResult(errors.Join(err, rollbackErr, deleteErr), false)
		}
		if errors.Is(ctx.Err(), context.Canceled) && rollbackErr == nil && deleteErr == nil {
			s.emit(context.WithoutCancel(ctx), EventCreationCanceled, item)
		} else if deleteErr == nil {
			s.publishCatalogFailure(item, err)
		}
		return
	}
	s.invalidateDiscovery(item.WorkspaceID)
	s.emit(ctx, EventCreated, item)
}

func (s *Service) cancelAcceptedCreation(ctx context.Context, workspaceID, id string) (bool, error) {
	operation := s.getCreateOperation(workspaceID, id)
	if operation == nil {
		return false, nil
	}
	operation.once.Do(operation.cancel)
	select {
	case <-ctx.Done():
		return true, ctx.Err()
	case <-operation.done:
	}
	canceled, err := operation.result()
	if err != nil {
		return true, fmt.Errorf("worktree: cancel accepted creation: %w", err)
	}
	if !canceled {
		return true, ErrNotPending
	}
	return true, nil
}

func (s *Service) setCreateOperation(workspaceID, id string, operation *createOperation) {
	s.createMu.Lock()
	s.creates[createOperationKey(workspaceID, id)] = operation
	s.createMu.Unlock()
}

func (s *Service) getCreateOperation(workspaceID, id string) *createOperation {
	s.createMu.Lock()
	operation := s.creates[createOperationKey(workspaceID, id)]
	s.createMu.Unlock()
	return operation
}

func (s *Service) deleteCreateOperation(workspaceID, id string, operation *createOperation) {
	s.createMu.Lock()
	key := createOperationKey(workspaceID, id)
	if s.creates[key] == operation {
		delete(s.creates, key)
	}
	s.createMu.Unlock()
}

func createOperationKey(workspaceID, id string) string {
	return workspaceID + "\x00" + id
}

func (o *createOperation) setResult(err error, canceled bool) {
	o.errMu.Lock()
	o.err = err
	o.canceled = canceled
	o.errMu.Unlock()
}

func (o *createOperation) result() (bool, error) {
	o.errMu.Lock()
	defer o.errMu.Unlock()
	return o.canceled, o.err
}
