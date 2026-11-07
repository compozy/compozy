package core

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// selectableModels drops models of providers disabled in the request's effective config: the named
// workspace with the selected profile's layers, the selected profile's user config, or the active
// config (BR-2, BR-9).
func (h *BaseHandlers) selectableModels(c *gin.Context, models []modelcatalog.Model) ([]modelcatalog.Model, error) {
	cfg, err := h.selectionConfig(c)
	if err != nil {
		return nil, err
	}
	return SelectableProviderModels(&cfg, models), nil
}

func (h *BaseHandlers) selectionConfig(c *gin.Context) (compozyconfig.Config, error) {
	ctx := c.Request.Context()
	selection, err := h.resolveProfileReadSelection(c)
	if err != nil {
		return compozyconfig.Config{}, NewModelCatalogValidationError(err)
	}
	workspaceID := strings.TrimSpace(firstNonEmpty(c.Query("workspace_id"), c.Query("workspace")))
	switch {
	case workspaceID != "" && h.Workspaces != nil:
		resolve := func() (workspacepkg.ResolvedWorkspace, error) { return h.Workspaces.Resolve(ctx, workspaceID) }
		profiles, layered := h.Workspaces.(workspacepkg.ProfileRuntimeResolver)
		if name := strings.TrimSpace(selection.ProfileName); name != "" && layered {
			resolve = func() (workspacepkg.ResolvedWorkspace, error) {
				return profiles.ResolveForProfile(ctx, workspaceID, name)
			}
		}
		resolved, err := resolve()
		if err != nil {
			return compozyconfig.Config{}, fmt.Errorf("api: resolve model catalog workspace %q: %w", workspaceID, err)
		}
		return resolved.Config, nil
	case namedProfile(selection.Scope.ProfileID):
		// ponytail: a named profile's layers are read from its files, the same source session
		// admission uses; the default profile keeps the active snapshot so pending global edits
		// stay isolated (B-013). Any resolution failure keeps the active snapshot too.
		if users, ok := h.Workspaces.(workspacepkg.UserRuntimeResolver); ok {
			if resolved, err := users.ResolveUser(ctx, selection.Scope.ProfileID); err == nil {
				return resolved.Config, nil
			}
		}
	}
	return h.activeConfig(ctx)
}

// namedProfile reports a selected profile other than the default one.
func namedProfile(profileID string) bool {
	profileID = strings.TrimSpace(profileID)
	return profileID != "" && profileID != store.DefaultProfileID
}
