package mcp

import (
	"context"
	"encoding/json"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const hostAPIWorkspaceLiteral = "workspace"

func bindHostAPIParams(
	ctx context.Context,
	raw json.RawMessage,
	binding workspaceBinding,
	workspaceID string,
	workspaceRoot string,
	workspaces workspacepkg.RuntimeResolver,
) (json.RawMessage, error) {
	return extensionpkg.BindHostAPIWorkspaceParams(
		ctx,
		raw,
		binding,
		workspaceID,
		workspaceRoot,
		workspaces,
	)
}
