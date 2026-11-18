package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

type RemovalTarget struct {
	Ref         string
	Path        string
	ExpectStale bool
}

// PreviewRemoval measures current risk without hooks, teardown, fencing, or checkout mutation.
func (s *Service) PreviewRemoval(
	ctx context.Context, workspaceID string, target RemovalTarget, force bool,
) (RemovalRisk, *RemovalRefusal, error) {
	if target.Path != "" {
		return s.previewDiscoveredRemoval(ctx, workspaceID, target, force)
	}
	item, err := s.store.Get(ctx, workspaceID, strings.TrimSpace(target.Ref))
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	if item == nil {
		return RemovalRisk{}, nil, ErrNotFound
	}
	releaseUsage, acquired := s.usage.tryAcquireExclusive(worktreeUsageKey(workspaceID, item.ID))
	if !acquired {
		return RemovalRisk{}, nil, ErrOperationInProgress
	}
	defer releaseUsage()
	item, err = s.store.Get(ctx, workspaceID, item.ID)
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	if item == nil {
		return RemovalRisk{}, nil, ErrNotFound
	}
	if item.State == StateRemoved || item.State == StateDismissed {
		return RemovalRisk{}, nil, nil
	}
	if item.State != StateReady && item.State != StateMissing {
		return RemovalRisk{}, nil, ErrNotReady
	}
	if item.State == StateMissing {
		return s.previewMissingRemoval(ctx, *item)
	}
	return s.previewRegisteredRemoval(ctx, *item, force)
}

func (s *Service) previewRegisteredRemoval(
	ctx context.Context, item Worktree, force bool,
) (RemovalRisk, *RemovalRefusal, error) {
	workspace, err := s.resolveProfileWorkspace(ctx, item.WorkspaceID, item.ProfileID)
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	if err := s.requireNoActiveSession(ctx, item); err != nil {
		return RemovalRisk{}, nil, err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	defer release()
	presence, orphan, err := s.validateRemovalIdentity(ctx, item, commonDir)
	if err != nil || presence == checkoutAbsent {
		return RemovalRisk{}, nil, err
	}
	if presence == checkoutOrphaned {
		return s.evaluateOrphanedRemoval(ctx, workspace.Root, orphan, force)
	}
	if _, err := checkoutRemovalArgs(commonDir, item.Path, item.GitDir, force); err != nil {
		return RemovalRisk{}, nil, err
	}
	return s.previewLiveRemoval(ctx, item, force)
}

func (s *Service) previewLiveRemoval(
	ctx context.Context, item Worktree, force bool,
) (RemovalRisk, *RemovalRefusal, error) {
	status := s.collectStatus(ctx, item)
	risk, err := s.removalRiskFromStatus(ctx, item, &status)
	if err != nil {
		return risk, nil, fmt.Errorf("%w: %v", ErrSafetyCheckFailed, err)
	}
	refusalResult, err := classifyRemovalRisk(risk, force)
	return risk, refusalResult, err
}

func (s *Service) previewDiscoveredRemoval(
	ctx context.Context, workspaceID string, request RemovalTarget, force bool,
) (RemovalRisk, *RemovalRefusal, error) {
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	defer release()
	target, err := s.readDiscoveredRemovalTarget(ctx, workspaceID, workspace, commonDir, request.Path)
	if err != nil {
		return RemovalRisk{}, nil, err
	}
	if request.ExpectStale && !target.stale {
		return RemovalRisk{}, nil, ErrDiscoveredChanged
	}
	if target.orphaned {
		return s.evaluateOrphanedRemoval(ctx, workspace.Root, target.orphan, force)
	}
	if target.stale {
		return RemovalRisk{}, nil, nil
	}
	_, risk, refusalResult, err := s.evaluateDiscoveredRemoval(ctx, workspaceID, commonDir, target, force)
	return risk, refusalResult, err
}

func (s *Service) previewMissingRemoval(ctx context.Context, item Worktree) (RemovalRisk, *RemovalRefusal, error) {
	if _, err := os.Lstat(item.Path); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return RemovalRisk{}, nil, err
		}
		return RemovalRisk{}, nil, ErrNotReady
	}
	return RemovalRisk{}, nil, s.requireNoBoundSession(ctx, item)
}
