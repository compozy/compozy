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

type repositoryScope struct{ profile, workspace string }

// linkedWorkReads is one page's batch of storage and cache reads.
type linkedWorkReads struct {
	family    []store.SessionWorkMember
	stored    map[string][]store.SessionWorkLink
	snapshots map[string]string
	pending   map[string]bool
	repos     map[repositoryScope]string
	defaults  map[BranchKey]bool
	views     map[BranchKey]PullRequestView
	keys      []BranchKey
}

// SessionLinkedWork projects a bounded catalog page and its scoped descendants
// with batch storage/cache reads. No forge request is made per item.
func (m *Manager) SessionLinkedWork(ctx context.Context, infos []*Info) (map[string]LinkedWorkProjection, error) {
	return m.sessionLinkedWork(ctx, infos, false)
}

func (m *Manager) sessionLinkedWork(
	ctx context.Context,
	infos []*Info,
	refresh bool,
) (map[string]LinkedWorkProjection, error) {
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
	reads, err := readLinkedWork(ctx, links, branches, reader, infos, ids)
	if err != nil {
		return nil, err
	}
	if refresh {
		requestDisplayedLinkedWork(ctx, reader, reads, roots)
	}
	for _, member := range reads.family {
		root := roots[member.RootID]
		if root == nil || root.WorkspaceID != member.WorkspaceID || root.ProfileID != member.ProfileID {
			continue
		}
		out[root.ID] = reads.projectMember(out[root.ID], root, member)
	}
	for id, projection := range out {
		out[id] = finishLinkedWorkProjection(projection)
	}
	return out, nil
}

func readLinkedWork(
	ctx context.Context,
	links WorkLinkStore,
	branches BranchSnapshotReader,
	reader PullRequestReader,
	infos []*Info,
	ids []string,
) (linkedWorkReads, error) {
	family, err := links.SessionWorkFamily(ctx, ids)
	if err != nil {
		return linkedWorkReads{}, err
	}
	all := make([]string, 0, len(family))
	for _, member := range family {
		all = append(all, member.ID)
	}
	stored, err := links.SessionWorkLinks(ctx, all)
	if err != nil {
		return linkedWorkReads{}, err
	}
	snapshots, err := branches.SessionBranches(ctx, all)
	if err != nil {
		return linkedWorkReads{}, err
	}
	pending, err := links.SessionPendingWork(ctx, ids)
	if err != nil {
		return linkedWorkReads{}, err
	}
	reads := linkedWorkReads{
		family: family, stored: stored, snapshots: snapshots, pending: pending,
		repos: linkedWorkRepositories(ctx, reader, infos), defaults: map[BranchKey]bool{},
	}
	keys, explicit := reads.branchKeys()
	if r, ok := reader.(interface {
		DefaultBranches(context.Context, []BranchKey) map[BranchKey]bool
	}); ok {
		reads.defaults = r.DefaultBranches(ctx, keys)
	}
	filtered := make([]BranchKey, 0, len(keys))
	for _, key := range keys {
		if !reads.defaults[key] || explicit[key] {
			filtered = append(filtered, key)
		}
	}
	reads.keys = filtered
	reads.views = reader.PullRequests(ctx, filtered)
	return reads, nil
}

func linkedWorkRepositories(
	ctx context.Context,
	reader PullRequestReader,
	infos []*Info,
) map[repositoryScope]string {
	repos := map[repositoryScope]string{}
	r, ok := reader.(PullRequestRepositoryReader)
	if !ok {
		return repos
	}
	resolved := map[repositoryScope]bool{}
	for _, info := range infos {
		if info == nil {
			continue
		}
		scope := repositoryScope{info.ProfileID, info.WorkspaceID}
		if !resolved[scope] {
			keys := r.RepositoryKeys(ctx, store.ReadScope{ProfileID: info.ProfileID}, []string{info.WorkspaceID})
			repos[scope] = keys[info.WorkspaceID]
			resolved[scope] = true
		}
	}
	return repos
}

func (r linkedWorkReads) repository(member store.SessionWorkMember) string {
	return r.repos[repositoryScope{member.ProfileID, member.WorkspaceID}]
}

// branchKeys lists every branch to read, and which of them a link names explicitly.
func (r linkedWorkReads) branchKeys() ([]BranchKey, map[BranchKey]bool) {
	keys := []BranchKey{}
	seen := map[BranchKey]bool{}
	explicit := map[BranchKey]bool{}
	add := func(key BranchKey) {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for _, member := range r.family {
		for _, link := range r.stored[member.ID] {
			if !link.Unlinked && link.Repository == r.repository(member) {
				key := BranchKey{member.WorkspaceID, link.Branch}
				explicit[key] = true
				add(key)
			}
		}
		if branch := r.snapshots[member.ID]; branch != "" {
			add(BranchKey{member.WorkspaceID, branch})
		}
	}
	return keys, explicit
}

// memberLinks adds the member's own branch when no stored link names it.
func (r linkedWorkReads) memberLinks(member store.SessionWorkMember) ([]store.SessionWorkLink, bool) {
	items := r.stored[member.ID]
	branch := r.snapshots[member.ID]
	found := slices.ContainsFunc(items, func(item store.SessionWorkLink) bool {
		return item.Branch == branch && item.Repository == r.repository(member)
	})
	if branch == "" || found || r.defaults[BranchKey{member.WorkspaceID, branch}] && member.WorktreeID == "" {
		return items, found
	}
	return append(append([]store.SessionWorkLink{}, items...), store.SessionWorkLink{
		Repository:   r.repository(member),
		Branch:       branch,
		WorktreeID:   member.WorktreeID,
		WorktreeName: member.WorktreeName,
		Provenance:   LinkProvenanceCreated,
	}), found
}

// linkView is the PR facts a link may show: none that belong to another PR or repository.
func (r linkedWorkReads) linkView(
	member store.SessionWorkMember,
	link store.SessionWorkLink,
	derivedDefault bool,
) PullRequestView {
	view := r.views[BranchKey{member.WorkspaceID, link.Branch}]
	if derivedDefault {
		view = PullRequestView{Lookup: PullRequestLookup{State: LookupOK}}
	}
	if link.PRNumber > 0 && view.PullRequest != nil && view.PullRequest.Number != link.PRNumber {
		view = PullRequestView{
			Lookup: PullRequestLookup{State: LookupError, Cause: "pr_identity_changed", Stale: true},
		}
	}
	if link.Repository != r.repository(member) {
		view = PullRequestView{
			Lookup: PullRequestLookup{State: LookupError, Cause: "repository_changed", Stale: true},
		}
	}
	return view
}

func (r linkedWorkReads) projectMember(
	projection LinkedWorkProjection,
	root *Info,
	member store.SessionWorkMember,
) LinkedWorkProjection {
	direct := member.ID == root.ID
	currentWorktreeID := member.WorktreeID
	if currentWorktreeID == "" && direct {
		currentWorktreeID = root.WorktreeID
	}
	branch := r.snapshots[member.ID]
	ownKey := BranchKey{member.WorkspaceID, branch}
	items, found := r.memberLinks(member)
	projection.Pending = r.pending[root.ID]
	if direct && !r.defaults[ownKey] {
		if pr := r.views[ownKey].PullRequest; pr != nil {
			projection.Own = pr.State
		}
	}
	for _, link := range items {
		if link.Unlinked {
			continue
		}
		view := r.linkView(member, link, !found && link.Branch == branch && r.defaults[ownKey])
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
	return projection
}

func compareLinkedWorkOwner(a, b LinkedWorkItem) int {
	if (a.SourceSessionID == "") == (b.SourceSessionID == "") {
		return 0
	}
	if a.SourceSessionID == "" {
		return -1
	}
	return 1
}

// uniqueLinkedWork keeps the first item for each PR, or for each branch without one.
func uniqueLinkedWork(items []LinkedWorkItem) []LinkedWorkItem {
	unique := make([]LinkedWorkItem, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		key := item.Repository + ":" + item.Branch
		if item.PRNumber > 0 {
			key = fmt.Sprintf("%s:#%d", item.Repository, item.PRNumber)
		}
		if item.PullRequest != nil {
			key = fmt.Sprintf("%s:#%d", item.Repository, item.PullRequest.Number)
		}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, item)
		}
	}
	return unique
}

func finishLinkedWorkProjection(p LinkedWorkProjection) LinkedWorkProjection {
	slices.SortStableFunc(p.Items, func(a, b LinkedWorkItem) int {
		if owner := compareLinkedWorkOwner(a, b); owner != 0 {
			return owner
		}
		return cmp.Compare(a.Branch, b.Branch)
	})
	unique := uniqueLinkedWork(p.Items)
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
		return compareLinkedWorkOwner(a, b)
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
	return p
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
