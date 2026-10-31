package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/diagnostics"
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
		recoverErr = errors.Join(recoverErr, s.recoverRow(ctx, workspace, item, capabilityErr))
	}
	running, err := s.store.ListRunningExitOperations(ctx)
	if err != nil {
		recoverErr = errors.Join(recoverErr, fmt.Errorf("worktree: list interrupted exit operations: %w", err))
		return recoverErr
	}

	for _, operation := range running {
		if ExitAction(operation.Action) == ExitActionDeliver {
			continue
		}
		item, itemErr := s.store.Get(ctx, operation.WorkspaceID, operation.WorktreeID)
		if itemErr != nil || item == nil {
			recoverErr = errors.Join(recoverErr, itemErr, ErrNotFound)
			continue
		}
		operation.ProfileID = item.ProfileID
		_, finishErr := s.finishExitOperation(
			ctx,
			operation,
			exitStepFailed,
			EventExitActionFailed,
			ExitEventPayload{
				OperationID: operation.ID,
				Action:      ExitAction(operation.Action),
				State:       exitStepFailed,
				Message:     "Exit action interrupted by daemon restart.",
			},
		)
		recoverErr = errors.Join(recoverErr, finishErr)
	}

	return recoverErr
}

// recoverRemoval finishes a removal interrupted by a daemon stop. Anything
// that keeps it from finishing, such as a checkout the interrupted Git command
// left orphaned or an unreadable leftover, returns the record to the user and
// is logged; one worktree must not fail daemon startup.
func (s *Service) recoverRemoval(ctx context.Context, item Worktree) error {
	refusalResult, err := s.removeFenced(ctx, item, false)
	if err == nil && refusalResult == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(err, ctxErr)
	}
	attrs := []any{"worktree_id", item.ID, "error", diagnostics.RedactAndBound(fmt.Sprint(err), 2048)}
	if refusalResult != nil || errors.Is(err, ErrCheckoutOrphaned) {
		s.logger.WarnContext(ctx, "interrupted worktree removal needs confirmation", attrs...)
		return nil
	}
	s.logger.ErrorContext(ctx, "interrupted worktree removal could not finish", attrs...)
	return nil
}

// recoverRow settles one recoverable row. Every row waits for Git: even a
// copy/setup checkout needs its Git identity checked before it is usable.
func (s *Service) recoverRow(ctx context.Context, workspace Workspace, item Worktree, capabilityErr error) error {
	if capabilityErr != nil {
		return nil
	}
	switch item.State {
	case StatePending:
		return s.recoverPending(ctx, workspace, item)
	case StateRemoving:
		return s.recoverRemoval(ctx, item)
	case StateMissing:
		return s.restoreMissing(ctx, workspace, item)
	case StateFailed:
		if err := s.retryFailedCreateRollback(ctx, workspace, item); err != nil {
			s.logger.WarnContext(ctx, "worktree creation rollback is still incomplete",
				"worktree_id", item.ID, logErrorKey, diagnostics.RedactAndBound(err.Error(), 2048))
		}
	}
	return nil
}

func (s *Service) recoverPending(ctx context.Context, workspace Workspace, item Worktree) error {
	switch item.PendingPhase {
	case PhaseCopy, PhaseSetup:
		// A rollback may have run without its failure being recorded, so the
		// checkout must still be this creation's linked worktree to be usable.
		checkout, err := s.inspectInterruptedCheckout(ctx, workspace, item)
		if err != nil {
			return err
		}
		switch checkout {
		case interruptedCheckoutGone:
			return s.recoverInterruptedCreate(ctx, workspace, item, true)
		case interruptedCheckoutForeign:
			return s.recordCreateFailure(ctx, item, errCreationInterrupted, errCheckoutReplaced)
		}
		message := "setup interrupted by daemon restart"
		if err := s.store.CompleteCreate(
			ctx,
			item.WorkspaceID,
			item.ID,
			SetupFailed,
			message,
			s.now().UTC(),
		); err != nil {
			return fmt.Errorf("worktree: recover usable checkout: %w", err)
		}
		item.State, item.SetupState, item.SetupError = StateReady, SetupFailed, message
		s.emit(ctx, EventSetupFailed, item)
		return nil
	default:
		return s.recoverInterruptedCreate(ctx, workspace, item, false)
	}
}

// recoverInterruptedCreate rolls back an interrupted creation and records it
// failed. When the checkout was found gone, that is rechecked under the lock so
// a checkout that appeared meanwhile is left for the next boot to classify.
func (s *Service) recoverInterruptedCreate(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
	checkoutGone bool,
) error {
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return err
	}
	defer release()
	if checkoutGone {
		if _, statErr := os.Lstat(item.Path); statErr == nil {
			return nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("worktree: recheck interrupted checkout: %w", statErr)
		}
	}
	rollbackErr := s.rollbackCreate(ctx, workspace, commonDir, item)
	recordErr := s.recordCreateFailure(ctx, item, errCreationInterrupted, rollbackErr)
	return errors.Join(rollbackErr, recordErr)
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
