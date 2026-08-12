package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

type RemovalRisk struct {
	ChangedFiles    int
	Insertions      int
	Deletions       int
	UnpushedCommits int
	ExistsOnRemote  bool
}

type RemovalRefusal struct {
	Code      string
	Risk      RemovalRisk
	Downgrade bool
}

type HookPreRemovePayload struct {
	Worktree HookWorktree `json:"worktree"`
	Force    bool         `json:"force"`
	Risk     RemovalRisk  `json:"risk"`
}

func (s *Service) Remove(
	ctx context.Context,
	workspaceID string,
	id string,
	force bool,
) (*RemovalRefusal, error) {
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || item == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("worktree: read removal target: %w", err)
	}
	if item.State == StateRemoved {
		return nil, nil
	}
	if item.State != StateReady {
		return nil, ErrNotReady
	}
	if _, err := s.capability.Check(ctx); err != nil {
		return nil, err
	}
	swapped, err := s.store.CompareAndSwapState(
		ctx, workspaceID, id, StateReady, StateRemoving, s.now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("worktree: fence removal: %w", err)
	}
	if !swapped {
		return nil, ErrNotReady
	}
	return s.removeFenced(ctx, *item, force)
}

func (s *Service) removeFenced(ctx context.Context, item Worktree, force bool) (_ *RemovalRefusal, err error) {
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

	workspace, err := s.resolveWorkspace(ctx, item.WorkspaceID)
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

	current, err := s.store.Get(ctx, item.WorkspaceID, item.ID)
	if errors.Is(err, ErrNotFound) || current == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("worktree: reread fenced removal target: %w", err)
	}
	if current.State != StateRemoving {
		return nil, ErrNotReady
	}
	item = *current
	pathPresent, err := s.validateRemovalIdentity(ctx, item, commonDir)
	if err != nil {
		return nil, err
	}
	if !pathPresent {
		restoreReady = false
		return nil, s.finishRemoval(ctx, workspace, item)
	}
	risk, refusalResult, err := s.authoritativeRemovalEvaluation(ctx, item, force)
	if err != nil || refusalResult != nil {
		return refusalResult, err
	}
	if err := s.dispatchGate(ctx, EventPreRemove, HookPreRemovePayload{
		Worktree: eventPayloadFromWorktree(item, workspace.Root), Force: force, Risk: risk,
	}); err != nil {
		return nil, err
	}

	// Hooks can run arbitrary code. Close that final race before deleting.
	current, err = s.store.Get(ctx, item.WorkspaceID, item.ID)
	if err != nil || current == nil || current.State != StateRemoving {
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("worktree: final removal target read: %w", err)
		}
		return nil, ErrNotReady
	}
	item = *current
	pathPresent, err = s.validateRemovalIdentity(ctx, item, commonDir)
	if err != nil {
		return nil, err
	}
	if !pathPresent {
		restoreReady = false
		return nil, s.finishRemoval(ctx, workspace, item)
	}
	_, refusalResult, err = s.authoritativeRemovalEvaluation(ctx, item, force)
	if err != nil || refusalResult != nil {
		return refusalResult, err
	}

	args := []string{"--git-dir", commonDir, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, item.Path)
	_, stderr, removeErr := s.runner.Run(ctx, workspace.Root, args...)
	if removeErr != nil {
		stillPresent, identityErr := s.validateRemovalIdentity(ctx, item, commonDir)
		switch {
		case identityErr != nil:
			restoreReady = false
			return nil, errors.Join(
				fmt.Errorf(
					"%w: %s: %v",
					ErrRemovalFailed, diagnostics.RedactAndBound(string(stderr), 2048), removeErr,
				),
				identityErr,
			)
		case !stillPresent:
			// Git returned an error after removing the checkout. Complete the
			// durable tombstone instead of resurrecting a ready row.
		case force:
			if cleanupErr := s.removeVerifiedLeftover(ctx, item.Path, commonDir, item.GitDir); cleanupErr != nil {
				return nil, fmt.Errorf(
					"%w: %s: %v: %v",
					ErrRemovalFailed, diagnostics.RedactAndBound(string(stderr), 2048), removeErr, cleanupErr,
				)
			}
		default:
			return nil, fmt.Errorf(
				"%w: %s: %v",
				ErrRemovalFailed, diagnostics.RedactAndBound(string(stderr), 2048), removeErr,
			)
		}
	}

	restoreReady = false
	return nil, s.finishRemoval(ctx, workspace, item)
}

func (s *Service) authoritativeRemovalEvaluation(
	ctx context.Context,
	item Worktree,
	force bool,
) (RemovalRisk, *RemovalRefusal, error) {
	if err := s.requireNoActiveSession(ctx, item); err != nil {
		return RemovalRisk{}, nil, err
	}
	risk, err := s.removalRisk(ctx, item)
	if err != nil {
		return RemovalRisk{}, nil, fmt.Errorf("%w: %v", ErrSafetyCheckFailed, err)
	}
	refusalResult, err := classifyRemovalRisk(risk, force)
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

func (s *Service) finishRemoval(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
) error {
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
	if err := s.store.SetState(ctx, item.WorkspaceID, item.ID, StateRemoved, s.now().UTC()); err != nil {
		return fmt.Errorf("worktree: tombstone removal: %w", err)
	}
	item.State = StateRemoved
	s.invalidateDiscovery(item.WorkspaceID)
	s.emit(ctx, EventRemoved, item)
	return nil
}

func (s *Service) removalRisk(ctx context.Context, item Worktree) (RemovalRisk, error) {
	status, err := s.refreshStatus(ctx, item)
	if err != nil {
		return RemovalRisk{}, err
	}
	if status.ReadError != "" {
		return RemovalRisk{}, refusal(ErrStatusUnreadable, status.ReadError)
	}
	risk := RemovalRisk{
		ChangedFiles: valueOrZero(status.DirtyFiles), Insertions: valueOrZero(status.Insertions),
		Deletions: valueOrZero(status.Deletions),
	}
	comparison := ""
	if item.BaseRef != "" {
		comparison = item.BaseRef + "..HEAD"
	} else if status.Detached != nil && *status.Detached {
		comparison = "HEAD --not --all"
	} else if status.HasUpstream != nil && *status.HasUpstream {
		comparison = "@{upstream}..HEAD"
	}
	if comparison == "" {
		return risk, nil
	}
	args := []string{"rev-list", "--count"}
	args = append(args, strings.Fields(comparison)...)
	stdout, stderr, err := s.runner.Run(ctx, item.Path, args...)
	if err != nil {
		return RemovalRisk{}, fmt.Errorf(
			"count unique commits: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	risk.UnpushedCommits, err = strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return RemovalRisk{}, fmt.Errorf("parse unique commit count: %w", err)
	}
	if risk.UnpushedCommits == 0 || (status.Detached != nil && *status.Detached) {
		return risk, nil
	}
	stdout, stderr, err = s.runner.Run(ctx, item.Path, "branch", "-r", "--contains", "HEAD")
	if err != nil {
		return RemovalRisk{}, fmt.Errorf(
			"inspect remote containment: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	risk.ExistsOnRemote = strings.TrimSpace(string(stdout)) != ""
	return risk, nil
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func (s *Service) validateRemovalIdentity(
	ctx context.Context,
	item Worktree,
	commonDir string,
) (bool, error) {
	if _, err := os.Lstat(item.Path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("worktree: inspect removal path: %w", err)
	}
	identity, err := s.validateAdoptionIdentity(ctx, item.Path, commonDir)
	if err != nil {
		return true, fmt.Errorf("%w: %v", ErrRemovalFailed, err)
	}
	if identity.adminGitDir != filepath.Clean(item.GitDir) {
		return true, refusal(ErrRemovalFailed, "worktree Git identity changed")
	}
	return true, nil
}

func (s *Service) removeVerifiedLeftover(
	ctx context.Context,
	path string,
	commonDir string,
	expectedGitDir string,
) error {
	identity, err := s.validateAdoptionIdentity(ctx, path, commonDir)
	if err != nil {
		return err
	}
	if identity.adminGitDir != filepath.Clean(expectedGitDir) {
		return refusal(ErrRemovalFailed, "leftover Git identity changed")
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("worktree: remove verified leftover: %w", err)
	}
	return nil
}
