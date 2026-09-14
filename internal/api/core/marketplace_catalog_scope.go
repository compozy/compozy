package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

// Catalog instances combine the workspace overlay with the selected profile lens.
func parseMarketplaceCatalogScope(rawScope, rawWorkspaceID, rawProfileName string) (marketplaceReadScope, error) {
	scope := strings.ToLower(strings.TrimSpace(rawScope))
	workspaceID, profileName := strings.TrimSpace(rawWorkspaceID), strings.TrimSpace(rawProfileName)
	if scope != "" && scope != contract.MarketplaceScopeGlobal && scope != contract.MarketplaceScopeProfile &&
		scope != contract.MarketplaceScopeWorkspace {
		return marketplaceReadScope{}, marketplaceValidationf("unsupported scope %q", scope)
	}
	if profileName == profileDefaultName {
		profileName = ""
	}
	if workspaceID != "" {
		if scope == contract.MarketplaceScopeGlobal {
			return marketplaceReadScope{}, marketplaceValidationf("global scope must not include workspace_id")
		}
		return marketplaceReadScope{
			scope:       settingspkg.ScopeWorkspace,
			workspaceID: workspaceID,
			profileName: profileName,
		}, nil
	}
	if scope == contract.MarketplaceScopeWorkspace {
		return marketplaceReadScope{}, marketplaceValidationf("workspace scope requires workspace_id")
	}
	if profileName != "" {
		return marketplaceReadScope{scope: settingspkg.ScopeProfile, profileName: profileName}, nil
	}
	if scope == contract.MarketplaceScopeProfile {
		return marketplaceReadScope{}, marketplaceValidationf("profile scope requires a non-default profile")
	}
	return marketplaceReadScope{scope: settingspkg.ScopeUser}, nil
}

func (h *BaseHandlers) marketplaceCatalogReadActor(c *gin.Context, action string) (*taskpkg.ActorContext, bool) {
	scope, err := parseMarketplaceCatalogScope(c.Query("scope"), c.Query("workspace_id"), c.Query("profile"))
	if err != nil {
		h.respondMarketplaceError(c, err)
		return nil, false
	}
	return h.marketplaceReadActorForScope(c, action, scope)
}
