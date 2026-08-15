package worktree

import (
	"context"
	"fmt"
)

type AgentActivity string

const (
	AgentActivityIdle    AgentActivity = "idle"
	AgentActivityRunning AgentActivity = "running"
)

type Inspection struct {
	Worktree      Worktree
	Status        *Status
	ForgeStatus   *ForgeStatus
	Repo          RepoState
	AgentActivity AgentActivity
}

type DetailedListing struct {
	Worktrees  []Inspection
	Discovered []DiscoveredWorktree
	Repo       RepoState
}

// Inspect returns durable registry and cached derived state. It probes only
// repository capability; status, discovery, and forge reads remain cached.
func (s *Service) Inspect(ctx context.Context, workspaceID, ref string) (*Inspection, error) {
	item, err := s.Resolve(ctx, workspaceID, ref)
	if err != nil {
		return nil, err
	}
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	repo, err := s.repoState(ctx, workspace)
	if err != nil {
		return nil, err
	}
	inspection, err := s.inspectItem(ctx, *item)
	if err != nil {
		return nil, err
	}
	inspection.Repo = repo
	return &inspection, nil
}

// ListDetails returns the registry/discovery merge plus cached status, forge,
// and live activity. Git status is refreshed only when refresh is explicit.
func (s *Service) ListDetails(
	ctx context.Context,
	workspaceID string,
	refresh bool,
) (*DetailedListing, error) {
	listing, err := s.List(ctx, workspaceID, refresh)
	if err != nil {
		return nil, err
	}
	detailed := &DetailedListing{
		Worktrees:  make([]Inspection, 0, len(listing.Worktrees)),
		Discovered: listing.Discovered,
		Repo:       listing.Repo,
	}
	for _, item := range listing.Worktrees {
		inspection, inspectErr := s.inspectItem(ctx, item)
		if inspectErr != nil {
			return nil, inspectErr
		}
		if refresh && item.State == StateReady {
			status, statusErr := s.Status(ctx, item.WorkspaceID, item.ID, true)
			if statusErr != nil {
				return nil, statusErr
			}
			inspection.Status = status
		}
		detailed.Worktrees = append(detailed.Worktrees, inspection)
	}
	return detailed, nil
}

func (s *Service) inspectItem(ctx context.Context, item Worktree) (Inspection, error) {
	status, err := s.store.GetStatus(ctx, item.WorkspaceID, item.ID)
	if err != nil {
		return Inspection{}, fmt.Errorf("worktree: inspect cached status: %w", err)
	}
	forgeStatus, err := s.store.GetForgeStatus(ctx, item.WorkspaceID, item.ID)
	if err != nil {
		return Inspection{}, fmt.Errorf("worktree: inspect cached forge status: %w", err)
	}
	activity := AgentActivityIdle
	if s.sessions != nil {
		active, activeErr := s.sessions.HasActiveSession(ctx, item.WorkspaceID, item.ID)
		if activeErr != nil {
			return Inspection{}, fmt.Errorf("worktree: inspect session activity: %w", activeErr)
		}
		if active {
			activity = AgentActivityRunning
		}
	}
	return Inspection{
		Worktree: item, Status: status, ForgeStatus: forgeStatus, AgentActivity: activity,
	}, nil
}
