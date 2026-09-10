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

func agentCatalogLensFor(resolved *workspacepkg.ResolvedWorkspace) agentCatalogLens {
	return agentCatalogWorkspaceSnapshot(resolved).lens
}

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
