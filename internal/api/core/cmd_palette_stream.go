package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) StreamCmdPalette(c *gin.Context) {
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	if h.CmdPalette == nil {
		h.respondCmdPaletteError(c, workspaceID, errors.New("cmd palette service is unavailable"))
		return
	}
	subscriber, ok := h.CmdPalette.(cmdpalette.EventSubscriber)
	if !ok {
		h.respondCmdPaletteError(c, workspaceID, errors.New("cmd palette event stream is unavailable"))
		return
	}
	updates, cancel, err := subscriber.SubscribeCmdPaletteEvents(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	defer cancel()
	catalog, err := h.CmdPalette.Catalog(c.Request.Context(), workspaceID, "")
	if err != nil {
		h.respondCmdPaletteError(c, workspaceID, err)
		return
	}
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := writeCmdPaletteCatalogChanged(writer, workspaceID, catalog.Revision); err != nil {
		h.logSSEWriteFailure(string(cmdpalette.EventCatalogChanged), err)
		return
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case event, open := <-updates:
			if !open {
				return
			}
			if event.Name != cmdpalette.EventCatalogChanged {
				continue
			}
			if err := writeCmdPaletteCatalogChanged(writer, event.WorkspaceID, event.CatalogRevision); err != nil {
				h.logSSEWriteFailure(string(event.Name), err)
				return
			}
		}
	}
}

func writeCmdPaletteCatalogChanged(
	writer FlushWriter,
	workspaceID cmdpalette.WorkspaceID,
	revision string,
) error {
	return WriteSSE(writer, SSEMessage{
		Name: string(cmdpalette.EventCatalogChanged),
		Data: contract.CmdPaletteCatalogChangedEvent{
			Workspace: workspaceID, CatalogRevision: revision,
		},
	})
}
