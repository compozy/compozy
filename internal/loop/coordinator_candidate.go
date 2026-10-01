package loop

import (
	"context"

	"github.com/compozy/compozy/internal/loop/dsl"
)

type WorktreeCandidateVerifier interface {
	VerifyReviewCandidate(context.Context, string, string, []string, string) (bool, error)
}

func WithCoordinatorWorktreeCandidateVerifier(verifier WorktreeCandidateVerifier) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) { r.candidateVerifier = verifier }
}

func (r *CoordinatorRunner) recoveredCandidateMatches(
	ctx context.Context,
	parent Run,
	inputs map[string]any,
) (bool, error) {
	candidate, err := dsl.DecodeReviewedWorktree(inputs)
	if err != nil {
		return false, nil
	}
	if candidate == nil || r.candidateVerifier == nil {
		return false, nil
	}
	return r.candidateVerifier.VerifyReviewCandidate(
		ctx,
		string(parent.WorkspaceID),
		candidate.WorktreeID,
		candidate.IncludePaths,
		candidate.Fingerprint,
	)
}
