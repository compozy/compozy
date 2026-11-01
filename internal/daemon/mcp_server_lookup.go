package daemon

import (
	"context"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const mcpDefinitionOwnerManual = "manual"

// mcpSourceResource resolves diagnostic calls without a previously discovered resource ID.
func mcpSourceResource(
	ctx context.Context, state *bootState, source toolspkg.SourceRef, definitionOwner string,
) (toolspkg.SourceRef, bool, error) {
	owner := strings.TrimSpace(definitionOwner)
	name := strings.TrimSpace(firstNonEmpty(source.RawServerName, source.Owner))
	if err := (mcpauth.Target{Scope: mcpauth.ScopeUser, ServerName: name, Owner: owner}).Normalize().
		Validate(); err != nil {
		return source, false, err
	}
	if state.mcpServerCatalog == nil {
		return source, false, nil
	}
	manual, extension := mcpSourceSelection{}, mcpSourceSelection{}
	for _, record := range state.mcpServerCatalog.Snapshot() {
		definitionOwner := mcpDefinitionOwnerManual
		if record.Owner.Normalize().Kind == extensionResourceOwnerKind {
			definitionOwner = "extension:" + record.Owner.Normalize().ID
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
		selection := &extension
		if definitionOwner == mcpDefinitionOwnerManual {
			selection = &manual
		}
		if err := selection.consider(record, rank, name); err != nil {
			return source, false, err
		}
	}
	selected := manual
	// An unqualified name means the operator's own server first; extensions answer only when none exists.
	if selected.rank == 0 && (owner != "" || !daemonConfigDeclaresMCPServer(state, name)) {
		selected = extension
	}
	if selected.rank == 0 {
		return source, false, nil
	}
	source.ResourceID = selected.record.ID
	source.ResourceVersion = fmt.Sprint(selected.record.Version)
	source.RawServerName = selected.record.Spec.EffectiveRuntimeName()
	return source, true, nil
}

type mcpSourceSelection struct {
	record resources.Record[compozyconfig.MCPServer]
	rank   int
}

func (s *mcpSourceSelection) consider(
	record resources.Record[compozyconfig.MCPServer], rank int, name string,
) error {
	if rank == 0 || rank < s.rank {
		return nil
	}
	if rank == s.rank {
		return fmt.Errorf("daemon: multiple MCP definitions address %q; pass an explicit owner", name)
	}
	s.record, s.rank = record, rank
	return nil
}

func daemonConfigDeclaresMCPServer(state *bootState, name string) bool {
	matches := func(server compozyconfig.MCPServer) bool { return strings.TrimSpace(server.Name) == name }
	if slices.ContainsFunc(state.cfg.MCPServers, matches) {
		return true
	}
	for _, provider := range state.cfg.Providers {
		if slices.ContainsFunc(provider.MCPServers, matches) {
			return true
		}
	}
	return false
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
	if profileID == "" && owner != mcpDefinitionOwnerManual {
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
