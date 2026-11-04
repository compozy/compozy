package worktree

import (
	"context"
	"maps"
)

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
	if refreshForge && s.pullRequests != nil && item.Branch != "" && displayRefreshes(item.State) {
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
	read := s.pullRequests.Peek
	if displayRefreshes(item.State) {
		read = s.pullRequests.Read
	}
	return read(ctx, []PullRequestKey{key})[key].Facts
}

// hydrateDisplayPullRequests shares workspace resolution only within this read.
// Each later read resolves the current root and retains the PR cache's freshness policy.
func (s *Service) hydrateDisplayPullRequests(ctx context.Context, inspections []Inspection) {
	if s.pullRequests == nil || len(inspections) == 0 {
		return
	}
	repositories := make(map[string]string)
	keys := make([]PullRequestKey, 0, len(inspections))
	active := make(map[PullRequestKey]bool)
	for _, inspection := range inspections {
		item := inspection.Worktree
		if item.Branch == "" {
			continue
		}
		repo, resolved := repositories[item.WorkspaceID]
		if !resolved {
			workspace, err := s.resolveWorkspace(ctx, item.WorkspaceID)
			if err != nil {
				continue
			}
			repo, err = s.pullRequests.RepoKey(ctx, workspace.Root)
			if err != nil {
				continue
			}
			repositories[item.WorkspaceID] = repo
		}
		if repo != "" {
			key := PullRequestKey{item.WorkspaceID, repo, item.Branch}
			keys = append(keys, key)
			if displayRefreshes(item.State) {
				active[key] = true
			}
		}
	}
	if len(keys) == 0 {
		return
	}
	entries := s.readDisplayPullRequests(ctx, keys, active)
	for index := range inspections {
		item := inspections[index].Worktree
		key := PullRequestKey{item.WorkspaceID, repositories[item.WorkspaceID], item.Branch}
		inspections[index].ForgeStatus = entries[key].Facts
	}
}

// displayRefreshes reports whether a list read may refresh a worktree's pull request. Removed,
// dismissed, missing, and failed rows show their last cached facts without contacting the forge.
func displayRefreshes(state State) bool {
	return state == StatePending || state == StateReady
}

// readDisplayPullRequests refreshes stale keys in active and only peeks at the rest. A failed batch
// read falls back to one read per key so one bad row does not blank the list.
func (s *Service) readDisplayPullRequests(
	ctx context.Context,
	keys []PullRequestKey,
	active map[PullRequestKey]bool,
) map[PullRequestKey]PullRequestEntry {
	read := func(batch []PullRequestKey) map[PullRequestKey]PullRequestEntry {
		refresh, peek := make([]PullRequestKey, 0, len(batch)), make([]PullRequestKey, 0, len(batch))
		for _, key := range batch {
			if active[key] {
				refresh = append(refresh, key)
			} else {
				peek = append(peek, key)
			}
		}
		var entries map[PullRequestKey]PullRequestEntry
		if len(refresh) > 0 {
			if entries = s.pullRequests.Read(ctx, refresh); entries == nil {
				return nil
			}
		}
		if len(peek) > 0 {
			peeked := s.pullRequests.Peek(ctx, peek)
			if peeked == nil {
				return nil
			}
			if entries == nil {
				return peeked
			}
			maps.Copy(entries, peeked)
		}
		return entries
	}
	entries := read(keys)
	if entries == nil && len(keys) > 1 && ctx.Err() == nil {
		entries = make(map[PullRequestKey]PullRequestEntry, len(keys))
		for _, key := range keys {
			if ctx.Err() != nil {
				break
			}
			entries[key] = read([]PullRequestKey{key})[key]
		}
	}
	return entries
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
