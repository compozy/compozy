package worktree

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

const removalRevListCommand = "rev-list"

func (s *Service) removalRisk(ctx context.Context, item Worktree) (RemovalRisk, error) {
	status, err := s.refreshStatus(ctx, item)
	if err != nil {
		return RemovalRisk{}, err
	}
	return s.removalRiskFromStatus(ctx, item, status)
}

func (s *Service) removalRiskFromStatus(ctx context.Context, item Worktree, status *Status) (RemovalRisk, error) {
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
	args = append([]string{removalRevListCommand, "--count"}, args...)
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
		if s.removalMergeIsCurrent(ctx, item) {
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
		// --all would include every linked worktree's HEAD, this one too, and
		// hide the commits that removal would orphan.
		return []string{removalHeadRef, "--not", "--branches", "--tags", "--remotes"}
	case status.HasUpstream != nil && *status.HasUpstream:
		if status.Ahead != nil {
			return []string{"@{upstream}..HEAD"}
		}
		return []string{removalHeadRef, "--not", "--remotes", "--exclude=" + item.Branch, "--branches"}
	default:
		return nil
	}
}

// Compare the preservation refs in one walk so shared remote history is visited once.
func (s *Service) unpreservedRemovalCommits(ctx context.Context, item Worktree, status *Status) (int, error) {
	args := append([]string{removalRevListCommand}, removalCommitComparison(item, status)...)
	stdout, stderr, err := s.runner.Run(ctx, item.Path, args...)
	if err != nil {
		return 0, fmt.Errorf("list unique commits: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	if len(stdout) >= maxGitOutputBytes {
		return 0, errors.New("worktree: candidate commit inventory exceeds the safety output limit")
	}
	remaining := make(map[string]struct{})
	for hash := range strings.FieldsSeq(string(stdout)) {
		remaining[hash] = struct{}{}
	}
	refs, err := s.removalPreservationRefs(ctx, item, status)
	if err != nil {
		return 0, err
	}
	var revisions strings.Builder
	seen := make(map[string]bool)
	for _, ref := range refs {
		if !seen[ref] {
			revisions.WriteString(ref + "...HEAD\n")
			seen[ref] = true
		}
	}
	if len(seen) == 0 {
		return len(remaining), nil
	}
	runner, ok := s.runner.(GitInputRunner)
	if !ok {
		return 0, errors.New("worktree: Git runner does not support revision input")
	}
	stdout, stderr, err = runner.RunWithInput(
		ctx,
		item.Path,
		revisions.String(),
		removalRevListCommand,
		"--cherry-pick",
		"--right-only",
		"--stdin",
	)
	if err != nil {
		return 0, fmt.Errorf("compare preserved patches: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	if len(stdout) >= maxGitOutputBytes {
		return 0, errors.New("worktree: patch comparison exceeds the safety output limit")
	}
	count := 0
	for hash := range strings.FieldsSeq(string(stdout)) {
		if _, found := remaining[hash]; found {
			count++
		}
	}
	return count, nil
}

func (s *Service) removalPreservationRefs(ctx context.Context, item Worktree, status *Status) ([]string, error) {
	stdout, stderr, err := s.runner.Run(
		ctx, item.Path, "for-each-ref", "--format=%(refname) %(objectname) %(symref)", "refs/remotes/", "refs/heads/",
	)
	if err != nil {
		return nil, fmt.Errorf("list preservation refs: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	if len(stdout) >= maxGitOutputBytes {
		return nil, errors.New("worktree: preservation refs exceed the safety output limit")
	}
	refs := []string{}
	if item.BaseRef != "" {
		refs = append(refs, item.BaseRef)
	}
	if status.HasUpstream != nil && *status.HasUpstream && status.Ahead != nil {
		refs = append(refs, "@{upstream}")
	}
	localRefs := make(map[string]string)
	defaultRef := "refs/heads/" + defaultBaseBranch
	for line := range strings.SplitSeq(string(stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.HasPrefix(fields[0], "refs/heads/") {
			localRefs[fields[0]] = fields[1]
		} else {
			refs = append(refs, fields[1])
		}
		if fields[0] == "refs/remotes/origin/HEAD" && len(fields) == 3 {
			defaultRef = "refs/heads/" + strings.TrimPrefix(fields[2], "refs/remotes/origin/")
		}
	}
	if tip := localRefs[defaultRef]; tip != "" {
		refs = append(refs, tip)
	}
	return refs, nil
}
