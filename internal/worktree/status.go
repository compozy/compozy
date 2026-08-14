package worktree

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

func (s *Service) Status(ctx context.Context, workspaceID, id string, refresh bool) (*Status, error) {
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || item == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("worktree: read status target: %w", err)
	}
	if item.State == StateMissing {
		return nil, ErrMissing
	}
	if !refresh {
		cached, err := s.store.GetStatus(ctx, workspaceID, id)
		if err != nil {
			return nil, fmt.Errorf("worktree: read cached status: %w", err)
		}
		if cached != nil {
			return cached, nil
		}
		return &Status{WorktreeID: item.ID}, nil
	}
	if _, err := s.capability.Check(ctx); err != nil {
		return nil, err
	}
	return s.refreshStatus(ctx, *item)
}

func (s *Service) refreshStatus(ctx context.Context, item Worktree) (*Status, error) {
	statusOutput, stderr, err := s.runner.Run(ctx, item.Path, "status", "--porcelain=v2", "--branch", "-z")
	if err != nil {
		return s.persistStatusError(ctx, item, stderr, err)
	}
	parsed, err := ParseStatusV2(statusOutput)
	if err != nil {
		return s.persistStatusError(ctx, item, nil, err)
	}
	unstagedOutput, stderr, err := s.runner.Run(ctx, item.Path, "diff", "--numstat", "-z")
	if err != nil {
		return s.persistStatusError(ctx, item, stderr, err)
	}
	stagedOutput, stderr, err := s.runner.Run(ctx, item.Path, "diff", "--cached", "--numstat", "-z")
	if err != nil {
		return s.persistStatusError(ctx, item, stderr, err)
	}
	unstaged, err := ParseNumstat(unstagedOutput)
	if err != nil {
		return s.persistStatusError(ctx, item, nil, err)
	}
	staged, err := ParseNumstat(stagedOutput)
	if err != nil {
		return s.persistStatusError(ctx, item, nil, err)
	}
	merged := MergeNumstat(unstaged, staged)
	var aheadOfBase *int
	if !parsed.HasUpstream && item.BaseRef != "" {
		stdout, _, countErr := s.runner.Run(ctx, item.Path, "rev-list", "--count", item.BaseRef+"..HEAD")
		if countErr == nil {
			if count, parseErr := strconv.Atoi(strings.TrimSpace(string(stdout))); parseErr == nil {
				aheadOfBase = &count
			}
		}
	}
	branch, detached, headSHA := parsed.Branch, parsed.Detached, parsed.HeadSHA
	dirtyFiles, insertions, deletions := parsed.DirtyFiles, merged.Insertions, merged.Deletions
	hasUpstream := parsed.HasUpstream
	refreshedAt := s.now().UTC()
	status := Status{
		WorktreeID: item.ID, Branch: &branch, Detached: &detached, HeadSHA: &headSHA,
		DirtyFiles: &dirtyFiles, Insertions: &insertions, Deletions: &deletions,
		HasUpstream: &hasUpstream, Ahead: parsed.Ahead, AheadOfBase: aheadOfBase, Behind: parsed.Behind,
		RefreshedAt: &refreshedAt,
	}
	if err := s.store.SaveStatus(ctx, item.WorkspaceID, item.ID, status); err != nil {
		return nil, fmt.Errorf("worktree: persist status: %w", err)
	}
	s.emit(ctx, EventStatusRefreshed, item)
	return &status, nil
}

func (s *Service) persistStatusError(
	ctx context.Context,
	item Worktree,
	stderr []byte,
	cause error,
) (*Status, error) {
	detail := strings.TrimSpace(string(stderr))
	if detail == "" {
		detail = cause.Error()
	}
	refreshedAt := s.now().UTC()
	status := Status{
		WorktreeID: item.ID, ReadError: diagnostics.RedactAndBound(detail, setupDiagnosticLimit),
		RefreshedAt: &refreshedAt,
	}
	if err := s.store.SaveStatus(ctx, item.WorkspaceID, item.ID, status); err != nil {
		return nil, fmt.Errorf("worktree: persist status read failure: %w", err)
	}
	s.emit(ctx, EventStatusRefreshed, item)
	return &status, nil
}
