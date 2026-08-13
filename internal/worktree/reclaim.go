package worktree

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/diagnostics"
)

type ReclaimOutcome struct {
	Eligible bool
	Deleted  bool
	Reason   string
}

func (s *Service) reclaimBranch(
	ctx context.Context,
	root string,
	item Worktree,
	status *Status,
) (ReclaimOutcome, error) {
	if !ReclamationEligible(item.Branch, item.RunNamespace, item.CreatedBranch) || item.CreatedHead == "" {
		return ReclaimOutcome{Reason: "branch is not runtime-owned"}, nil
	}
	if status == nil || status.Detached == nil || *status.Detached || status.Branch == nil ||
		*status.Branch != item.Branch {
		return ReclaimOutcome{Eligible: true, Reason: "checkout no longer owns the recorded branch"}, nil
	}
	ref := "refs/heads/" + item.Branch
	_, stderr, err := s.runner.Run(ctx, root, "update-ref", "-d", ref, item.CreatedHead)
	if err != nil {
		if len(stderr) == 0 {
			return ReclaimOutcome{Eligible: true, Reason: "branch pointer changed"}, nil
		}
		return ReclaimOutcome{Eligible: true}, fmt.Errorf(
			"worktree: reclaim branch: %s: %w",
			diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	if len(stderr) > 0 {
		return ReclaimOutcome{}, fmt.Errorf(
			"worktree: reclaim branch: %s",
			diagnostics.RedactAndBound(string(stderr), 2048),
		)
	}
	s.emit(ctx, EventBranchReclaimed, item)
	return ReclaimOutcome{Eligible: true, Deleted: true}, nil
}
