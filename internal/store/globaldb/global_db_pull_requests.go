package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/worktree"
)

func (g *WorktreeRepo) ReadPullRequests(ctx context.Context, keys []worktree.PullRequestKey) (map[worktree.PullRequestKey]worktree.PullRequestEntry, error) {
	if err := g.checkReady(ctx, "read pull requests"); err != nil {
		return nil, err
	}
	input := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		input = append(input, map[string]string{"workspace_id": key.WorkspaceID, "repo_key": key.RepoKey, "branch": key.Branch})
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	rows, err := g.queries.ReadForgePullRequests(ctx, string(data))
	if err != nil {
		return nil, fmt.Errorf("store: read pull requests: %w", err)
	}
	entries := make(map[worktree.PullRequestKey]worktree.PullRequestEntry, len(rows))
	for _, row := range rows {
		entry, err := pullRequestEntry(row)
		if err != nil {
			return nil, err
		}
		entries[entry.Key] = entry
	}
	return entries, nil
}

func pullRequestEntry(row sqlcgen.ForgePullRequest) (worktree.PullRequestEntry, error) {
	entry := worktree.PullRequestEntry{Key: worktree.PullRequestKey{WorkspaceID: row.WorkspaceID, RepoKey: row.RepoKey, Branch: row.Branch}, LookupState: row.LookupState, LookupCause: row.LookupCause, FailureCount: int(row.FailureCount)}
	var err error
	if entry.AttemptAt, err = pointerTime(row.AttemptAt); err != nil {
		return entry, err
	}
	if entry.RetryAfter, err = pointerTime(row.RetryAfter); err != nil {
		return entry, err
	}
	if row.ObservedAt.Valid {
		facts := &worktree.ForgeStatus{Provider: row.Provider, PRNumber: pointerInt(row.PrNumber), PRState: pointerString(row.PrState), PRURL: row.PrUrl, Merged: pointerBool(row.Merged), Draft: pointerBool(row.Draft), HeadSHA: row.HeadSha, Base: row.BaseRef, ReviewDecision: row.ReviewDecision, Mergeable: row.Mergeable, Checks: &worktree.ForgeChecks{State: row.ChecksState, Passed: int(row.ChecksPassed), Failed: int(row.ChecksFailed), Pending: int(row.ChecksPending)}}
		if facts.FetchedAt, err = pointerTime(row.ObservedAt); err != nil {
			return entry, err
		}
		if facts.UpdatedAt, err = pointerTime(row.PrUpdatedAt); err != nil {
			return entry, err
		}
		if err = json.Unmarshal([]byte(row.ChecksFailedNames), &facts.Checks.FailedNames); err != nil {
			return entry, fmt.Errorf("store: decode failed check names: %w", err)
		}
		entry.Facts = facts
	}
	return entry, nil
}

func (g *WorktreeRepo) SavePullRequest(ctx context.Context, entry worktree.PullRequestEntry) error {
	if err := g.checkReady(ctx, "save pull request"); err != nil {
		return err
	}
	facts := entry.Facts
	if facts == nil {
		facts = &worktree.ForgeStatus{}
	}
	checks := facts.Checks
	if checks == nil {
		checks = &worktree.ForgeChecks{State: "unknown", FailedNames: []string{}}
	}
	names := checks.FailedNames
	if names == nil {
		names = []string{}
	}
	data, err := json.Marshal(names[:min(len(names), 5)])
	if err != nil {
		return err
	}
	review, mergeable := facts.ReviewDecision, facts.Mergeable
	if review == "" {
		review = "unknown"
	}
	if mergeable == "" {
		mergeable = "unknown"
	}
	err = g.queries.UpsertForgePullRequest(ctx, sqlcgen.UpsertForgePullRequestParams{
		WorkspaceID: entry.Key.WorkspaceID, RepoKey: entry.Key.RepoKey, Branch: entry.Key.Branch,
		Provider: facts.Provider, PrNumber: nullableInt(facts.PRNumber), PrUrl: facts.PRURL, PrState: nullableString(facts.PRState), Merged: nullableBool(facts.Merged), Draft: nullableBool(facts.Draft), HeadSha: facts.HeadSHA, BaseRef: facts.Base,
		ChecksState: checks.State, ChecksPassed: int64(checks.Passed), ChecksFailed: int64(checks.Failed), ChecksPending: int64(checks.Pending), ChecksFailedNames: string(data), ReviewDecision: review, Mergeable: mergeable, PrUpdatedAt: nullableTime(facts.UpdatedAt), ObservedAt: nullableTime(facts.FetchedAt), LookupState: entry.LookupState, LookupCause: entry.LookupCause, AttemptAt: nullableTime(entry.AttemptAt), FailureCount: int64(entry.FailureCount), RetryAfter: nullableTime(entry.RetryAfter),
	})
	if err != nil {
		return fmt.Errorf("store: save pull request: %w", err)
	}
	return nil
}
