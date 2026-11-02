package daemon

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type agentCatalogLens struct {
	profileID   string
	profileName string
	workspaceID string
}

type agentCatalogSnapshot struct {
	lens    agentCatalogLens
	agents  []compozyconfig.AgentDef
	present bool
}

// agentCatalogWorkspaceSnapshot retains workspace visibility and agent definitions without copying the resource graph.
func agentCatalogWorkspaceSnapshot(resolved *workspacepkg.ResolvedWorkspace) agentCatalogSnapshot {
	if resolved == nil {
		return agentCatalogSnapshot{lens: agentCatalogLensForIdentity("", "", "")}
	}
	return agentCatalogSnapshot{
		lens:    agentCatalogLensForIdentity(resolved.ID, resolved.ProfileID, resolved.ProfileName),
		agents:  resolved.Agents,
		present: true,
	}
}

// agentCatalogPolicySnapshot gives policy resolution the same visibility rules as full workspace resolution.
func agentCatalogPolicySnapshot(resolved *workspacepkg.ResolvedAgentConfig) agentCatalogSnapshot {
	if resolved == nil {
		return agentCatalogSnapshot{lens: agentCatalogLensForIdentity("", "", "")}
	}
	return agentCatalogSnapshot{
		lens:    agentCatalogLensForIdentity(resolved.ID, resolved.ProfileID, resolved.ProfileName),
		agents:  resolved.Agents,
		present: true,
	}
}

// agentCatalogLensFor uses the default profile when no workspace snapshot is available.
func agentCatalogLensFor(resolved *workspacepkg.ResolvedWorkspace) agentCatalogLens {
	return agentCatalogWorkspaceSnapshot(resolved).lens
}

// agentCatalogLensForIdentity normalizes scope identifiers while retaining default profile visibility.
func agentCatalogLensForIdentity(workspaceID, profileID, profileName string) agentCatalogLens {
	lens := agentCatalogLens{profileID: store.DefaultProfileID, profileName: daemonDefaultProfileName}
	if profileID := strings.TrimSpace(profileID); profileID != "" {
		lens.profileID = profileID
	}
	if profileName := strings.TrimSpace(profileName); profileName != "" {
		lens.profileName = profileName
	}
	lens.workspaceID = strings.TrimSpace(workspaceID)
	return lens
}

func (l agentCatalogLens) rank(scope resources.ResourceScope) (int, bool) {
	normalized := scope.Normalize()
	switch normalized.Kind {
	case resources.ResourceScopeKindUser:
		return 0, true
	case resources.ResourceScopeKindProfile:
		return 1, normalized.ID == l.profileID
	case resources.ResourceScopeKindWorkspace:
		return 2, l.workspaceID != "" && normalized.ID == l.workspaceID
	case resources.ResourceScopeKindWorkspaceProfile:
		key := l.workspaceID + "@pf:" + l.profileName
		return 3, l.workspaceID != "" && l.profileName != "" && normalized.ID == key
	default:
		return 0, false
	}
}

func (l agentCatalogLens) entryScope(scope resources.ResourceScope) (workspaceID string, workspace bool) {
	normalized := scope.Normalize()
	if normalized.Kind == resources.ResourceScopeKindWorkspace ||
		normalized.Kind == resources.ResourceScopeKindWorkspaceProfile {
		return l.workspaceID, true
	}
	return "", false
}

// Resolved workspace catalogs also publish inherited user definitions. Their visibility
// scope does not change the authored definition's memory ownership.
func (l agentCatalogLens) userAgentSources(records []resources.Record[compozyconfig.AgentDef]) map[string]bool {
	sources := make(map[string]bool)
	for i := range records {
		record := &records[i]
		_, visible := l.rank(record.Scope)
		_, workspace := l.entryScope(record.Scope)
		if visible && !workspace && record.Spec.SourcePath != "" &&
			record.Owner.Kind.Normalize() != extensionResourceOwnerKind {
			sources[record.Spec.SourcePath] = true
		}
	}
	return sources
}

func (l agentCatalogLens) memoryOrigin(
	record *resources.Record[compozyconfig.AgentDef], userSources map[string]bool,
) string {
	if record.Owner.Kind.Normalize() == extensionResourceOwnerKind {
		if record.Owner.ID == "" {
			return ""
		}
		return "extension-" + record.Owner.ID
	}
	if workspaceID, workspace := l.entryScope(record.Scope); workspace && !userSources[record.Spec.SourcePath] {
		if workspaceID == "" {
			return ""
		}
		return "workspace-" + workspaceID
	}
	return string(TurnOriginUser)
}
