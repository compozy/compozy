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
	item, err := s.statusTarget(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if !refresh {
		cached, err := s.store.GetStatus(ctx, workspaceID, item.ID)
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

// statusTarget resolves the worktree a status read is about to run Git
// against. A checkout that went missing is refused as ErrMissing before any
// command touches its path or a status row is written for it.
func (s *Service) statusTarget(ctx context.Context, workspaceID, id string) (*Worktree, error) {
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
	return item, nil
}

// refreshStatus is the explicit refresh gesture: it rereads, persists, and
// announces the status so every stream consumer rereads after it.
func (s *Service) refreshStatus(ctx context.Context, item Worktree) (*Status, error) {
	status, err := s.persistStatus(ctx, item)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, EventStatusRefreshed, item)
	return status, nil
}

// persistStatus rereads and persists the status without announcing it. A read
// that refreshes as a side effect must not publish a frame its own readers
// treat as an invalidation, or every read schedules the next one.
func (s *Service) persistStatus(ctx context.Context, item Worktree) (*Status, error) {
	status := s.collectStatus(ctx, item)
	if err := s.store.SaveStatus(ctx, item.WorkspaceID, item.ID, status); err != nil {
		if status.ReadError != "" {
			return nil, fmt.Errorf("worktree: persist status read failure: %w", err)
		}
		return nil, fmt.Errorf("worktree: persist status: %w", err)
	}
	return &status, nil
}

// collectStatus reads the checkout's Git status without persisting it. A Git
// read failure is reported through ReadError so callers keep one result shape.
func (s *Service) collectStatus(ctx context.Context, item Worktree) Status {
	statusOutput, stderr, err := s.runner.Run(ctx, item.Path, "status", "--porcelain=v2", "--branch", "-z")
	if err != nil {
		return s.statusReadFailure(item, stderr, err)
	}
	parsed, err := ParseStatusV2(statusOutput)
	if err != nil {
		return s.statusReadFailure(item, nil, err)
	}
	merged := Numstat{}
	if parsed.DirtyFiles > 0 {
		unstagedOutput, stderr, err := s.runner.Run(ctx, item.Path, "diff", "--numstat", "-z")
		if err != nil {
			return s.statusReadFailure(item, stderr, err)
		}
		stagedOutput, stderr, err := s.runner.Run(ctx, item.Path, "diff", "--cached", "--numstat", "-z")
		if err != nil {
			return s.statusReadFailure(item, stderr, err)
		}
		unstaged, err := ParseNumstat(unstagedOutput)
		if err != nil {
			return s.statusReadFailure(item, nil, err)
		}
		staged, err := ParseNumstat(stagedOutput)
		if err != nil {
			return s.statusReadFailure(item, nil, err)
		}
		merged = MergeNumstat(unstaged, staged)
	}
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
	return Status{
		WorktreeID: item.ID, Branch: &branch, Detached: &detached, HeadSHA: &headSHA,
		DirtyFiles: &dirtyFiles, Insertions: &insertions, Deletions: &deletions,
		HasUpstream: &hasUpstream, Ahead: parsed.Ahead, AheadOfBase: aheadOfBase, Behind: parsed.Behind,
		RefreshedAt: &refreshedAt,
	}
}

func (s *Service) statusReadFailure(item Worktree, stderr []byte, cause error) Status {
	detail := strings.TrimSpace(string(stderr))
	if detail == "" {
		detail = cause.Error()
	}
	refreshedAt := s.now().UTC()
	return Status{
		WorktreeID: item.ID, ReadError: diagnostics.RedactAndBound(detail, setupDiagnosticLimit),
		RefreshedAt: &refreshedAt,
	}
}
