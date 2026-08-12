package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
)

func (s *Service) RecoverCreations(ctx context.Context) error {
	rows, err := s.store.ListRecoverable(ctx)
	if err != nil {
		return fmt.Errorf("worktree: list recoverable rows: %w", err)
	}
	_, capabilityErr := s.capability.Check(ctx)
	var recoverErr error
	for _, item := range rows {
		workspace, workspaceErr := s.resolveWorkspace(ctx, item.WorkspaceID)
		if workspaceErr != nil {
			continue
		}
		workspaceInfo, statErr := os.Stat(workspace.Root)
		if statErr != nil || !workspaceInfo.IsDir() {
			continue
		}
		switch item.State {
		case StatePending:
			if capabilityErr != nil && item.PendingPhase != PhaseCopy && item.PendingPhase != PhaseSetup {
				continue
			}
			recoverErr = errors.Join(recoverErr, s.recoverPending(ctx, workspace, item))
		case StateRemoving:
			if capabilityErr == nil {
				_, removeErr := s.removeFenced(ctx, item, false)
				recoverErr = errors.Join(recoverErr, removeErr)
			}
		case StateMissing:
			if capabilityErr == nil {
				recoverErr = errors.Join(recoverErr, s.restoreMissing(ctx, workspace, item))
			}
		}
	}
	if err := s.store.FailRunningExitOperations(ctx, s.now().UTC()); err != nil {
		recoverErr = errors.Join(recoverErr, fmt.Errorf("worktree: fail interrupted exit operations: %w", err))
	}
	return recoverErr
}

func (s *Service) recoverPending(ctx context.Context, workspace Workspace, item Worktree) error {
	switch item.PendingPhase {
	case PhaseCopy, PhaseSetup:
		message := "setup interrupted by daemon restart"
		if err := s.store.CompleteCreate(ctx, item.WorkspaceID, item.ID, SetupFailed, message, s.now().UTC()); err != nil {
			return fmt.Errorf("worktree: recover usable checkout: %w", err)
		}
		item.State, item.SetupState, item.SetupError = StateReady, SetupFailed, message
		s.emit(ctx, EventSetupFailed, item)
		return nil
	default:
		commonDir, err := s.commonDir(ctx, workspace.Root)
		if err != nil {
			return err
		}
		release, err := s.locks.Acquire(ctx, commonDir)
		if err != nil {
			return err
		}
		defer release()
		rollbackErr := s.rollbackCreate(ctx, workspace, commonDir, item)
		stateErr := s.store.SetState(ctx, item.WorkspaceID, item.ID, StateFailed, s.now().UTC())
		return errors.Join(rollbackErr, stateErr)
	}
}

func (s *Service) restoreMissing(ctx context.Context, workspace Workspace, item Worktree) error {
	if _, err := os.Stat(item.Path); err != nil {
		return nil
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return nil
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return err
	}
	defer release()
	current, err := s.store.Get(ctx, item.WorkspaceID, item.ID)
	if err != nil || current == nil || current.State != StateMissing {
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("worktree: reread missing row: %w", err)
		}
		return nil
	}
	identity, err := s.validateAdoptionIdentity(ctx, item.Path, commonDir)
	if err != nil || identity.adminGitDir != item.GitDir {
		return nil
	}
	swapped, err := s.store.CompareAndSwapState(
		ctx, item.WorkspaceID, item.ID, StateMissing, StateReady, s.now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("worktree: restore missing row: %w", err)
	}
	if swapped {
		s.invalidateDiscovery(item.WorkspaceID)
	}
	return nil
}
