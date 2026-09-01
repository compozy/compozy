package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const toolWorkspaceRootResolverDependency = "tool_workspace_root_resolver"

// ActionToolWorkspaceRootRequest identifies the effective workspace root for one tool action.
type ActionToolWorkspaceRootRequest struct {
	WorkspaceID WorkspaceID
	Environment dsl.EnvironmentSpec
}

// ActionToolWorkspaceRootResolver resolves trusted roots outside the Loop package.
type ActionToolWorkspaceRootResolver interface {
	AcquireActionToolWorkspaceRoot(context.Context, ActionToolWorkspaceRootRequest) (string, func(), error)
}

func (e *ToolCallActionExecutor) acquireTrustedWorkspaceRoot(
	ctx context.Context,
	in ActionExecutionInput,
) (string, func(), error) {
	environment := in.EnvironmentValue()
	if environment.Mode != dsl.EnvironmentWorktree {
		return "", nil, nil
	}
	if e.workspaceRootResolver == nil {
		return "", nil, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: toolWorkspaceRootResolverDependency},
		)
	}
	root, release, err := e.workspaceRootResolver.AcquireActionToolWorkspaceRoot(ctx, ActionToolWorkspaceRootRequest{
		WorkspaceID: in.WorkspaceID,
		Environment: environment,
	})
	if err != nil {
		return "", nil, fmt.Errorf("acquire action tool workspace root: %w", err)
	}
	if release == nil {
		return "", nil, reasonError(
			ReasonCodeActionDependencyMissing,
			fmt.Errorf("%w: tool workspace root resolver returned no release function", ErrActionDependencyMissing),
			map[string]string{actionDependencyMetaKey: toolWorkspaceRootResolverDependency},
		)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		release()
		return "", nil, reasonError(
			ReasonCodeActionDependencyMissing,
			fmt.Errorf("%w: tool workspace root resolver returned an empty root", ErrActionDependencyMissing),
			map[string]string{actionDependencyMetaKey: toolWorkspaceRootResolverDependency},
		)
	}
	return root, release, nil
}
