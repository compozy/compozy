package daemon

import (
	"context"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func newGoalRunPolicyResolver(
	homePaths compozyconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
) looppkg.GoalRunPolicyResolver {
	return looppkg.GoalRunPolicyResolverFunc(func(
		ctx context.Context,
		workspaceID looppkg.WorkspaceID,
	) (*looppkg.GoalRunPolicy, error) {
		cfg, err := resolveLoopServiceConfig(ctx, homePaths, workspaceResolver, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("daemon: resolve Goal policy for workspace %q: %w", workspaceID, err)
		}
		return &looppkg.GoalRunPolicy{
			ContextNudgeRatio: cfg.Goals.ContextNudgeRatio,
		}, nil
	})
}
