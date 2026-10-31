package session

import "context"

type BranchSnapshotReader interface {
	SessionBranches(context.Context, []string) (map[string]string, error)
}

// BranchSnapshotSaver records the branch a session currently has checked out, so a
// branch created during a turn is visible to delivery checks before the turn-end refresh.
type BranchSnapshotSaver func(context.Context, *Info) error

type PullRequestProjection struct {
	Branch string
	PullRequestView
}

// ConfigurePullRequests connects the optional read model at daemon composition.
func (m *Manager) ConfigurePullRequests(reader PullRequestReader, branches BranchSnapshotReader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pullRequestReader = reader
	m.branchSnapshots = branches
}

// ConfigureBranchSnapshotSaver connects the optional branch snapshot writer at daemon composition.
func (m *Manager) ConfigureBranchSnapshotSaver(save BranchSnapshotSaver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.branchSnapshotSaver = save
}

func (m *Manager) saveSessionBranch(ctx context.Context, info *Info) error {
	m.mu.RLock()
	save := m.branchSnapshotSaver
	m.mu.RUnlock()
	if save == nil || info == nil {
		return nil
	}
	return save(ctx, info)
}

func (m *Manager) SessionPullRequests(ctx context.Context, infos []*Info) (map[string]PullRequestProjection, error) {
	m.mu.RLock()
	reader, branches := m.pullRequestReader, m.branchSnapshots
	m.mu.RUnlock()
	out := make(map[string]PullRequestProjection, len(infos))
	if reader == nil || branches == nil {
		return out, nil
	}
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		if info != nil {
			ids = append(ids, info.ID)
		}
	}
	snapshots, err := branches.SessionBranches(ctx, ids)
	if err != nil {
		return nil, err
	}
	keys := make([]BranchKey, 0, len(infos))
	seen := make(map[BranchKey]bool)
	for _, info := range infos {
		if info == nil {
			continue
		}
		branch := snapshots[info.ID]
		if branch != "" {
			key := BranchKey{info.WorkspaceID, branch}
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	defaults := map[BranchKey]bool{}
	if filter, ok := reader.(interface {
		DefaultBranches(context.Context, []BranchKey) map[BranchKey]bool
	}); ok {
		defaults = filter.DefaultBranches(ctx, keys)
	}
	filtered := keys[:0]
	for _, key := range keys {
		if !defaults[key] {
			filtered = append(filtered, key)
		}
	}
	keys = filtered
	views := reader.PullRequests(ctx, keys)
	for _, info := range infos {
		if info == nil {
			continue
		}
		branch := snapshots[info.ID]
		if branch != "" && !defaults[BranchKey{info.WorkspaceID, branch}] {
			out[info.ID] = PullRequestProjection{
				Branch:          branch,
				PullRequestView: views[BranchKey{info.WorkspaceID, branch}],
			}
		}
	}
	return out, nil
}

func (m *Manager) RefreshSessionPullRequest(ctx context.Context, info *Info) (PullRequestProjection, error) {
	m.mu.RLock()
	reader, branches := m.pullRequestReader, m.branchSnapshots
	m.mu.RUnlock()
	if reader == nil || branches == nil {
		return PullRequestProjection{}, nil
	}
	snapshots, err := branches.SessionBranches(ctx, []string{info.ID})
	if err != nil {
		return PullRequestProjection{}, err
	}
	branch := snapshots[info.ID]
	if branch == "" {
		return PullRequestProjection{}, nil
	}
	if filter, ok := reader.(interface {
		DefaultBranches(context.Context, []BranchKey) map[BranchKey]bool
	}); ok {
		key := BranchKey{info.WorkspaceID, branch}
		if filter.DefaultBranches(ctx, []BranchKey{key})[key] {
			return PullRequestProjection{}, nil
		}
	}
	refresher, ok := reader.(interface {
		RefreshBranch(context.Context, BranchKey) (PullRequestView, error)
	})
	if !ok {
		return PullRequestProjection{Branch: branch}, nil
	}
	view, err := refresher.RefreshBranch(ctx, BranchKey{info.WorkspaceID, branch})
	return PullRequestProjection{Branch: branch, PullRequestView: view}, err
}

// PublishWorkspaceCatalogChanged wakes catalog and session reads in every affected profile.
func (m *Manager) PublishWorkspaceCatalogChanged(workspaceID string, profiles []string) {
	for _, profileID := range profiles {
		m.publishSessionCatalogEvent(
			CatalogEvent{
				Name:        CatalogEventNameChanged,
				Kind:        CatalogEventLayout,
				ProfileID:   profileID,
				WorkspaceID: workspaceID,
			},
		)
	}
}
