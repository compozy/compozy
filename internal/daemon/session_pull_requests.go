package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
)

type sessionPullRequestAdapter struct {
	service  *worktree.PullRequestService
	resolver worktree.WorkspaceResolver
}

func (a sessionPullRequestAdapter) PullRequests(
	ctx context.Context,
	keys []session.BranchKey,
) map[session.BranchKey]session.PullRequestView {
	resolved := make(map[session.BranchKey]worktree.PullRequestKey, len(keys))
	repos := make(map[string]string)
	out := make(map[session.BranchKey]session.PullRequestView, len(keys))
	for _, key := range keys {
		repo, ok := repos[key.WorkspaceID]
		if !ok {
			workspace, err := a.resolver.ResolveWorktreeWorkspace(ctx, key.WorkspaceID, "")
			if err == nil {
				if key, keyErr := a.service.RepoKey(ctx, workspace.Root); keyErr == nil {
					repo = key
				}
			}
			repos[key.WorkspaceID] = repo
		}
		if repo == "" {
			out[key] = session.PullRequestView{
				Lookup: session.PullRequestLookup{State: session.LookupError, Cause: "unsupported_remote", Stale: true},
			}
			continue
		}
		resolved[key] = worktree.PullRequestKey{WorkspaceID: key.WorkspaceID, RepoKey: repo, Branch: key.Branch}
	}
	cacheKeys := make([]worktree.PullRequestKey, 0, len(resolved))
	for _, key := range resolved {
		cacheKeys = append(cacheKeys, key)
	}
	entries := a.service.Read(ctx, cacheKeys)
	for branch, key := range resolved {
		entry, ok := entries[key]
		if !ok {
			entry = worktree.PullRequestEntry{LookupState: "pending", Stale: true}
		}
		out[branch] = sessionPullRequestView(entry)
	}
	return out
}

func (a sessionPullRequestAdapter) RefreshBranch(
	ctx context.Context,
	key session.BranchKey,
) (session.PullRequestView, error) {
	workspace, err := a.resolver.ResolveWorktreeWorkspace(ctx, key.WorkspaceID, "")
	if err != nil {
		return session.PullRequestView{}, err
	}
	repo, err := a.service.RepoKey(ctx, workspace.Root)
	if err != nil {
		return session.PullRequestView{
			Lookup: session.PullRequestLookup{State: session.LookupError, Cause: "unsupported_remote", Stale: true},
		}, err
	}
	entry, err := a.service.Refresh(
		ctx,
		worktree.PullRequestKey{WorkspaceID: key.WorkspaceID, RepoKey: repo, Branch: key.Branch},
	)
	return sessionPullRequestView(entry), err
}

func sessionPullRequestView(entry worktree.PullRequestEntry) session.PullRequestView {
	view := session.PullRequestView{
		Lookup: session.PullRequestLookup{
			State: session.LookupState(entry.LookupState),
			Cause: entry.LookupCause,
			Stale: entry.Stale,
		},
	}
	if view.Lookup.State == "" {
		view.Lookup.State = session.LookupPending
		view.Lookup.Stale = true
	}
	facts := entry.Facts
	if facts == nil {
		return view
	}
	if facts.FetchedAt != nil {
		view.Lookup.ObservedAt = *facts.FetchedAt
	}
	if facts.PRNumber == nil || facts.PRState == nil {
		return view
	}
	checks := session.PullRequestChecks{State: session.ChecksUnknown, FailedNames: []string{}}
	if facts.Checks != nil {
		checks = session.PullRequestChecks{
			State:       session.ChecksState(facts.Checks.State),
			Passed:      facts.Checks.Passed,
			Failed:      facts.Checks.Failed,
			Pending:     facts.Checks.Pending,
			FailedNames: append([]string{}, facts.Checks.FailedNames...),
		}
	}
	review, mergeable := session.ReviewDecision(facts.ReviewDecision), session.Mergeable(facts.Mergeable)
	if review == "" {
		review = session.ReviewUnknown
	}
	if mergeable == "" {
		mergeable = session.MergeableUnknown
	}
	pr := &session.PullRequest{
		Provider:       facts.Provider,
		Number:         *facts.PRNumber,
		URL:            facts.PRURL,
		State:          session.PullRequestState(*facts.PRState),
		Draft:          facts.Draft,
		HeadSHA:        facts.HeadSHA,
		Checks:         checks,
		ReviewDecision: review,
		Mergeable:      mergeable,
	}
	pr.ReadyToMerge = pr.State == session.PullRequestOpen && pr.Draft != nil && !*pr.Draft &&
		(checks.State == session.ChecksPassing || checks.State == session.ChecksNone) &&
		(review == session.ReviewApproved || review == session.ReviewNone) &&
		mergeable == session.MergeableYes &&
		!entry.Stale &&
		entry.LookupState == "ok"
	view.PullRequest = pr
	return view
}

type sessionBranchAdapter struct{ runner worktree.GitRunner }

func (a sessionBranchAdapter) CurrentBranch(ctx context.Context, root string) (string, error) {
	head, _, err := a.runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", nil
	} // Detached HEAD and non-repositories have no branch.
	branch := strings.TrimSpace(string(head))
	origin, _, originErr := a.runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if originErr == nil {
		if branch == strings.TrimPrefix(strings.TrimSpace(string(origin)), "origin/") {
			return "", nil
		}
	} else if branch == "main" || branch == "master" {
		return "", nil
	}
	return branch, nil
}

// RepositoryKeys resolves shared workspace origins; the caller applies profile
// ownership to session rows before joining these identities to cached facts.
func (a sessionPullRequestAdapter) RepositoryKeys(
	ctx context.Context,
	scope store.ReadScope,
	workspaceIDs []string,
) map[string]string {
	result := make(map[string]string)
	for id, root := range a.workspaceRoots(ctx, scope, workspaceIDs) {
		if key, err := a.service.RepoKey(ctx, root); err == nil {
			result[id] = key
		}
	}
	return result
}

func (a sessionPullRequestAdapter) DefaultBranchNames(
	ctx context.Context,
	scope store.ReadScope,
	workspaceIDs []string,
) map[string][]string {
	result := make(map[string][]string)
	for id, root := range a.workspaceRoots(ctx, scope, workspaceIDs) {
		result[id] = a.service.DefaultBranchNames(ctx, root)
	}
	return result
}

func (a sessionPullRequestAdapter) workspaceRoots(
	ctx context.Context,
	scope store.ReadScope,
	workspaceIDs []string,
) map[string]string {
	result := make(map[string]string)
	if scope.Validate() != nil {
		return result
	}
	if workspaceIDs == nil {
		workspaces, err := a.resolver.ListWorktreeWorkspaces(ctx)
		if err != nil {
			return result
		}
		for _, workspace := range workspaces {
			if workspace.ID != "" && workspace.ID != "@user" {
				result[workspace.ID] = workspace.Root
			}
		}
		return result
	}
	for _, id := range workspaceIDs {
		if id == "" || id == "@user" {
			continue
		}
		if _, ok := result[id]; ok {
			continue
		}
		if workspace, err := a.resolver.ResolveWorktreeWorkspace(ctx, id, ""); err == nil {
			result[id] = workspace.Root
		}
	}
	return result
}

func (a sessionPullRequestAdapter) DefaultBranches(
	ctx context.Context,
	keys []session.BranchKey,
) map[session.BranchKey]bool {
	result := make(map[session.BranchKey]bool)
	roots := make(map[string]string)
	for _, key := range keys {
		root, ok := roots[key.WorkspaceID]
		if !ok {
			workspace, err := a.resolver.ResolveWorktreeWorkspace(ctx, key.WorkspaceID, "")
			if err != nil {
				continue
			}
			root = workspace.Root
			roots[key.WorkspaceID] = root
		}
		result[key] = a.service.IsDefaultBranch(ctx, root, key.Branch)
	}
	return result
}
