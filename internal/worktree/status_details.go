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

func (s *Service) requestPullRequestRefresh(ctx context.Context, item Worktree) {
	if s.pullRequests == nil {
		return
	}
	key, err := s.displayPullRequestKey(ctx, item)
	if err == nil {
		s.pullRequests.RequestRefresh(key)
	}
}
