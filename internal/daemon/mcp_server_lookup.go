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
	var extensionErr error
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
			// An unreadable extension record only matters if no manual definition answers.
			if owner == "" && definitionOwner != mcpDefinitionOwnerManual {
				extensionErr = err
				continue
			}
			return source, false, err
		}
		selection := &extension
		if definitionOwner == mcpDefinitionOwnerManual {
			selection = &manual
		}
		selection.consider(record, rank)
	}
	selected := manual
	// An unqualified name means the operator's own server first; extensions answer only when none exists.
	if selected.rank == 0 && (owner != "" || !daemonConfigDeclaresMCPServer(state, name)) {
		if extensionErr != nil {
			return source, false, extensionErr
		}
		selected = extension
	}
	if selected.rank == 0 {
		return source, false, nil
	}
	if selected.ambiguous {
		return source, false, fmt.Errorf(
			"daemon: multiple MCP definitions address %q; select an explicit owner when they come from different owners, "+
				"or rename duplicate declarations within one owner", name,
		)
	}
	source.ResourceID = selected.record.ID
	source.ResourceVersion = fmt.Sprint(selected.record.Version)
	source.RawServerName = selected.record.Spec.EffectiveRuntimeName()
	return source, true, nil
}

// mcpSourceSelection tracks the best candidate of one owner class. A tie stays a flag so it only
// fails the lookup when this class is the one that answers.
type mcpSourceSelection struct {
	record    resources.Record[compozyconfig.MCPServer]
	rank      int
	ambiguous bool
}

func (s *mcpSourceSelection) consider(record resources.Record[compozyconfig.MCPServer], rank int) {
	switch {
	case rank == 0 || rank < s.rank:
	case rank == s.rank:
		s.ambiguous = true
	default:
		s.record, s.rank, s.ambiguous = record, rank, false
	}
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
