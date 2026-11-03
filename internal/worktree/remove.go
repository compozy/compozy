package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/diagnostics"
)

const removalHeadRef = "HEAD"

type RemovalRisk struct {
	ChangedFiles    int
	Insertions      int
	Deletions       int
	UnpushedCommits int
	ExistsOnRemote  bool
	// LeftoverFiles counts files left in an orphaned checkout. Git can no
	// longer tell which of them hold changes, so all of them are at risk.
	LeftoverFiles int
}

type RemovalRefusal struct {
	Code      string
	Risk      RemovalRisk
	Downgrade bool
}

type HookPreRemovePayload struct {
	Worktree   HookWorktree `json:"worktree"`
	Force      bool         `json:"force"`
	Risk       RemovalRisk  `json:"risk"`
	Discovered bool         `json:"discovered,omitempty"`
}

// removalMode is what a removal may discard. force deletes uncommitted files and
// unpushed commits; keepBranch removes only the checkout, so its branch keeps any
// unpushed commits and is never reclaimed. Only force deletes uncommitted files.
type removalMode struct {
	force      bool
	keepBranch bool
}

func (s *Service) Remove(
	ctx context.Context,
	workspaceID string,
	id string,
	force bool,
) (*RemovalRefusal, error) {
	return s.remove(ctx, workspaceID, id, removalMode{force: force})
}

// RemoveKeepingBranch removes a clean checkout and keeps its branch: unpushed
// commits stay on the branch, so they do not block the removal, while uncommitted
// files still refuse it, before and after removal hooks run. Moving a session to
// another profile checks the same branch out again elsewhere.
func (s *Service) RemoveKeepingBranch(ctx context.Context, workspaceID, id string) (*RemovalRefusal, error) {
	return s.remove(ctx, workspaceID, id, removalMode{keepBranch: true})
}

func (s *Service) remove(
	ctx context.Context,
	workspaceID string,
	id string,
	mode removalMode,
) (*RemovalRefusal, error) {
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || item == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("worktree: read removal target: %w", err)
	}
	if item.State == StateRemoved || item.State == StateDismissed {
		return nil, nil
	}
	if item.State != StateReady && item.State != StateMissing {
		return nil, ErrNotReady
	}
	releaseUsage, acquired := s.usage.tryAcquireExclusive(worktreeUsageKey(item.WorkspaceID, item.ID))
	if !acquired {
		return nil, ErrOperationInProgress
	}
	defer releaseUsage()

	item, err = s.store.Get(ctx, workspaceID, item.ID)
	if errors.Is(err, ErrNotFound) || item == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("worktree: reread removal target: %w", err)
	}
	if item.State == StateRemoved || item.State == StateDismissed {
		return nil, nil
	}
	if item.State != StateReady && item.State != StateMissing {
		return nil, ErrNotReady
	}
	if item.State == StateMissing {
		return nil, s.removeMissing(ctx, *item)
	}
	if _, err := s.capability.Check(ctx); err != nil {
		return nil, err
	}
	swapped, err := s.store.FenceRemoval(ctx, workspaceID, item.ID, mode.keepBranch, s.now().UTC())
	if err != nil {
		return nil, fmt.Errorf("worktree: fence removal: %w", err)
	}
	if !swapped {
		return nil, ErrNotReady
	}
	item.RemovalKeepsBranch = mode.keepBranch
	return s.removeFenced(ctx, *item, mode)
}

func (s *Service) removeFenced(ctx context.Context, item Worktree, mode removalMode) (_ *RemovalRefusal, err error) {
	restoreReady := true
	defer func() {
		if !restoreReady {
			return
		}
		if revertErr := s.store.SetState(
			context.WithoutCancel(ctx), item.WorkspaceID, item.ID, StateReady, s.now().UTC(),
		); revertErr != nil {
			s.logger.Error(
				"worktree removal fence rollback failed",
				"worktree_id", item.ID,
				"error", diagnostics.RedactAndBound(revertErr.Error(), 2048),
			)
		}
	}()

	workspace, err := s.resolveProfileWorkspace(ctx, item.WorkspaceID, item.ProfileID)
	if err != nil {
		return nil, err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return nil, err
	}

	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return nil, err
	}
	defer release()

	item, err = s.requireFencedRemovalTarget(ctx, item, false)
	if err != nil {
		return nil, err
	}
	terminal, risk, refusalResult, err := s.evaluateFencedRemoval(ctx, workspace, item, commonDir, mode)
	if terminal {
		restoreReady = false
	}
	if terminal || err != nil || refusalResult != nil {
		return refusalResult, err
	}
	if err := s.dispatchGate(ctx, EventPreRemove, HookPreRemovePayload{
		Worktree: eventPayloadFromWorktree(item, workspace.Root), Force: mode.force, Risk: risk,
	}); err != nil {
		return nil, err
	}

	// Hooks can run arbitrary code. Close that final race before deleting.
	item, err = s.requireFencedRemovalTarget(ctx, item, true)
	if err != nil {
		return nil, err
	}
	terminal, _, refusalResult, err = s.prepareFencedRemoval(ctx, workspace, item, commonDir, mode)
	if terminal {
		restoreReady = false
	}
	if terminal || err != nil || refusalResult != nil {
		return refusalResult, err
	}

	// Once Git starts deleting, a client disconnect must not strand the record
	// in removing: the deletion and its bookkeeping finish together.
	completionCtx := context.WithoutCancel(ctx)
	removedDespiteError, refusalResult, err := s.runFencedRemoval(
		completionCtx, workspace.Root, item, commonDir, mode.force,
	)
	if removedDespiteError {
		restoreReady = false
	}
	if err != nil || refusalResult != nil {
		return refusalResult, err
	}

	restoreReady = false
	return nil, s.finishRemoval(completionCtx, workspace, item, mode)
}

func (s *Service) readFencedRemovalTarget(ctx context.Context, item Worktree) (Worktree, error) {
	current, err := s.store.Get(ctx, item.WorkspaceID, item.ID)
	if errors.Is(err, ErrNotFound) || current == nil {
		return Worktree{}, ErrNotFound
	}
	if err != nil {
		return Worktree{}, err
	}
	if current.State != StateRemoving {
		return Worktree{}, ErrNotReady
	}
	return *current, nil
}

func (s *Service) requireFencedRemovalTarget(
	ctx context.Context,
	item Worktree,
	final bool,
) (Worktree, error) {
	current, err := s.readFencedRemovalTarget(ctx, item)
	if err == nil {
		return current, nil
	}
	if final {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotReady) {
			return Worktree{}, ErrNotReady
		}
		return Worktree{}, fmt.Errorf("worktree: final removal target read: %w", err)
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotReady) {
		return Worktree{}, err
	}
	return Worktree{}, fmt.Errorf("worktree: reread fenced removal target: %w", err)
}

func (s *Service) runFencedRemoval(
	ctx context.Context,
	workspaceRoot string,
	item Worktree,
	commonDir string,
	force bool,
) (bool, *RemovalRefusal, error) {
	presence, orphan, err := s.validateRemovalIdentity(ctx, item, commonDir)
	if err != nil {
		return false, nil, err
	}
	if presence == checkoutOrphaned {
		return false, nil, s.removeOrphanedCheckout(ctx, workspaceRoot, commonDir, item.Path, orphan, force)
	}
	if presence == checkoutAbsent {
		return false, nil, s.removeAbsentCheckout(ctx, workspaceRoot, commonDir, item, force)
	}
	args := []string{gitDirFlag, commonDir, worktreeDescriptorName, exitRemoveAction}
	if force {
		args = append(args, "--force")
	}
	args = append(args, item.Path)
	_, stderr, removeErr := s.runCheckoutRemoval(ctx, workspaceRoot, args...)
	if removeErr == nil {
		return false, nil, nil
	}
	detail := diagnostics.RedactAndBound(string(stderr), 2048)
	presence, orphan, identityErr := s.validateRemovalIdentity(ctx, item, commonDir)
	switch {
	case identityErr != nil:
		return true, nil, errors.Join(fmt.Errorf("%w: %s: %v", ErrRemovalFailed, detail, removeErr), identityErr)
	case presence == checkoutAbsent:
		return false, nil, s.removeAbsentCheckout(ctx, workspaceRoot, commonDir, item, force)
	case presence == checkoutOrphaned:
		// Git stopped midway, after deleting the .git link. The record stays
		// ready, and the refusal offers to delete what is left.
		refusalResult, err := s.interruptedRemovalRefusal(
			ctx, workspaceRoot, orphan, fmt.Errorf("%s: %w", detail, removeErr),
		)
		return false, refusalResult, err
	case force:
		// Git documents a second --force as the explicit override for a locked
		// linked worktree. Keep deletion inside Git so its admin record and the
		// checkout directory are removed as one operation.
		retryArgs := append([]string(nil), args...)
		forceIndex := len(retryArgs) - 2
		retryArgs = append(retryArgs[:forceIndex], append([]string{"--force"}, retryArgs[forceIndex:]...)...)
		_, retryStderr, retryErr := s.runCheckoutRemoval(ctx, workspaceRoot, retryArgs...)
		if retryErr != nil {
			return false, nil, fmt.Errorf(
				"%w: forced removal retry: %s: %v",
				ErrRemovalFailed,
				diagnostics.RedactAndBound(string(retryStderr), 2048),
				retryErr,
			)
		}
		return false, nil, nil
	default:
		return false, nil, fmt.Errorf("%w: %s: %v", ErrRemovalFailed, detail, removeErr)
	}
}

// evaluateFencedRemoval revalidates the fenced checkout and its removal risks.
// An absent path completes metadata reconciliation only when no session is active.
func (s *Service) evaluateFencedRemoval(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
	commonDir string,
	mode removalMode,
) (bool, RemovalRisk, *RemovalRefusal, error) {
	presence, orphan, err := s.validateRemovalIdentity(ctx, item, commonDir)
	if err != nil {
		return false, RemovalRisk{}, nil, err
	}
	switch presence {
	case checkoutAbsent:
		if err := s.requireNoActiveSession(ctx, item); err != nil {
			return false, RemovalRisk{}, nil, err
		}
		if err := s.removeAbsentCheckout(ctx, workspace.Root, commonDir, item, mode.force); err != nil {
			return false, RemovalRisk{}, nil, err
		}
		return true, RemovalRisk{}, nil, s.finishRemoval(ctx, workspace, item, mode)
	case checkoutOrphaned:
		if err := s.requireNoActiveSession(ctx, item); err != nil {
			return false, RemovalRisk{}, nil, err
		}
		risk, refusalResult, err := s.evaluateOrphanedRemoval(ctx, workspace.Root, orphan, mode.force)
		return false, risk, refusalResult, err
	default:
		risk, refusalResult, err := s.authoritativeRemovalEvaluation(ctx, item, mode)
		return false, risk, refusalResult, err
	}
}

func (s *Service) authoritativeRemovalEvaluation(
	ctx context.Context,
	item Worktree,
	mode removalMode,
) (RemovalRisk, *RemovalRefusal, error) {
	if err := s.requireNoActiveSession(ctx, item); err != nil {
		return RemovalRisk{}, nil, err
	}
	if mode.keepBranch {
		if err := s.requireNoBoundSession(ctx, item); err != nil {
			return RemovalRisk{}, nil, err
		}
	}
	risk, err := s.removalRisk(ctx, item)
	if err != nil {
		return RemovalRisk{}, nil, fmt.Errorf("%w: %v", ErrSafetyCheckFailed, err)
	}
	classified := risk
	if mode.keepBranch {
		// The kept branch holds them; hooks still see the real count.
		classified.UnpushedCommits = 0
	}
	refusalResult, err := classifyRemovalRisk(classified, mode.force)
	return risk, refusalResult, err
}

func classifyRemovalRisk(risk RemovalRisk, force bool) (*RemovalRefusal, error) {
	if !force && risk.ChangedFiles > 0 {
		return &RemovalRefusal{Code: ErrDirtyRequiresForce.Error(), Risk: risk}, ErrDirtyRequiresForce
	}
	if !force && risk.UnpushedCommits > 0 && !risk.ExistsOnRemote {
		return &RemovalRefusal{Code: ErrUnpushedRequiresForce.Error(), Risk: risk}, ErrUnpushedRequiresForce
	}
	return nil, nil
}

func (s *Service) requireNoActiveSession(ctx context.Context, item Worktree) error {
	if s.sessions == nil {
		return nil
	}
	active, err := s.sessions.HasActiveSession(ctx, item.WorkspaceID, item.ID)
	if err != nil {
		return fmt.Errorf("worktree: inspect active sessions: %w", err)
	}
	if active {
		return ErrSessionActive
	}
	return nil
}

// requireNoBoundSession refuses a keep-branch removal while an unarchived session
// still binds the checkout. It runs inside the removal's exclusive usage, so no
// binding change can commit between this read and the removal.
func (s *Service) requireNoBoundSession(ctx context.Context, item Worktree) error {
	if s.sessions == nil {
		return nil
	}
	guard, ok := s.sessions.(BoundSessionGuard)
	if !ok {
		return errors.New("worktree: bound session guard is unavailable")
	}
	bound, err := guard.HasUnarchivedSession(ctx, item.WorkspaceID, item.ID)
	if err != nil {
		return fmt.Errorf("worktree: inspect bound sessions: %w", err)
	}
	if bound {
		return ErrSessionBound
	}
	return nil
}

func (s *Service) finishRemoval(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
	mode removalMode,
) error {
	if !mode.keepBranch {
		s.reclaimRemovedBranch(ctx, workspace, item)
	}
	if err := s.store.SetState(ctx, item.WorkspaceID, item.ID, StateRemoved, s.now().UTC()); err != nil {
		return fmt.Errorf("worktree: tombstone removal: %w", err)
	}
	item.State = StateRemoved
	s.invalidateDiscovery(item.WorkspaceID)
	s.emit(ctx, EventRemoved, item)
	return nil
}

// reclaimRemovedBranch deletes a runtime-owned branch the removed checkout left unchanged.
func (s *Service) reclaimRemovedBranch(ctx context.Context, workspace Workspace, item Worktree) {
	status, statusErr := s.store.GetStatus(ctx, item.WorkspaceID, item.ID)
	if statusErr != nil {
		s.logger.WarnContext(
			ctx,
			"worktree status unavailable for branch reclamation",
			"error",
			diagnostics.RedactAndBound(statusErr.Error(), 2048),
		)
	}
	if _, err := s.reclaimBranch(ctx, workspace.Root, item, status); err != nil {
		s.logger.WarnContext(
			ctx,
			"worktree branch reclamation skipped",
			"worktree_id",
			item.ID,
			"error",
			diagnostics.RedactAndBound(err.Error(), 2048),
		)
	}
}

// checkoutPresence classifies what a removal finds at the record's path.
type checkoutPresence int

const (
	checkoutAbsent checkoutPresence = iota
	checkoutLive
	checkoutOrphaned
)

func (s *Service) validateRemovalIdentity(
	ctx context.Context,
	item Worktree,
	commonDir string,
) (checkoutPresence, orphanedCheckout, error) {
	if _, err := os.Lstat(item.Path); errors.Is(err, os.ErrNotExist) {
		return checkoutAbsent, orphanedCheckout{}, nil
	} else if err != nil {
		return checkoutLive, orphanedCheckout{}, fmt.Errorf("worktree: inspect removal path: %w", err)
	}
	orphan, orphaned, err := findOrphanedCheckout(item.Path, commonDir)
	if err != nil {
		return checkoutLive, orphanedCheckout{}, fmt.Errorf("%w: %v", ErrRemovalFailed, err)
	}
	if orphaned {
		if orphan.adminGitDir != filepath.Clean(item.GitDir) {
			return checkoutLive, orphanedCheckout{}, refusal(ErrRemovalFailed, "worktree Git identity changed")
		}
		return checkoutOrphaned, orphan, nil
	}
	identity, err := s.validateAdoptionIdentity(ctx, item.Path, commonDir)
	if err != nil {
		return checkoutLive, orphanedCheckout{}, fmt.Errorf("%w: %v", ErrRemovalFailed, err)
	}
	if identity.adminGitDir != filepath.Clean(item.GitDir) {
		return checkoutLive, orphanedCheckout{}, refusal(ErrRemovalFailed, "worktree Git identity changed")
	}
	return checkoutLive, orphanedCheckout{}, nil
}
