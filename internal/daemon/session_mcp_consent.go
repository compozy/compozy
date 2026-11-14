package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionMCPConsentFilter(state *bootState) session.MCPConsentFilter {
	return func(ctx context.Context, profileID, workspaceID string, servers []config.MCPServer) ([]config.MCPServer, error) {
		var admitted []config.MCPServer
		sources, err := daemonMCPSources(ctx, state)
		if err != nil {
			return nil, err
		}
		visible := toolspkg.MCPSourcesForScope(sources, toolspkg.Scope{ProfileID: profileID, WorkspaceID: workspaceID})
		for _, server := range servers {
			source := toolspkg.SourceRef{
				Kind:          toolspkg.SourceMCP,
				ProfileID:     profileID,
				WorkspaceID:   workspaceID,
				Owner:         server.EffectiveRuntimeName(),
				RawServerName: server.Name,
			}
			if state.mcpServerCatalog != nil && server.DefinitionSource != "" {
				for _, candidate := range visible {
					for _, record := range state.mcpServerCatalog.Snapshot() {
						if candidate.ResourceID == record.ID && record.Spec.Name == server.Name &&
							record.Spec.DefinitionSource == server.DefinitionSource {
							source.ResourceID = record.ID
							break
						}
					}
					if source.ResourceID != "" {
						break
					}
				}
			}
			decision, err := mcpSourceConsent(
				ctx,
				state,
				toolspkg.Scope{ProfileID: profileID, WorkspaceID: workspaceID},
				source,
			)
			if err != nil {
				return nil, err
			}
			digest, err := config.MCPDefinitionDigest(server)
			if err != nil {
				return nil, err
			}
			if decision.Admitted && digest == decision.DefinitionDigest {
				admitted = append(admitted, server)
			}
		}
		return admitted, nil
	}
}
