package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// ActionToolWorkspaceRootRequest identifies the effective workspace root for one tool action.
type ActionToolWorkspaceRootRequest struct {
	WorkspaceID WorkspaceID
	Environment dsl.EnvironmentSpec
}

// ActionToolWorkspaceRootResolver resolves trusted roots outside the Loop package.
type ActionToolWorkspaceRootResolver interface {
	ResolveActionToolWorkspaceRoot(context.Context, ActionToolWorkspaceRootRequest) (string, error)
}

func (e *ToolCallActionExecutor) trustedWorkspaceRoot(
	ctx context.Context,
	in ActionExecutionInput,
) (string, error) {
	environment := in.EnvironmentValue()
	if environment.Mode != dsl.EnvironmentWorktree {
		return "", nil
	}
	if e.workspaceRootResolver == nil {
		return "", reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "tool_workspace_root_resolver"},
		)
	}
	root, err := e.workspaceRootResolver.ResolveActionToolWorkspaceRoot(ctx, ActionToolWorkspaceRootRequest{
		WorkspaceID: in.WorkspaceID,
		Environment: environment,
	})
	if err != nil {
		return "", fmt.Errorf("resolve action tool workspace root: %w", err)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return "", reasonError(
			ReasonCodeActionDependencyMissing,
			fmt.Errorf("%w: tool workspace root resolver returned an empty root", ErrActionDependencyMissing),
			map[string]string{actionDependencyMetaKey: "tool_workspace_root_resolver"},
		)
	}
	return root, nil
}
