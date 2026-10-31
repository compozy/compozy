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

// rollbackIncompleteMarker separates a failed creation's cause from the
// rollback error in create_error. Boot reconciliation retries such rows.
const rollbackIncompleteMarker = "; rollback incomplete: "

// logErrorKey names the structured log attribute carrying the primary cause.
const logErrorKey = "error"

// creationFailedPayload extends the lifecycle payload with the failed phase
// and the redacted cause recorded on the row.
type creationFailedPayload struct {
	HookWorktree
	PendingPhase PendingPhase `json:"pending_phase,omitempty"`
	Error        string       `json:"error"`
}

// rollbackCreate undoes one creation's Git artifacts in dependency order. The
// checkout and its Git admin entry go first; the branch is deleted only when
// this creation minted it and the checkout is gone, so an incomplete rollback
// never leaves a registered worktree on a dangling branch ref.
func (s *Service) rollbackCreate(
	ctx context.Context,
	workspace Workspace,
	commonDir string,
	item Worktree,
) error {
	if checkoutStarted(item) {
		if err := s.rollbackCheckout(ctx, workspace.Root, commonDir, item); err != nil {
			return err
		}
	}
	if !item.CreatedBranch || item.CreatedHead == "" {
		return nil
	}
	ref := "refs/heads/" + item.Branch
	if _, stderr, err := s.runner.Run(ctx, workspace.Root, "update-ref", "-d", ref, item.CreatedHead); err != nil {
		return fmt.Errorf(
			"worktree: roll back branch: %s: %w",
			diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	return nil
}

func checkoutStarted(item Worktree) bool {
	switch item.PendingPhase {
	case PhaseCheckout, PhaseCopy, PhaseSetup:
		return true
	default:
		return item.GitDir != ""
	}
}

// rollbackCheckout removes the checkout and its Git admin entry. An
// interrupted `git worktree add` leaves the entry locked as "initializing", so
// a creation-owned (pending or failed) row passes the second --force Git
// requires for locked trees.
// Git refuses to remove an entry whose directory is already gone; that entry
// is dropped directly once nothing on disk can still use it.
func (s *Service) rollbackCheckout(ctx context.Context, root, commonDir string, item Worktree) error {
	args := []string{gitDirFlag, commonDir, worktreeDescriptorName, exitRemoveAction, "--force"}
	if item.State == StatePending || item.State == StateFailed {
		args = append(args, "--force")
	}
	_, stderr, err := s.runCheckoutRemoval(ctx, root, append(args, item.Path)...)
	if err == nil {
		return nil
	}
	removeErr := fmt.Errorf(
		"worktree: roll back checkout: %s: %w",
		diagnostics.RedactAndBound(string(stderr), 2048), err,
	)
	if _, statErr := os.Lstat(item.Path); statErr == nil {
		return removeErr
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return errors.Join(removeErr, fmt.Errorf("worktree: inspect checkout rollback path: %w", statErr))
	}
	// Git refused only because the directory is gone; the entry is all that is left.
	if err := dropCheckoutAdminEntries(commonDir, item.Path); err != nil {
		return errors.Join(removeErr, err)
	}
	return nil
}

// dropCheckoutAdminEntries deletes the admin entries under commonDir whose
// gitdir backlink names path/.git. Callers first prove path is absent. Git
// names an entry after the checkout's base name, adding digits on collision;
// such an entry whose backlink cannot be read could still be this checkout's,
// so it is reported instead of being treated as removed.
func dropCheckoutAdminEntries(commonDir, path string) error {
	adminRoot := filepath.Join(commonDir, "worktrees")
	entries, err := os.ReadDir(adminRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("worktree: list Git admin entries: %w", err)
	}
	want := filepath.Join(filepath.Clean(path), ".git")
	base := filepath.Base(path)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		adminPath := filepath.Join(adminRoot, entry.Name())
		raw, readErr := os.ReadFile(filepath.Join(adminPath, "gitdir"))
		if readErr != nil {
			if adminEntryNamedFor(entry.Name(), base) {
				return fmt.Errorf("worktree: verify Git admin entry %s: %w", entry.Name(), readErr)
			}
			continue
		}
		backlink := strings.TrimSpace(string(raw))
		if !filepath.IsAbs(backlink) {
			backlink = filepath.Join(adminPath, backlink)
		}
		if filepath.Clean(backlink) != want {
			continue
		}
		if err := os.RemoveAll(adminPath); err != nil {
			return fmt.Errorf("worktree: drop Git admin entry: %w", err)
		}
	}
	return nil
}

func adminEntryNamedFor(name, base string) bool {
	suffix, ok := strings.CutPrefix(name, base)
	if !ok {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// recordCreateFailure keeps a failed creation visible as a failed row with its
// cause until dismissal, then reports it through a structured log, the
// worktree.creation_failed lifecycle event, and the catalog stream. A row the
// store cannot settle stays pending for boot reconciliation.
func (s *Service) recordCreateFailure(ctx context.Context, item Worktree, cause, rollbackErr error) error {
	message := createFailureMessage(item.PendingPhase, cause, rollbackErr)
	now := s.now().UTC()
	recordErr := s.store.FailCreate(ctx, item.WorkspaceID, item.ID, message, now)
	s.logCreateFailure(ctx, item, cause, rollbackErr, recordErr)
	if recordErr != nil {
		return fmt.Errorf("worktree: record failed creation: %w", recordErr)
	}
	item.State, item.CreateError, item.UpdatedAt = StateFailed, message, now
	s.invalidateDiscovery(item.WorkspaceID)
	s.emitWithPayload(ctx, EventCreationFailed, item, creationFailedPayload{
		HookWorktree: eventPayloadFromWorktree(item, ""), PendingPhase: item.PendingPhase, Error: message,
	})
	s.publishCatalogFailure(item, errors.New(message))
	return nil
}

func (s *Service) logCreateFailure(ctx context.Context, item Worktree, cause, rollbackErr, recordErr error) {
	attrs := []any{
		"workspace_id", item.WorkspaceID, "worktree_id", item.ID, "phase", string(item.PendingPhase),
		logErrorKey, diagnostics.RedactAndBound(fmt.Sprint(cause), setupDiagnosticLimit),
	}
	if rollbackErr == nil && recordErr == nil {
		s.logger.WarnContext(ctx, "worktree creation failed and was rolled back", attrs...)
		return
	}
	if rollbackErr != nil {
		attrs = append(attrs, "rollback_error", diagnostics.RedactAndBound(rollbackErr.Error(), setupDiagnosticLimit))
	}
	if recordErr != nil {
		attrs = append(attrs, "record_error", diagnostics.RedactAndBound(recordErr.Error(), setupDiagnosticLimit))
	}
	s.logger.ErrorContext(ctx, "worktree creation failed and its rollback is incomplete", attrs...)
}

func createFailureMessage(phase PendingPhase, cause, rollbackErr error) string {
	message := fmt.Sprint(cause)
	if phase != PhaseNone {
		message = string(phase) + " phase failed: " + message
	}
	if rollbackErr != nil {
		message += rollbackIncompleteMarker + rollbackErr.Error()
	}
	return diagnostics.RedactAndBound(message, setupDiagnosticLimit)
}

// retryFailedCreateRollback finishes a rollback a failed row records as
// incomplete. The failed row no longer reserves its path or branch, so it
// touches only what Git still attributes to it: a checkout registered at the
// recorded path on the recorded branch, and a branch this creation minted that
// still points at its recorded head and that no checkout holds.
func (s *Service) retryFailedCreateRollback(ctx context.Context, workspace Workspace, item Worktree) error {
	cause, _, incomplete := strings.Cut(item.CreateError, rollbackIncompleteMarker)
	if !incomplete {
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
		if filepath.Clean(entry.Path) == filepath.Clean(item.Path) {
			if entry.Branch != item.Branch {
				return nil
			}
			if err := s.rollbackCheckout(ctx, workspace.Root, commonDir, item); err != nil {
				return err
			}
			continue
		}
		branchHeld = branchHeld || entry.Branch == item.Branch
	}
	if _, statErr := os.Lstat(item.Path); statErr == nil {
		return fmt.Errorf(
			"worktree: retry failed creation rollback: %s is not a checkout Git attributes to it",
			item.Path,
		)
	}
	if item.CreatedBranch && item.CreatedHead != "" && !branchHeld {
		ref := "refs/heads/" + item.Branch
		head, _, headErr := s.runner.Run(ctx, workspace.Root, "rev-parse", "--verify", "--quiet", ref)
		if headErr == nil && strings.TrimSpace(string(head)) == item.CreatedHead {
			if _, stderr, err := s.runner.Run(
				ctx,
				workspace.Root,
				"update-ref",
				"-d",
				ref,
				item.CreatedHead,
			); err != nil {
				return fmt.Errorf(
					"worktree: retry branch rollback: %s: %w",
					diagnostics.RedactAndBound(string(stderr), 2048), err,
				)
			}
		}
	}
	if err := s.store.UpdateFailedCreateError(ctx, item.WorkspaceID, item.ID, cause, s.now().UTC()); err != nil {
		return fmt.Errorf("worktree: record completed creation rollback: %w", err)
	}
	item.CreateError = cause
	s.logger.InfoContext(ctx, "worktree creation rollback completed on recovery",
		"workspace_id", item.WorkspaceID, "worktree_id", item.ID)
	s.invalidateDiscovery(item.WorkspaceID)
	s.publishCatalogEvent(CatalogEventUpserted, item)
	return nil
}
