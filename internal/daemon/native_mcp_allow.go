package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/core"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) mcpAllow(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct {
		ServerName       string `json:"server_name"`
		Workspace        string `json:"workspace"`
		Decision         string `json:"decision"`
		DefinitionDigest string `json:"definition_digest"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.Decision != "allow" && input.Decision != "revoke" {
		return toolspkg.ToolResult{}, toolspkg.NewValidationError(
			"decision",
			toolspkg.ReasonSchemaInvalid,
			"decision must be allow or revoke",
		)
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	service, ok := n.settingsService().(core.MCPSettingsApproval)
	if !ok {
		return toolspkg.ToolResult{}, fmt.Errorf("daemon: MCP approvals are unavailable")
	}
	profile := "default"
	target := settingspkg.MCPAuthTargetRequest{
		Scope:       settingspkg.ScopeProfile,
		WorkspaceID: resolved.ID,
		Name:        input.ServerName,
	}
	if scope.ProfileID != "" && scope.ProfileID != store.DefaultProfileID {
		profile, err = n.deps.Profiles.ProfileName(ctx, scope.ProfileID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		target.Scope = settingspkg.ScopeProfile
	}
	target.ProfileName = profile
	approval, err := service.SetMCPApproval(ctx, target, input.DefinitionDigest, input.Decision == "revoke")
	if err != nil {
		if approval, ok := errors.AsType[*settingspkg.MCPApprovalError](err); ok {
			return toolspkg.ToolResult{}, toolspkg.NewToolError(
				toolspkg.ErrorCode(approval.Code),
				req.ToolID,
				approval.Message,
				err,
			)
		}
		return toolspkg.ToolResult{}, nativeHTTPStatusToolError(req.ToolID, err, core.StatusForSettingsError(err))
	}
	state := "allowed"
	if input.Decision == "revoke" {
		state = "waiting"
	}
	return structuredResult(
		map[string]any{
			"approval": map[string]any{
				"server_name":  input.ServerName,
				"workspace_id": resolved.ID,
				"profile":      profile,
				"state":        state,
				"approved_at":  approval.ApprovedAt,
			},
		},
		state+" "+input.ServerName,
	)
}
