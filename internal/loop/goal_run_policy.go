package loop

import (
	"context"
	"fmt"
	"math"
)

var _ GoalRunPolicyResolver = GoalRunPolicyResolverFunc(nil)

// GoalRunPolicy is the workspace-resolved policy pinned into a new Run.
type GoalRunPolicy struct {
	ContextNudgeRatio float64
}

// Validate rejects values that cannot be persisted or used as a context threshold.
func (p GoalRunPolicy) Validate() error {
	ratio := p.ContextNudgeRatio
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 1 {
		return fmt.Errorf(
			"%w: Goal context nudge ratio must be between 0 and 1: %v",
			ErrValidation,
			ratio,
		)
	}
	return nil
}

// GoalRunPolicyResolver resolves workspace policy before any Run start write.
type GoalRunPolicyResolver interface {
	ResolveGoalRunPolicy(context.Context, WorkspaceID) (*GoalRunPolicy, error)
}

// GoalRunPolicyResolverFunc adapts a function to GoalRunPolicyResolver.
type GoalRunPolicyResolverFunc func(context.Context, WorkspaceID) (*GoalRunPolicy, error)

// ResolveGoalRunPolicy implements GoalRunPolicyResolver.
func (f GoalRunPolicyResolverFunc) ResolveGoalRunPolicy(
	ctx context.Context,
	workspaceID WorkspaceID,
) (*GoalRunPolicy, error) {
	return f(ctx, workspaceID)
}
