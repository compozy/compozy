package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/runtimemcp"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) mcpImport(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct {
		Runtime    string `json:"runtime"`
		ServerName string `json:"server_name"`
		Scope      string `json:"scope"`
		Workspace  string `json:"workspace"`
		Name       string `json:"name"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	service, ok := n.settingsService().(core.RuntimeMCPImporter)
	if !ok {
		return toolspkg.ToolResult{}, fmt.Errorf("daemon: MCP copy unavailable")
	}
	body := contract.RuntimeMCPImportRequest{Scope: contract.SettingsScopeKind(input.Scope), Name: input.Name}
	if input.Workspace != "" || scope.WorkspaceID != "" || input.Scope == "workspace" {
		resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if scope.WorkspaceID != "" && resolved.ID != scope.WorkspaceID {
			return toolspkg.ToolResult{}, errors.New("runtime MCP copy is limited to the caller workspace")
		}
		body.WorkspaceID = resolved.ID
	}
	if input.Scope == "profile" && scope.ProfileID != "" && scope.ProfileID != store.DefaultProfileID {
		if n.deps.Profiles == nil {
			return toolspkg.ToolResult{}, fmt.Errorf("daemon: profile resolver unavailable")
		}
		profile, err := n.deps.Profiles.ProfileName(ctx, scope.ProfileID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		body.Profile = profile
	}
	result, err := core.ImportRuntimeMCP(ctx, service, input.Runtime, input.ServerName, body)
	if err != nil {
		if failure, ok := errors.AsType[*runtimemcp.ImportError](err); ok {
			return toolspkg.ToolResult{}, toolspkg.NewToolError(
				toolspkg.ErrorCode(failure.Code),
				req.ToolID,
				failure.Message,
				err,
			)
		}
		return toolspkg.ToolResult{}, nativeHTTPStatusToolError(req.ToolID, err, core.StatusForSettingsError(err))
	}
	return structuredResult(result, "Copied "+result.MCPServer.Name+" to CompozyOS")
}
