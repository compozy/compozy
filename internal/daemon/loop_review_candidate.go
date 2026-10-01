package daemon

import (
	"context"

	looppkg "github.com/compozy/compozy/internal/loop"
)

type daemonReviewCandidateVerifier struct{ state *bootState }

var _ looppkg.WorktreeCandidateVerifier = daemonReviewCandidateVerifier{}

func (v daemonReviewCandidateVerifier) VerifyReviewCandidate(
	ctx context.Context,
	workspaceID, worktreeID string,
	paths []string,
	fingerprint string,
) (bool, error) {
	if v.state == nil || v.state.worktrees == nil {
		return false, nil
	}
	return v.state.worktrees.VerifyReviewCandidate(ctx, workspaceID, worktreeID, paths, fingerprint)
}
