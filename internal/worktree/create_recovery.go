package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

// gitAddLockReason is the lock `git worktree add` holds until its checkout
// completes; a killed add leaves it behind.
const gitAddLockReason = "initializing"

// retryFailedCreateRollback finishes a rollback a failed row records as
// incomplete. The failed row no longer reserves its path or branch, so it
// touches only what Git still attributes to it and never discards work:
//   - a checkout at the recorded path on the recorded branch is removed only
//     while it still holds the add lock of the interrupted creation, which a
//     half-written checkout keeps and a replacement does not;
//   - the branch this creation minted is deleted only while it still points
//     at the recorded head (no unique commits) and no checkout holds it.
//
// Anything it cannot attribute is reported, and the row stays marked.
func (s *Service) retryFailedCreateRollback(ctx context.Context, workspace Workspace, item Worktree) error {
	if item.CreateRollbackError == "" {
		return nil
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return err
	}
	defer release()
	entries, err := s.readWorktreeList(ctx, workspace.Root, commonDir)
	if err != nil {
		return err
	}
	branchHeld := false
	for _, entry := range entries {
		if filepath.Clean(entry.Path) != filepath.Clean(item.Path) {
			branchHeld = branchHeld || entry.Branch == item.Branch
			continue
		}
		if entry.Branch != item.Branch {
			return fmt.Errorf("worktree: retry creation rollback: %s is checked out on another branch", item.Path)
		}
		if !entry.Locked || entry.LockReason != gitAddLockReason {
			return fmt.Errorf(
				"worktree: retry creation rollback: %s is not attributable to the failed creation",
				item.Path,
			)
		}
		if err := s.removeCreatedCheckout(ctx, workspace.Root, commonDir, item.Path, 2); err != nil {
			return err
		}
	}
	if _, statErr := os.Lstat(item.Path); statErr == nil {
		return fmt.Errorf("worktree: retry creation rollback: Git does not attribute %s to this creation", item.Path)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("worktree: inspect creation rollback path: %w", statErr)
	}
	if item.CreatedBranch && item.CreatedHead != "" {
		if branchHeld {
			return fmt.Errorf("worktree: retry creation rollback: branch %s is checked out elsewhere", item.Branch)
		}
		if err := s.retryCreatedBranchRollback(ctx, workspace.Root, item); err != nil {
			return err
		}
	}
	if err := s.store.CompleteFailedCreateRollback(ctx, item.WorkspaceID, item.ID, s.now().UTC()); err != nil {
		return fmt.Errorf("worktree: record completed creation rollback: %w", err)
	}
	item.CreateRollbackError = ""
	s.logger.InfoContext(ctx, "worktree creation rollback completed on recovery",
		"workspace_id", item.WorkspaceID, "worktree_id", item.ID)
	s.invalidateDiscovery(item.WorkspaceID)
	s.publishCatalogEvent(CatalogEventUpserted, item)
	return nil
}

// retryCreatedBranchRollback deletes the minted branch only while it still
// points at the recorded head. A missing ref is already rolled back.
func (s *Service) retryCreatedBranchRollback(ctx context.Context, root string, item Worktree) error {
	ref := "refs/heads/" + item.Branch
	head, stderr, err := s.runner.Run(ctx, root, "for-each-ref", "--format=%(objectname)", ref)
	if err != nil {
		return fmt.Errorf(
			"worktree: read branch for creation rollback: %s: %w",
			diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	switch strings.TrimSpace(string(head)) {
	case "":
		return nil
	case item.CreatedHead:
	default:
		return fmt.Errorf("worktree: retry creation rollback: branch %s moved past its recorded head", item.Branch)
	}
	if _, stderr, err := s.runner.Run(ctx, root, "update-ref", "-d", ref, item.CreatedHead); err != nil {
		return fmt.Errorf(
			"worktree: retry branch rollback: %s: %w",
			diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	return nil
}

type interruptedCheckout int

const (
	interruptedCheckoutUsable interruptedCheckout = iota
	interruptedCheckoutGone
	interruptedCheckoutForeign
)

// errCheckoutReplaced names a recorded checkout path that now holds something
// other than this creation's linked worktree; recovery leaves it in place.
var errCheckoutReplaced = errors.New("the checkout path no longer holds this creation's linked worktree; left in place")

// inspectInterruptedCheckout classifies an interrupted copy/setup row's
// checkout. Errors that do not prove a replacement are returned so the row
// stays pending for the next boot instead of being rolled back.
func (s *Service) inspectInterruptedCheckout(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
) (interruptedCheckout, error) {
	info, err := os.Lstat(item.Path)
	if errors.Is(err, os.ErrNotExist) {
		return interruptedCheckoutGone, nil
	}
	if err != nil {
		return 0, fmt.Errorf("worktree: inspect interrupted checkout: %w", err)
	}
	if !info.IsDir() {
		return interruptedCheckoutForeign, nil
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return 0, err
	}
	identity, err := s.validateAdoptionIdentity(ctx, item.Path, commonDir)
	if err != nil || identity.adminGitDir != item.GitDir {
		return interruptedCheckoutForeign, nil
	}
	return interruptedCheckoutUsable, nil
}
