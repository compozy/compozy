package session

import "context"

// SessionDisplayedLinkedWork refreshes the facts that contribute to an authorized visible page.
func (m *Manager) SessionDisplayedLinkedWork(
	ctx context.Context,
	infos []*Info,
) (map[string]LinkedWorkProjection, error) {
	return m.sessionLinkedWork(ctx, infos, true)
}

func requestDisplayedLinkedWork(
	ctx context.Context,
	reader PullRequestReader,
	reads linkedWorkReads,
	roots map[string]*Info,
) {
	requester, ok := reader.(interface {
		RequestPullRequests(context.Context, []BranchKey)
	})
	if !ok {
		return
	}
	eligible := make(map[BranchKey]bool)
	for _, member := range reads.family {
		root := roots[member.RootID]
		if root == nil || root.WorkspaceID != member.WorkspaceID || root.ProfileID != member.ProfileID {
			continue
		}
		if branch := reads.snapshots[member.ID]; branch != "" {
			eligible[BranchKey{member.WorkspaceID, branch}] = true
		}
		for _, link := range reads.stored[member.ID] {
			if !link.Unlinked && !link.WorktreeRemoved && link.Repository == reads.repository(member) {
				eligible[BranchKey{member.WorkspaceID, link.Branch}] = true
			}
		}
	}
	keys := make([]BranchKey, 0, len(reads.keys))
	for _, key := range reads.keys {
		if eligible[key] {
			keys = append(keys, key)
		}
	}
	requester.RequestPullRequests(ctx, keys)
}
