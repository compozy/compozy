package daemon

import (
	"context"
	"errors"

	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/worktree"
)

const (
	pullRequestLookupOK         = "ok"
	pullRequestLookupPending    = "pending"
	pullRequestLookupIncomplete = "incomplete"
	pullRequestChecksUnknown    = "unknown"
)

// sessionMonitorPullRequests reads pull requests for monitors through the same service that owns the
// display cache, so a watch joins in-flight reads and runs no agent-supplied command or URL.
type sessionMonitorPullRequests struct {
	reader     sessionPullRequestAdapter
	branches   sessionBranchStore
	sessions   SessionManager
	saveBranch session.BranchSnapshotSaver
}

// lateMonitorPullRequests binds the monitor service, built before worktrees, to the source booted after them.
type lateMonitorPullRequests struct {
	source func() *sessionMonitorPullRequests
}

var errMonitorPullRequestsUnavailable = errors.New("daemon: pull request reads are unavailable")

func (l lateMonitorPullRequests) ResolvePullRequest(
	ctx context.Context,
	spec monitorpkg.Spec,
) (monitorpkg.PullRequestTarget, error) {
	source := l.source()
	if source == nil {
		return monitorpkg.PullRequestTarget{}, errMonitorPullRequestsUnavailable
	}
	return source.resolve(ctx, spec)
}

func (l lateMonitorPullRequests) ReadPullRequest(
	ctx context.Context,
	target monitorpkg.PullRequestTarget,
) (monitorpkg.PullRequestObservation, error) {
	source := l.source()
	if source == nil {
		return monitorpkg.PullRequestObservation{}, errMonitorPullRequestsUnavailable
	}
	entry, err := source.reader.service.RefreshDue(ctx, worktree.PullRequestKey{
		WorkspaceID: target.WorkspaceID, RepoKey: target.RepoKey, Branch: target.Branch,
	})
	return monitorPullRequestObservation(entry, err), nil
}

func (s *sessionMonitorPullRequests) resolve(
	ctx context.Context,
	spec monitorpkg.Spec,
) (monitorpkg.PullRequestTarget, error) {
	workspace, err := s.reader.resolver.ResolveWorktreeWorkspace(ctx, spec.WorkspaceID, "")
	if err != nil {
		return monitorpkg.PullRequestTarget{}, monitorpkg.InvalidSpec("pull_request monitors need a project workspace")
	}
	branch := spec.Branch
	if branch == "" {
		if branch, err = s.sessionBranch(ctx, spec.SessionID); err != nil {
			return monitorpkg.PullRequestTarget{}, err
		}
	}
	if branch == "" {
		return monitorpkg.PullRequestTarget{}, monitorpkg.InvalidSpec(
			"this session has no pull request branch: check out a feature branch or pass branch")
	}
	if s.reader.service.IsDefaultBranch(ctx, workspace.Root, branch) {
		return monitorpkg.PullRequestTarget{}, monitorpkg.InvalidSpec(
			"branch is a default branch, which has no pull request to watch")
	}
	repo, err := s.reader.service.RepoKey(ctx, workspace.Root)
	if err != nil {
		return monitorpkg.PullRequestTarget{}, monitorpkg.InvalidSpec(
			"pull_request monitors need an origin remote the forge can read")
	}
	return monitorpkg.PullRequestTarget{WorkspaceID: spec.WorkspaceID, RepoKey: repo, Branch: branch}, nil
}

// sessionBranch refreshes the session's branch first, because the branch that needs watching is often the
// one this turn just created.
func (s *sessionMonitorPullRequests) sessionBranch(ctx context.Context, sessionID string) (string, error) {
	info, err := s.sessions.Status(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if err := s.saveBranch(ctx, info); err != nil {
		return "", err
	}
	snapshot, err := s.branches.SessionBranches(ctx, []string{sessionID})
	if err != nil {
		return "", err
	}
	return snapshot[sessionID], nil
}

// monitorPullRequestObservation trusts Facts only for a current successful lookup: an error entry keeps
// its last facts, which must never be read as check results.
func monitorPullRequestObservation(
	entry worktree.PullRequestEntry,
	err error,
) monitorpkg.PullRequestObservation {
	switch {
	case err != nil:
		return failedMonitorObservation(entry, err)
	case entry.LookupState == pullRequestLookupPending, entry.LookupState == pullRequestLookupIncomplete:
		return monitorpkg.PullRequestObservation{Lookup: monitorpkg.PullRequestAbsent}
	case entry.LookupState != pullRequestLookupOK:
		return failedMonitorObservation(entry, nil)
	}
	facts := entry.Facts
	if facts == nil || facts.PRNumber == nil || facts.PRState == nil {
		return monitorpkg.PullRequestObservation{Lookup: monitorpkg.PullRequestAbsent}
	}
	observation := monitorpkg.PullRequestObservation{
		Lookup: monitorpkg.PullRequestFound, Number: *facts.PRNumber, URL: facts.PRURL,
		State: *facts.PRState, Head: facts.HeadSHA,
	}
	if checks := facts.Checks; checks != nil && checks.State != pullRequestChecksUnknown {
		observation.Checks = &monitorpkg.PullRequestChecks{
			Passed: checks.Passed, Failed: checks.Failed, Pending: checks.Pending, FailedNames: checks.FailedNames,
		}
	}
	if reviews := facts.Reviews; reviews != nil {
		items := make([]monitorpkg.PullRequestReview, 0, len(reviews.Items))
		for i := range reviews.Items {
			item := &reviews.Items[i]
			items = append(items, monitorpkg.PullRequestReview{
				ID: item.ID, Author: item.Author, IsBot: item.IsBot, State: item.State, Commit: item.Commit,
			})
		}
		observation.Reviews = &monitorpkg.PullRequestReviews{
			Items: items, UnresolvedThreads: reviews.UnresolvedThreads, Truncated: reviews.Truncated,
		}
	}
	return observation
}

func failedMonitorObservation(entry worktree.PullRequestEntry, err error) monitorpkg.PullRequestObservation {
	cause := entry.LookupCause
	if cause == "" {
		cause = worktree.ForgeFailureCause(err)
	}
	if cause == "" {
		cause = "provider_failed"
	}
	return monitorpkg.PullRequestObservation{Lookup: monitorpkg.PullRequestFailed, Cause: cause}
}
