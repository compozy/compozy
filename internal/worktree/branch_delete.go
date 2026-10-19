package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/fileutil"
)

// Branch outcome reasons explain why an opted-in local branch deletion kept the branch.
const (
	BranchKeptNoBranch            = "no_branch"
	BranchKeptAbsent              = "branch_absent"
	BranchKeptCheckoutPresent     = "checkout_present"
	BranchKeptRootCheckout        = "root_checkout_branch"
	BranchKeptDefaultBranch       = "default_branch"
	BranchKeptRegistrationPresent = "registration_present"
	BranchKeptCheckedOutElsewhere = "checked_out_elsewhere"
	BranchKeptNotFullyMerged      = "not_fully_merged"
	BranchKeptDeleteFailed        = "delete_failed"
)

// BranchOutcome reports an opt-in local branch deletion that followed a cleanup.
// Kept branches carry a stable Reason; Detail holds bounded Git diagnostics.
type BranchOutcome struct {
	Name    string
	Deleted bool
	Reason  string
	Detail  string
}

// DeleteLocalBranch deletes the recorded local branch of a removed or dismissed
// worktree. It refuses the root checkout's HEAD and default branches, never
// touches a path that reappeared, and uses Git's merged-only delete unless force
// is set. Refusals are outcomes; errors are reserved for unreadable state, and
// errors after the record is read still return the outcome with its branch name.
func (s *Service) DeleteLocalBranch(
	ctx context.Context,
	workspaceID string,
	id string,
	force bool,
) (BranchOutcome, error) {
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || (err == nil && item == nil) {
		return BranchOutcome{}, ErrNotFound
	}
	if err != nil {
		return BranchOutcome{}, fmt.Errorf("worktree: read branch deletion target: %w", err)
	}
	if item.State != StateRemoved && item.State != StateDismissed {
		return BranchOutcome{}, ErrNotReady
	}
	// Once the record is read, failures still name the branch they concern.
	branch := strings.TrimSpace(item.Branch)
	outcome := BranchOutcome{Name: branch}
	release, acquired := s.usage.tryAcquireExclusive(worktreeUsageKey(item.WorkspaceID, item.ID))
	if !acquired {
		return outcome, ErrOperationInProgress
	}
	defer release()

	if branch == "" {
		outcome.Reason = BranchKeptNoBranch
		return outcome, nil
	}
	workspace, err := s.resolveWorkspace(ctx, item.WorkspaceID)
	if err != nil {
		return outcome, err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return outcome, err
	}
	unlock, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return outcome, err
	}
	defer unlock()

	outcome.Reason, outcome.Detail, err = s.branchDeletionRefusal(ctx, workspace.Root, commonDir, *item, force)
	if err != nil || outcome.Reason != "" {
		return outcome, err
	}
	flag := "-d"
	if force {
		flag = "-D"
	}
	if _, stderr, err := s.runner.Run(ctx, workspace.Root, "branch", flag, branch); err != nil {
		outcome.Reason = BranchKeptDeleteFailed
		outcome.Detail = diagnostics.RedactAndBound(strings.TrimSpace(string(stderr)), 2048)
		return outcome, nil
	}
	outcome.Deleted = true
	s.logger.InfoContext(ctx, "worktree local branch deleted", "worktree_id", item.ID, "force", force)
	return outcome, nil
}

// branchDeletionRefusal runs every keep-the-branch check under the repository
// lock, releasing the record's stale registration on the way. An empty reason
// means Git may now delete the branch.
func (s *Service) branchDeletionRefusal(
	ctx context.Context,
	root, commonDir string,
	item Worktree,
	force bool,
) (string, string, error) {
	if _, err := os.Lstat(item.Path); err == nil {
		return BranchKeptCheckoutPresent, "", nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("worktree: inspect branch deletion path: %w", err)
	}
	branch := strings.TrimSpace(item.Branch)
	if reason := s.protectedBranchReason(ctx, root, branch); reason != "" {
		return reason, "", nil
	}
	ref := "refs/heads/" + branch
	if _, _, err := s.runner.Run(ctx, root, "show-ref", "--verify", "--quiet", ref); err != nil {
		return BranchKeptAbsent, "", nil
	}
	reason, detail, err := s.releaseStaleRegistration(ctx, root, commonDir, item.Path, ref)
	if err != nil || reason != "" {
		return reason, detail, err
	}
	if !force && !s.branchMerged(ctx, root, branch) {
		return BranchKeptNotFullyMerged, "", nil
	}
	return "", "", nil
}

// protectedBranchReason keeps branches whose deletion would strand the repository:
// the root checkout's HEAD and the remote default (or the conventional default).
func (s *Service) protectedBranchReason(ctx context.Context, root, branch string) string {
	stdout, _, err := s.runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil && strings.TrimSpace(string(stdout)) == branch {
		return BranchKeptRootCheckout
	}
	defaultBranch := defaultBaseBranch
	stdout, _, err = s.runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if value := strings.TrimSpace(string(stdout)); err == nil && value != "" {
		defaultBranch = value
		if _, name, found := strings.Cut(value, "/"); found {
			defaultBranch = name
		}
	}
	if branch == defaultBranch {
		return BranchKeptDefaultBranch
	}
	return ""
}

// releaseStaleRegistration clears Git's administrative entry for this record's
// absent checkout so the branch is no longer reported as in use. Only the entry
// at the recorded path is touched; other prunable worktrees stay registered. A
// non-empty reason keeps the branch: its entry is locked or could not be
// released, or another checkout still has the branch.
func (s *Service) releaseStaleRegistration(
	ctx context.Context,
	root, commonDir, path, ref string,
) (string, string, error) {
	entries, err := s.readWorktreeList(ctx, root, commonDir)
	if err != nil {
		return "", "", err
	}
	target, err := fileutil.CanonicalPathWithExistingPrefix(path)
	if err != nil {
		return "", "", fmt.Errorf("worktree: canonicalize branch deletion path: %w", err)
	}
	var stale *GitWorktree
	for index := range entries {
		entry := &entries[index]
		candidate, err := fileutil.CanonicalPathWithExistingPrefix(entry.Path)
		if err == nil && candidate == target {
			stale = entry
			continue
		}
		if entry.Branch == ref {
			return BranchKeptCheckedOutElsewhere, "", nil
		}
	}
	if stale == nil {
		return "", "", nil
	}
	if stale.Locked {
		return BranchKeptRegistrationPresent, "the worktree registration is locked", nil
	}
	_, stderr, err := s.runner.Run(
		ctx, root, "--git-dir", commonDir, worktreeDescriptorName, exitRemoveAction, stale.Path,
	)
	if err != nil {
		return BranchKeptRegistrationPresent, diagnostics.RedactAndBound(strings.TrimSpace(string(stderr)), 2048), nil
	}
	return "", "", nil
}

// branchMerged mirrors Git's merged-only rule: the branch must be contained in
// its upstream when one is set, otherwise in the root checkout's HEAD.
func (s *Service) branchMerged(ctx context.Context, root, branch string) bool {
	target := "HEAD"
	stdout, _, err := s.runner.Run(
		ctx, root, "rev-parse", "--verify", "--quiet", "--symbolic-full-name", branch+"@{upstream}",
	)
	if upstream := strings.TrimSpace(string(stdout)); err == nil && upstream != "" {
		target = upstream
	}
	_, _, err = s.runner.Run(ctx, root, "merge-base", "--is-ancestor", "refs/heads/"+branch, target)
	return err == nil
}
