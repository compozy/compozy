package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/gin-gonic/gin"
)

// SessionCommandCatalogManager exposes one unified session command projection.
type SessionCommandCatalogManager interface {
	CommandCatalog(ctx context.Context, id string) (commandpkg.Catalog, error)
}

// GetSessionCommands lists daemon, ACP, and effective skill commands for one session.
func (h *BaseHandlers) GetSessionCommands(c *gin.Context) {
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	manager, ok := h.Sessions.(SessionCommandCatalogManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session command catalog is unavailable"))
		return
	}
	catalog, err := manager.CommandCatalog(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, SessionCommandsResponseFromCatalog(catalog))
}

// SessionCommandsResponseFromCatalog maps the internal catalog to its public path-free contract.
func SessionCommandsResponseFromCatalog(catalog commandpkg.Catalog) contract.SessionCommandsResponse {
	commands := make([]contract.SessionCommandPayload, 0, len(catalog.Commands))
	for _, descriptor := range catalog.Commands {
		placements := make([]string, 0, len(descriptor.Placements))
		for _, placement := range descriptor.Placements {
			placements = append(placements, string(placement))
		}
		commands = append(commands, contract.SessionCommandPayload{
			ID: descriptor.ID, CanonicalToken: descriptor.CanonicalToken,
			DisplayName: descriptor.DisplayName, Description: descriptor.Description,
			Lane: string(descriptor.Lane), Placements: placements, InputHint: descriptor.InputHint,
			Source: contract.SessionCommandSource{
				Kind: descriptor.Source.Kind, ID: descriptor.Source.ID,
				Key: descriptor.Source.Key, Scope: descriptor.Source.Scope,
			},
		})
	}
	return contract.SessionCommandsResponse{Commands: commands, Revision: catalog.Revision}
}
