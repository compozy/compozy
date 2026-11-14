package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/resources"
	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (s settingsMCPConsent) PublishedMCPDefinitions(
	ctx context.Context,
	req settingspkg.MCPAuthTargetRequest,
) ([]settingspkg.MCPPublishedDefinition, error) {
	if s.state.mcpServerCatalog == nil || req.WorkspaceID == "" {
		return nil, nil
	}
	profileID, err := s.profileID(ctx, req.ProfileName)
	if err != nil {
		return nil, err
	}
	var definitions []settingspkg.MCPPublishedDefinition
	for _, record := range s.state.mcpServerCatalog.Snapshot() {
		if record.Owner.Kind == extensionResourceOwnerKind {
			continue
		}
		workspaceID, profile, scopeErr := mcpResourceProjectionWorkspace(record.Scope)
		if scopeErr != nil || workspaceID != req.WorkspaceID {
			continue
		}
		requestedProfile := req.ProfileName
		if requestedProfile == "" {
			requestedProfile = "default"
		}
		if record.Scope.Kind == resources.ResourceScopeKindWorkspaceProfile && profile != requestedProfile {
			continue
		}
		// Config declarations are read freshly by Settings, including definitions not published yet.
		if record.Source == toolMCPSyncActor().Source {
			continue
		}
		resolved, err := resolveDaemonMCPResource(
			ctx,
			s.state,
			toolspkg.SourceRef{
				Kind:        toolspkg.SourceMCP,
				ResourceID:  record.ID,
				ProfileID:   profileID,
				WorkspaceID: workspaceID,
			},
			strings.TrimSpace(record.Spec.Name),
			"manual",
		)
		if err != nil {
			return nil, err
		}
		definitions = append(
			definitions,
			settingspkg.MCPPublishedDefinition{Target: resolved.Target, Server: resolved.Server},
		)
	}
	return definitions, nil
}
