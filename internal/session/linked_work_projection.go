package session

import (
	"context"
	"fmt"
	"sort"

	"github.com/compozy/compozy/internal/store"
)

type LinkedWorkProjection struct {
	Items               []LinkedWorkItem
	Summary             LinkedWorkSummary
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
	repos := map[string]string{}
	if r, ok := reader.(PullRequestRepositoryReader); ok {
		for _, info := range infos {
			if info != nil {
				for id, key := range r.RepositoryKeys(ctx, store.ReadScope{ProfileID: info.ProfileID}, []string{info.WorkspaceID}) {
					repos[id] = key
				}
			}
		}
	}
	keys := []BranchKey{}
	seen := map[BranchKey]bool{}
	for _, member := range family {
		for _, link := range stored[member.ID] {
			if !link.Unlinked && link.Repository == repos[member.WorkspaceID] {
				key := BranchKey{member.WorkspaceID, link.Branch}
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
	views := reader.PullRequests(ctx, keys)
	defaults := map[BranchKey]bool{}
	if r, ok := reader.(interface {
		DefaultBranches(context.Context, []BranchKey) map[BranchKey]bool
	}); ok {
		defaults = r.DefaultBranches(ctx, keys)
	}
	for _, member := range family {
		root := roots[member.RootID]
		if root == nil || root.WorkspaceID != member.WorkspaceID || root.ProfileID != member.ProfileID {
			continue
		}
		direct := member.ID == root.ID
		items := stored[member.ID]
		branch := snapshots[member.ID]
		found := false
		for _, item := range items {
			if item.Branch == branch && item.Repository == repos[member.WorkspaceID] {
				found = true
			}
		}
		if branch != "" && !found && (!defaults[BranchKey{member.WorkspaceID, branch}] || member.WorktreeID != "") {
			items = append(append([]store.SessionWorkLink{}, items...), store.SessionWorkLink{Repository: repos[member.WorkspaceID], Branch: branch, WorktreeID: member.WorktreeID, Provenance: "created"})
		}
		projection := out[root.ID]
		projection.Pending = pending[root.ID]
		for _, link := range items {
			if link.Unlinked {
				continue
			}
			view := views[BranchKey{member.WorkspaceID, link.Branch}]
			if link.Repository != repos[member.WorkspaceID] {
				view = PullRequestView{Lookup: PullRequestLookup{State: LookupError, Cause: "repository_changed", Stale: true}}
			}
			item := LinkedWorkItem{Repository: link.Repository, Branch: link.Branch, PRNumber: link.PRNumber, WorktreeID: link.WorktreeID, WorktreeName: link.WorktreeName, WorktreeRemoved: link.WorktreeRemoved, Provenance: link.Provenance, RunsHere: direct && root.WorktreeID != "" && root.WorktreeID == link.WorktreeID, PullRequestView: view}
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
		sort.SliceStable(p.Items, func(i, j int) bool {
			a, b := p.Items[i], p.Items[j]
			if (a.SourceSessionID == "") != (b.SourceSessionID == "") {
				return a.SourceSessionID == ""
			}
			return a.Branch < b.Branch
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
		sort.SliceStable(unique, func(i, j int) bool { return linkedWorkUrgency(unique[i]) < linkedWorkUrgency(unique[j]) })
		p.Items = unique
		p.Summary.Total = len(unique)
		for _, item := range unique {
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
	if item.Lookup.State == LookupError || pr != nil && pr.State == PullRequestOpen && (pr.Checks.State == ChecksFailing || pr.ReviewDecision == ReviewChangesRequested || pr.Mergeable == MergeableConflicting || pr.ReadyToMerge) {
		return 0
	}
	if pr == nil || pr.State == PullRequestOpen {
		return 1
	}
	return 2
}
