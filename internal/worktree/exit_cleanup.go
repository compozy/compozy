package worktree

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	forgePRMergedState = "merged"
	cleanupLocalSource = "local"
)

const exitForgeFreshness = 30 * time.Second

func (s *Service) exitCleanupEvidence(ctx context.Context, item Worktree, forge *ForgeStatus) ExitCleanupEvidence {
	evidence := ExitCleanupEvidence{}
	if forge != nil && forge.PRState != nil {
		evidence.ForgeState = *forge.PRState
		evidence.Stale = forge.IsStale(s.now().UTC(), exitForgeFreshness)
	}
	if !evidence.Stale && forgeMergeIsCurrent(ctx, s.runner, item.Path, forge) {
		evidence.Safe, evidence.Source = true, "forge"
		evidence.Summary = "Merged on " + forge.Provider
		return evidence
	}
	if forge != nil && forge.PRState != nil && *forge.PRState == forgePRMergedState {
		// A merged verdict that cannot be proven against the current HEAD is not
		// current cleanup authority, even when its wall-clock TTL has not elapsed.
		evidence.Stale = true
	}
	stdout, _, err := s.runner.Run(
		ctx, item.Path, "rev-list", "--count", "HEAD", "--not", "--remotes",
		"--exclude=refs/heads/"+item.Branch, "--branches",
	)
	if err != nil {
		return evidence
	}
	unique, parseErr := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if parseErr != nil {
		return evidence
	}
	if unique == 0 {
		evidence.Safe, evidence.Source = true, cleanupLocalSource
		evidence.Summary = "No commits unique to this branch."
		return evidence
	}
	remote, _, remoteErr := s.runner.Run(ctx, item.Path, "branch", "-r", "--list", "*/"+item.Branch)
	if remoteErr == nil && strings.TrimSpace(string(remote)) != "" {
		evidence.Safe, evidence.Source, evidence.Downgraded = true, cleanupLocalSource, true
		evidence.Summary = "Unique commits are already on a remote branch."
		return evidence
	}
	evidence.Source = cleanupLocalSource
	evidence.Blocker = fmt.Sprintf("%d commits exist nowhere else.", unique)
	return evidence
}

func forgeMergeIsCurrent(
	ctx context.Context,
	runner GitRunner,
	path string,
	forge *ForgeStatus,
) bool {
	if forge == nil || forge.PRState == nil || *forge.PRState != forgePRMergedState {
		return false
	}
	if forge.FetchedAt == nil {
		return false
	}
	stdout, _, err := runner.Run(ctx, path, "log", "-1", "--format=%cI", "HEAD")
	if err != nil {
		return false
	}
	committedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(string(stdout)))
	if err != nil {
		return false
	}
	return !committedAt.After(*forge.FetchedAt)
}
