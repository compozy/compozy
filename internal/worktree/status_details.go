package worktree

import "context"

type StatusDetails struct {
	WorktreeID  string
	Status      *Status
	ForgeStatus *ForgeStatus
}

func (s *Service) StatusDetails(
	ctx context.Context,
	workspaceID, id string,
	refresh, refreshForge bool,
) (*StatusDetails, error) {
	item, err := s.store.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrNotFound
	}
	status, err := s.Status(ctx, workspaceID, item.ID, refresh)
	if err != nil {
		return nil, err
	}
	if refreshForge && item.Branch != "" && s.pullRequests == nil {
		return nil, ErrForgeUnavailable
	}
	var facts *ForgeStatus
	if refreshForge && s.pullRequests != nil && item.Branch != "" {
		key, keyErr := s.displayPullRequestKey(ctx, *item)
		if keyErr != nil {
			return nil, keyErr
		}
		entry, refreshErr := s.pullRequests.Refresh(ctx, key)
		if refreshErr != nil && entry.Facts == nil {
			return nil, refreshErr
		}
		facts = entry.Facts
	} else {
		facts = s.readDisplayPullRequest(ctx, *item)
	}
	if err != nil {
		return nil, err
	}
	return &StatusDetails{WorktreeID: item.ID, Status: status, ForgeStatus: facts}, nil
}

func (s *Service) displayPullRequestKey(ctx context.Context, item Worktree) (PullRequestKey, error) {
	workspace, err := s.resolveWorkspace(ctx, item.WorkspaceID)
	if err != nil {
		return PullRequestKey{}, err
	}
	repo, err := s.pullRequests.RepoKey(ctx, workspace.Root)
	return PullRequestKey{WorkspaceID: item.WorkspaceID, RepoKey: repo, Branch: item.Branch}, err
}

func (s *Service) readDisplayPullRequest(ctx context.Context, item Worktree) *ForgeStatus {
	if s.pullRequests == nil || item.Branch == "" {
		return nil
	}
	key, err := s.displayPullRequestKey(ctx, item)
	if err != nil {
		return nil
	}
	entry := s.pullRequests.Read(ctx, []PullRequestKey{key})[key]
	return entry.Facts
}

// hydrateDisplayPullRequests shares workspace resolution only within this read.
// Each later read resolves the current root and retains the PR cache's freshness policy.
func (s *Service) hydrateDisplayPullRequests(ctx context.Context, inspections []Inspection) {
	if s.pullRequests == nil || len(inspections) == 0 {
		return
	}
	repositories := make(map[string]string)
	keys := make([]PullRequestKey, 0, len(inspections))
	for _, inspection := range inspections {
		item := inspection.Worktree
		if item.Branch == "" {
			continue
		}
		repo, resolved := repositories[item.WorkspaceID]
		if !resolved {
			workspace, err := s.resolveWorkspace(ctx, item.WorkspaceID)
			if err == nil {
				if key, keyErr := s.pullRequests.RepoKey(ctx, workspace.Root); keyErr == nil {
					repo = key
				}
			}
			repositories[item.WorkspaceID] = repo
		}
		if repo != "" {
			keys = append(keys, PullRequestKey{item.WorkspaceID, repo, item.Branch})
		}
	}
	if len(keys) == 0 {
		return
	}
	entries := s.pullRequests.Read(ctx, keys)
	for index := range inspections {
		item := inspections[index].Worktree
		key := PullRequestKey{item.WorkspaceID, repositories[item.WorkspaceID], item.Branch}
		inspections[index].ForgeStatus = entries[key].Facts
	}
}

func (s *Service) requestPullRequestRefresh(ctx context.Context, item Worktree) {
	if s.pullRequests == nil {
		return
	}
	key, err := s.displayPullRequestKey(ctx, item)
	if err == nil {
		s.pullRequests.RequestRefresh(key)
	}
}

// ObserveForgeStatus reads fresh delivery evidence without consulting the display cache.
func (s *Service) ObserveForgeStatus(ctx context.Context, workspaceID, id string) (*ForgeStatus, error) {
	item, err := s.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if s.forge == nil {
		return nil, ErrForgeUnavailable
	}
	remotes, err := s.readOriginRemoteURLs(ctx, item.Path)
	if err != nil {
		return nil, err
	}
	return s.forge.Status(ctx, ForgeStatusRequest{
		WorkspaceID: workspaceID, WorktreeID: item.ID, RemoteURLs: remotes, Branch: item.Branch,
	})
}
