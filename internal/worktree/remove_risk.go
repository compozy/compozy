package worktree

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

func (s *Service) removalRisk(ctx context.Context, item Worktree) (RemovalRisk, error) {
	status, err := s.refreshStatus(ctx, item)
	if err != nil {
		return RemovalRisk{}, err
	}
	if status.ReadError != "" {
		return RemovalRisk{}, refusal(ErrStatusUnreadable, status.ReadError)
	}
	risk := RemovalRisk{
		ChangedFiles: valueOrZero(status.DirtyFiles), Insertions: valueOrZero(status.Insertions),
		Deletions: valueOrZero(status.Deletions),
	}
	args := removalCommitComparison(item, status)
	if len(args) == 0 {
		return risk, nil
	}
	args = append([]string{"rev-list", "--count"}, args...)
	stdout, stderr, err := s.runner.Run(ctx, item.Path, args...)
	if err != nil {
		return RemovalRisk{}, fmt.Errorf(
			"count unique commits: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	risk.UnpushedCommits, err = strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return RemovalRisk{}, fmt.Errorf("parse unique commit count: %w", err)
	}
	if risk.UnpushedCommits == 0 || (status.Detached != nil && *status.Detached) {
		return risk, nil
	}
	stdout, stderr, err = s.runner.Run(ctx, item.Path, "branch", "-r", "--contains", "HEAD")
	if err != nil {
		return RemovalRisk{}, fmt.Errorf(
			"inspect remote containment: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	risk.ExistsOnRemote = strings.TrimSpace(string(stdout)) != ""
	if !risk.ExistsOnRemote {
		risk.UnpushedCommits, err = s.unpreservedRemovalCommits(ctx, item, status)
		if err != nil {
			return RemovalRisk{}, err
		}
		if risk.UnpushedCommits == 0 {
			return risk, nil
		}
		merged, err := s.removalMergeIsCurrent(ctx, item)
		if err != nil {
			return RemovalRisk{}, err
		}
		if merged {
			risk.UnpushedCommits = 0
		}
	}
	return risk, nil
}

func removalCommitComparison(item Worktree, status *Status) []string {
	switch {
	case item.BaseRef != "":
		return []string{item.BaseRef + "..HEAD"}
	case status.Detached != nil && *status.Detached:
		return []string{removalHeadRef, "--not", "--all"}
	case status.HasUpstream != nil && *status.HasUpstream:
		if status.Ahead != nil {
			return []string{"@{upstream}..HEAD"}
		}
		return []string{removalHeadRef, "--not", "--remotes", "--exclude=" + item.Branch, "--branches"}
	default:
		return nil
	}
}

// Intersect unmatched commits so patches preserved across different refs count only once.
func (s *Service) unpreservedRemovalCommits(ctx context.Context, item Worktree, status *Status) (int, error) {
	args := append([]string{"rev-list"}, removalCommitComparison(item, status)...)
	stdout, stderr, err := s.runner.Run(ctx, item.Path, args...)
	if err != nil {
		return 0, fmt.Errorf("list unique commits: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	remaining := make(map[string]struct{})
	for hash := range strings.FieldsSeq(string(stdout)) {
		remaining[hash] = struct{}{}
	}
	stdout, stderr, err = s.runner.Run(
		ctx,
		item.Path,
		"for-each-ref",
		"--format=%(refname)",
		"refs/remotes/",
		"refs/heads/"+defaultBaseBranch,
	)
	if err != nil {
		return 0, fmt.Errorf("list preservation refs: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	refs := strings.Fields(string(stdout))
	if item.BaseRef != "" {
		refs = append(refs, item.BaseRef)
	}
	if status.HasUpstream != nil && *status.HasUpstream && status.Ahead != nil {
		refs = append(refs, "@{upstream}")
	}
	for _, ref := range refs {
		if len(remaining) == 0 {
			break
		}
		stdout, stderr, err = s.runner.Run(
			ctx,
			item.Path,
			"rev-list",
			"--cherry-pick",
			"--right-only",
			ref+"...HEAD",
			"--",
		)
		if err != nil {
			return 0, fmt.Errorf(
				"compare preserved patches: %s: %w",
				diagnostics.RedactAndBound(string(stderr), 2048),
				err,
			)
		}
		unmatched := make(map[string]struct{})
		for hash := range strings.FieldsSeq(string(stdout)) {
			unmatched[hash] = struct{}{}
		}
		for hash := range remaining {
			if _, found := unmatched[hash]; !found {
				delete(remaining, hash)
			}
		}
	}
	return len(remaining), nil
}
