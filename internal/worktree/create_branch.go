package worktree

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

func (s *Service) resolvePendingBranch(
	ctx context.Context,
	workspaceRoot string,
	item *Worktree,
	options CreateOptions,
) error {
	item.CreatedBranch = options.ExistingBranch == ""
	if item.CreatedBranch {
		baseRef, baseHead, err := s.resolveBaseRef(ctx, workspaceRoot, item.BaseRef)
		if err != nil {
			return err
		}
		item.BaseRef, item.CreatedHead = baseRef, baseHead
		return nil
	}
	head, stderr, err := s.runner.Run(ctx, workspaceRoot, "rev-parse", "--verify", item.Branch+"^{commit}")
	if err != nil {
		return refusal(ErrBaseRefNotFound, diagnostics.RedactAndBound(string(stderr), 2048))
	}
	item.CreatedHead = strings.TrimSpace(string(head))
	return nil
}

func (s *Service) recordPendingBranchBase(
	ctx context.Context,
	workspace Workspace,
	item *Worktree,
	placementRoot string,
	explicitPath bool,
) error {
	if !item.CreatedBranch {
		return nil
	}
	if err := s.revalidateMutationPath(ctx, workspace, *item, placementRoot, explicitPath); err != nil {
		return err
	}
	key := "branch." + item.Branch + ".gh-merge-base"
	if _, stderr, err := s.runner.Run(ctx, workspace.Root, "config", key, item.BaseRef); err != nil {
		return fmt.Errorf(
			"worktree: record branch base: %s: %w",
			diagnostics.RedactAndBound(string(stderr), 2048),
			err,
		)
	}
	return nil
}
