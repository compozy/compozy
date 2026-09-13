package daemon

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// mcpSourceResource resolves diagnostic calls without a previously discovered resource ID.
func mcpSourceResource(
	ctx context.Context, state *bootState, source toolspkg.SourceRef,
) (toolspkg.SourceRef, bool, error) {
	owner := strings.TrimSpace(source.MCPDefinitionOwner)
	name := strings.TrimSpace(firstNonEmpty(source.RawServerName, source.Owner))
	if err := (mcpauth.Target{Scope: mcpauth.ScopeUser, ServerName: name, Owner: owner}).Normalize().
		Validate(); err != nil {
		return source, false, err
	}
	if state.mcpServerCatalog == nil {
		return source, false, nil
	}
	var selected resources.Record[compozyconfig.MCPServer]
	best := 0
	for _, record := range state.mcpServerCatalog.Snapshot() {
		definitionOwner := "manual"
		if record.Owner.Normalize().Kind == extensionResourceOwnerKind {
			definitionOwner = "extension:" + record.Owner.Normalize().ID
		}
		if owner == "" && definitionOwner != "manual" {
			continue
		}
		if owner != "" && owner != definitionOwner {
			continue
		}
		if name != strings.TrimSpace(record.Spec.Name) {
			continue
		}
		rank, err := mcpSourceResourceRank(ctx, state, source, record, definitionOwner)
		if err != nil {
			return source, false, err
		}
		if rank == 0 {
			continue
		}
		if rank < best {
			continue
		}
		if rank == best {
			return source, false, fmt.Errorf("daemon: multiple MCP definitions address %q", name)
		}
		selected, best = record, rank
	}
	if best == 0 {
		return source, false, nil
	}
	source.ResourceID = selected.ID
	source.ResourceVersion = fmt.Sprint(selected.Version)
	source.RawServerName = selected.Spec.EffectiveRuntimeName()
	return source, true, nil
}

func mcpSourceResourceRank(
	ctx context.Context, state *bootState, source toolspkg.SourceRef,
	record resources.Record[compozyconfig.MCPServer], owner string,
) (int, error) {
	profileID, workspaceID, err := mcpResourceProjectionOwner(ctx, state, record.Scope)
	if err != nil {
		return 0, err
	}
	requestedProfile := strings.TrimSpace(source.ProfileID)
	if requestedProfile == "" {
		requestedProfile = store.DefaultProfileID
	}
	if profileID == "" && owner != "manual" {
		profileID = store.DefaultProfileID
	}
	if profileID != "" && profileID != requestedProfile {
		return 0, nil
	}
	if workspaceID != "" && workspaceID != strings.TrimSpace(source.WorkspaceID) {
		return 0, nil
	}
	rank := 1
	if profileID == requestedProfile {
		rank++
	}
	if workspaceID != "" {
		rank += 2
	}
	return rank, nil
}
