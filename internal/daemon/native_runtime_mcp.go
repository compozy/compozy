package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) mcpList(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
	var input struct {
		Runtime         string `json:"runtime"`
		IncludeRuntimes *bool  `json:"include_runtimes"`
		Workspace       string `json:"workspace"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if n.deps.Settings == nil || n.deps.Settings() == nil {
		return toolspkg.ToolResult{}, errors.New("MCP settings service unavailable")
	}
	workspaceID := strings.TrimSpace(firstNonEmpty(scope.WorkspaceID, req.WorkspaceID))
	if input.Workspace != "" {
		if n.deps.WorkspaceResolver == nil {
			return toolspkg.ToolResult{}, errors.New("workspace resolver unavailable")
		}
		resolved, err := n.deps.WorkspaceResolver.Resolve(ctx, input.Workspace)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if workspaceID != "" && workspaceID != resolved.ID {
			return toolspkg.ToolResult{}, errors.New("runtime MCP inventory is limited to the caller workspace")
		}
		workspaceID = resolved.ID
	}
	readReq := settingspkg.CollectionRequest{Collection: settingspkg.CollectionMCPServers, Scope: settingspkg.ScopeUser, WorkspaceID: workspaceID}
	if workspaceID != "" {
		readReq.Scope = settingspkg.ScopeWorkspace
	}
	if scope.ProfileID != "" && n.deps.Profiles != nil {
		name, err := n.deps.Profiles.ProfileName(ctx, scope.ProfileID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if name != "" && name != "default" {
			readReq.ProfileName = name
			readReq.Scope = settingspkg.ScopeProfile
		}
	}
	include := input.IncludeRuntimes == nil || *input.IncludeRuntimes
	result, err := core.MCPList(ctx, n.deps.Settings(), readReq, strings.TrimSpace(input.Runtime), include)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(result, "Configured MCP servers")
}
