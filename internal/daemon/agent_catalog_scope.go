package daemon

import (
	"strings"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type agentCatalogLens struct {
	profileID   string
	profileName string
	workspaceID string
}

func agentCatalogLensFor(resolved *workspacepkg.ResolvedWorkspace) agentCatalogLens {
	lens := agentCatalogLens{profileID: store.DefaultProfileID, profileName: daemonDefaultProfileName}
	if resolved == nil {
		return lens
	}
	if profileID := strings.TrimSpace(resolved.ProfileID); profileID != "" {
		lens.profileID = profileID
	}
	if profileName := strings.TrimSpace(resolved.ProfileName); profileName != "" {
		lens.profileName = profileName
	}
	lens.workspaceID = strings.TrimSpace(resolved.ID)
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
