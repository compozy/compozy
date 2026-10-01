package worktree

import (
	"context"
	"errors"
	"os"
	"strings"
)

func (s *Service) VerifyReviewCandidate(
	ctx context.Context,
	workspaceID, worktreeID string,
	paths []string,
	fingerprint string,
) (bool, error) {
	item, err := s.store.Get(ctx, workspaceID, worktreeID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if item == nil || item.ID != worktreeID || item.State != StateReady {
		return false, nil
	}
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return false, err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return false, err
	}
	defer release()
	current, err := s.store.Get(ctx, workspaceID, worktreeID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if current == nil || current.State != StateReady || current.Path != item.Path || current.Branch != item.Branch {
		return false, nil
	}
	matches, err := s.reviewCandidateRepositoryMatches(ctx, *item, commonDir)
	if err != nil || !matches {
		return false, err
	}

	scope, err := s.selectedCommitScope(ctx, item.Path, paths)
	if errors.Is(err, ErrExitActionInvalid) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return scope.Fingerprint == fingerprint, nil
}

func (s *Service) reviewCandidateRepositoryMatches(
	ctx context.Context, item Worktree, commonDir string,
) (bool, error) {
	actualCommonDir, err := s.commonDir(ctx, item.Path)
	if errors.Is(err, ErrWorkspaceNotGitBacked) || errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if actualCommonDir != commonDir {
		return false, nil
	}
	branch, _, err := s.runner.Run(ctx, item.Path, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(string(branch)) != item.Branch {
		return false, nil
	}

	return true, nil
}
