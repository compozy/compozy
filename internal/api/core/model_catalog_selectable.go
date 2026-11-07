package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
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
	case strings.TrimSpace(selection.Scope.ProfileID) != "":
		if users, ok := h.Workspaces.(workspacepkg.UserRuntimeResolver); ok {
			resolved, err := users.ResolveUser(ctx, selection.Scope.ProfileID)
			switch {
			case err == nil:
				return resolved.Config, nil
			case !errors.Is(err, workspacepkg.ErrUserScopeUnsupported):
				return compozyconfig.Config{}, fmt.Errorf("api: resolve model catalog profile config: %w", err)
			}
		}
	}
	return h.activeConfig(ctx)
}
