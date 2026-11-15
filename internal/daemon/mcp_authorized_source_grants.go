package daemon

import (
	"context"
	"log/slog"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func operatorConsentedSourceGrants(
	ctx context.Context,
	state *bootState,
	scope toolspkg.Scope,
) ([]toolspkg.SourceGrant, error) {
	if state == nil {
		return nil, nil
	}
	sources, err := daemonMCPSources(ctx, state)
	if err != nil {
		logMCPSourceGrantSkipped(ctx, state, toolspkg.SourceRef{}, err)
		return nil, nil
	}
	var grants []toolspkg.SourceGrant
	for _, source := range toolspkg.MCPSourcesForScope(sources, scope) {
		decision, err := mcpSourceConsent(ctx, state, scope, source)
		if err != nil {
			logMCPSourceGrantSkipped(ctx, state, source, err)
			continue
		}
		if decision.Admitted {
			grants = append(
				grants,
				toolspkg.SourceGrant{Kind: toolspkg.SourceMCP, Owner: source.Owner, ResourceID: source.ResourceID},
			)
		}
	}
	if state.toolCatalog != nil {
		for _, record := range state.toolCatalog.Snapshot() {
			if record.Owner.Kind != extensionResourceOwnerKind {
				continue
			}
			profileID, workspaceID, err := mcpResourceProjectionOwner(ctx, state, record.Scope)
			if err != nil {
				return nil, err
			}
			if profileID != "" && profileID != scope.ProfileID ||
				workspaceID != "" && workspaceID != scope.WorkspaceID {
				continue
			}
			grants = append(
				grants,
				toolspkg.SourceGrant{Kind: toolspkg.SourceExtension, Owner: record.Owner.ID, ResourceID: record.ID},
			)
		}
	}
	return grants, nil
}

func logMCPSourceGrantSkipped(ctx context.Context, state *bootState, source toolspkg.SourceRef, err error) {
	logger := state.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.DebugContext(ctx, "daemon.mcp.source_grant_skipped", "server", source.RawServerName, "error", err)
}

// mcpResourceLogicalNames keeps config grant names separate from allocated runtime tool names.
func mcpResourceLogicalNames(state *bootState) map[string]string {
	if state == nil || state.mcpServerCatalog == nil {
		return nil
	}
	names := make(map[string]string)
	records := state.mcpServerCatalog.Snapshot()
	for i := range records {
		record := &records[i]
		names[record.ID] = strings.TrimSpace(record.Spec.Name)
	}
	return names
}
