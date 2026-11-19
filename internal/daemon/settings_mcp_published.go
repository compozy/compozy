package daemon

import (
	"context"
	"strings"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (s settingsMCPConsent) PublishedMCPDefinitions(
	ctx context.Context,
	req settingspkg.MCPAuthTargetRequest,
) ([]settingspkg.MCPPublishedDefinition, error) {
	if s.state.mcpServerCatalog == nil {
		return nil, nil
	}
	profileID, err := s.profileID(ctx, req.ProfileName)
	if err != nil {
		return nil, err
	}
	var definitions []settingspkg.MCPPublishedDefinition
	seen := make(map[string]struct{})
	for _, record := range s.state.mcpServerCatalog.Snapshot() {
		if record.Owner.Kind == extensionResourceOwnerKind {
			continue
		}
		target, err := mcpAuthTargetForResource(ctx, s.state, record.Scope, record.Spec.Name, record.Owner)
		if err != nil {
			return nil, err
		}
		managementTarget := publishedMCPManagementTarget(record.Scope, target)
		lens := req
		if managementTarget.Scope == mcpauth.ScopeWorkspaceProfile && req.Scope == settingspkg.ScopeWorkspace {
			lens.Scope, lens.ProfileName = settingspkg.ScopeProfile, daemonDefaultProfileName
		}
		workspaceSource := managementTarget.Scope == mcpauth.ScopeWorkspace &&
			managementTarget.WorkspaceID == req.WorkspaceID
		userSource := managementTarget.Scope == mcpauth.ScopeUser
		if !workspaceSource && !userSource && !settingsMCPExtensionTargetVisible(lens, managementTarget) {
			continue
		}
		// Config declarations are read freshly by Settings, including definitions not published yet.
		if record.Source == toolMCPSyncActor().Source {
			continue
		}
		key := publishedMCPEnablementKey(record)
		if key != "" {
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
		}
		resolved, err := resolveDaemonMCPResource(
			ctx,
			s.state,
			toolspkg.SourceRef{
				Kind:        toolspkg.SourceMCP,
				ResourceID:  record.ID,
				ProfileID:   profileID,
				WorkspaceID: req.WorkspaceID,
			},
			strings.TrimSpace(record.Spec.Name),
			"manual",
		)
		if err != nil {
			return nil, err
		}
		definitions = append(
			definitions,
			settingspkg.MCPPublishedDefinition{
				Target: resolved.Target, ManagementTarget: managementTarget, Server: resolved.Server,
			},
		)
	}
	return definitions, nil
}
