package session

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/store"
)

type LinkedWorkProjection struct {
	Items               []LinkedWorkItem
	Summary             LinkedWorkSummary
	Own                 PullRequestState
	DirectOpen, Pending bool
}

// SessionLinkedWork projects a bounded catalog page and its scoped descendants
// with batch storage/cache reads. No forge request is made per item.
func (m *Manager) SessionLinkedWork(ctx context.Context, infos []*Info) (map[string]LinkedWorkProjection, error) {
	m.mu.RLock()
	links, ok := m.branchSnapshots.(WorkLinkStore)
	branches := m.branchSnapshots
	reader := m.pullRequestReader
	m.mu.RUnlock()
	out := make(map[string]LinkedWorkProjection, len(infos))
	if !ok || reader == nil {
		return out, nil
	}
	ids := make([]string, 0, len(infos))
	roots := make(map[string]*Info)
	for _, info := range infos {
		if info != nil {
			ids = append(ids, info.ID)
			roots[info.ID] = info
		}
	}
	family, err := links.SessionWorkFamily(ctx, ids)
	if err != nil {
		return nil, err
	}
	all := make([]string, 0, len(family))
	for _, member := range family {
		all = append(all, member.ID)
	}
	stored, err := links.SessionWorkLinks(ctx, all)
	if err != nil {
		return nil, err
	}
	snapshots, err := branches.SessionBranches(ctx, all)
	if err != nil {
		return nil, err
	}
	pending, err := links.SessionPendingWork(ctx, ids)
	if err != nil {
		return nil, err
	}
	type repositoryScope struct{ profile, workspace string }
	repos := map[repositoryScope]string{}
	resolved := map[repositoryScope]bool{}
	if r, ok := reader.(PullRequestRepositoryReader); ok {
		for _, info := range infos {
			if info != nil {
				scope := repositoryScope{info.ProfileID, info.WorkspaceID}
				if !resolved[scope] {
					repos[scope] = r.RepositoryKeys(ctx, store.ReadScope{ProfileID: info.ProfileID}, []string{info.WorkspaceID})[info.WorkspaceID]
					resolved[scope] = true
				}
			}
		}
	}
	keys := []BranchKey{}
	seen := map[BranchKey]bool{}
	explicit := map[BranchKey]bool{}
	for _, member := range family {
		for _, link := range stored[member.ID] {
			if !link.Unlinked && link.Repository == repos[repositoryScope{member.ProfileID, member.WorkspaceID}] {
				key := BranchKey{member.WorkspaceID, link.Branch}
				explicit[key] = true
				if !seen[key] {
					seen[key] = true
					keys = append(keys, key)
				}
			}
		}
		if branch := snapshots[member.ID]; branch != "" {
			key := BranchKey{member.WorkspaceID, branch}
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	defaults := map[BranchKey]bool{}
	if r, ok := reader.(interface {
		DefaultBranches(context.Context, []BranchKey) map[BranchKey]bool
	}); ok {
		defaults = r.DefaultBranches(ctx, keys)
	}
	filtered := make([]BranchKey, 0, len(keys))
	for _, key := range keys {
		if !defaults[key] || explicit[key] {
			filtered = append(filtered, key)
		}
	}
	views := reader.PullRequests(ctx, filtered)
	for _, member := range family {
		root := roots[member.RootID]
		if root == nil || root.WorkspaceID != member.WorkspaceID || root.ProfileID != member.ProfileID {
			continue
		}
		direct := member.ID == root.ID
		currentWorktreeID := member.WorktreeID
		if currentWorktreeID == "" && direct {
			currentWorktreeID = root.WorktreeID
		}
		items := stored[member.ID]
		branch := snapshots[member.ID]
		found := false
		for _, item := range items {
			if item.Branch == branch &&
				item.Repository == repos[repositoryScope{member.ProfileID, member.WorkspaceID}] {
				found = true
			}
		}
		if branch != "" && !found && (!defaults[BranchKey{member.WorkspaceID, branch}] || member.WorktreeID != "") {
			items = append(
				append([]store.SessionWorkLink{}, items...),
				store.SessionWorkLink{
					Repository:   repos[repositoryScope{member.ProfileID, member.WorkspaceID}],
					Branch:       branch,
					WorktreeID:   member.WorktreeID,
					WorktreeName: member.WorktreeName,
					Provenance:   "created",
				},
			)
		}
		projection := out[root.ID]
		projection.Pending = pending[root.ID]
		if direct && !defaults[BranchKey{member.WorkspaceID, branch}] {
			if pr := views[BranchKey{member.WorkspaceID, branch}].PullRequest; pr != nil {
				projection.Own = pr.State
			}
		}
		for _, link := range items {
			if link.Unlinked {
				continue
			}
			view := views[BranchKey{member.WorkspaceID, link.Branch}]
			if !found && link.Branch == branch && defaults[BranchKey{member.WorkspaceID, branch}] {
				view = PullRequestView{Lookup: PullRequestLookup{State: LookupOK}}
			}
			if link.PRNumber > 0 && view.PullRequest != nil && view.PullRequest.Number != link.PRNumber {
				view = PullRequestView{
					Lookup: PullRequestLookup{State: LookupError, Cause: "pr_identity_changed", Stale: true},
				}
			}
			if link.Repository != repos[repositoryScope{member.ProfileID, member.WorkspaceID}] {
				view = PullRequestView{
					Lookup: PullRequestLookup{State: LookupError, Cause: "repository_changed", Stale: true},
				}
			}
			if !direct && view.PullRequest == nil && link.PRNumber == 0 {
				continue
			}
			item := LinkedWorkItem{
				Repository:      link.Repository,
				Branch:          link.Branch,
				PRNumber:        link.PRNumber,
				WorktreeID:      link.WorktreeID,
				WorktreeName:    link.WorktreeName,
				WorktreeRemoved: link.WorktreeRemoved,
				Provenance:      link.Provenance,
				RunsHere:        direct && currentWorktreeID != "" && currentWorktreeID == link.WorktreeID,
				PullRequestView: view,
			}
			if !direct {
				item.SourceSessionID = member.ID
				item.SourceSessionTitle = member.Name
			}
			if direct && view.PullRequest != nil && view.PullRequest.State == PullRequestOpen {
				projection.DirectOpen = true
			}
			projection.Items = append(projection.Items, item)
		}
		out[root.ID] = projection
	}
	for id, p := range out {
		slices.SortStableFunc(p.Items, func(a, b LinkedWorkItem) int {
			if (a.SourceSessionID == "") != (b.SourceSessionID == "") {
				if a.SourceSessionID == "" {
					return -1
				}
				return 1
			}
			return cmp.Compare(a.Branch, b.Branch)
		})
		unique := make([]LinkedWorkItem, 0, len(p.Items))
		dedupe := map[string]bool{}
		for _, item := range p.Items {
			key := item.Repository + ":" + item.Branch
			if item.PRNumber > 0 {
				key = fmt.Sprintf("%s:#%d", item.Repository, item.PRNumber)
			}
			if item.PullRequest != nil {
				key = fmt.Sprintf("%s:#%d", item.Repository, item.PullRequest.Number)
			}
			if !dedupe[key] {
				dedupe[key] = true
				unique = append(unique, item)
			}
		}
		slices.SortStableFunc(unique, func(a, b LinkedWorkItem) int {
			if rank := cmp.Compare(linkedWorkUrgency(a), linkedWorkUrgency(b)); rank != 0 {
				return rank
			}
			if a.RunsHere != b.RunsHere {
				if a.RunsHere {
					return -1
				}
				return 1
			}
			if (a.SourceSessionID == "") != (b.SourceSessionID == "") {
				if a.SourceSessionID == "" {
					return -1
				}
				return 1
			}
			return 0
		})
		p.Items = unique
		p.Summary.Total = len(unique)
		for _, item := range unique {
			if item.PullRequest != nil || item.PRNumber > 0 {
				p.Summary.PullRequests++
			}
			if linkedWorkUrgency(item) == 0 {
				p.Summary.NeedsYou++
			}
		}
		if len(unique) > 0 {
			lead := unique[0]
			p.Summary.Lead = &lead
		}
		out[id] = p
	}
	return out, nil
}

func linkedWorkUrgency(item LinkedWorkItem) int {
	pr := item.PullRequest
	if pr != nil && (pr.State == PullRequestMerged || pr.State == PullRequestClosed) {
		return 4
	}
	if item.Lookup.State != LookupOK || item.Lookup.Stale {
		return 3
	}
	if pr != nil && pr.State == PullRequestOpen {
		if pr.Checks.State == ChecksFailing || pr.ReviewDecision == ReviewChangesRequested ||
			pr.Mergeable == MergeableConflicting {
			return 0
		}
		if pr.ReadyToMerge && pr.Draft != nil && !*pr.Draft {
			return 1
		}
	}
	return 2
}
