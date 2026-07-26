package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

// ListWindowManagerClients returns client-local views in one workspace partition.
func (h *BaseHandlers) ListWindowManagerClients(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	clients, err := h.WindowManager.Clients(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	payload := contract.WindowManagerClientsResponse{
		WorkspaceID: workspaceID,
		Clients:     make([]contract.WindowManagerClientView, 0, len(clients)),
	}
	for _, client := range clients {
		converted, convertErr := contract.WindowManagerClientFromDomain(client)
		if convertErr != nil {
			h.respondWindowManagerError(c, workspaceID, convertErr)
			return
		}
		payload.Clients = append(payload.Clients, converted)
	}
	c.JSON(http.StatusOK, payload)
}

// RegisterWindowManagerClient creates or refreshes one explicit client-local view.
func (h *BaseHandlers) RegisterWindowManagerClient(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	var request contract.WindowManagerClientRegistration
	if err := decodeWindowManagerJSON(c, &request); err != nil {
		h.respondWindowManagerError(
			c,
			workspaceID,
			fmt.Errorf("decode client registration: %w: %v", windowmanager.ErrInvalidCommand, err),
		)
		return
	}
	if err := validateWindowManagerWorkspace(workspaceID, request.WorkspaceID); err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	canonicalClientID := windowmanager.ClientID(strings.TrimSpace(string(request.ClientID)))
	if request.ClientID != "" && canonicalClientID == "" {
		h.respondWindowManagerError(
			c,
			workspaceID,
			fmt.Errorf("client_id must not be blank: %w", windowmanager.ErrInvalidCommand),
		)
		return
	}
	client, err := h.WindowManager.RegisterClient(c.Request.Context(), windowmanager.ClientRegistration{
		WorkspaceID: workspaceID, ClientID: canonicalClientID, ActiveDesktopID: request.ActiveDesktopID,
	})
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	converted, err := contract.WindowManagerClientFromDomain(client)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusCreated, converted)
}

// UnregisterWindowManagerClient removes transient presentation state only.
func (h *BaseHandlers) UnregisterWindowManagerClient(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	clientID, err := windowManagerClientID(c)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	if err := h.WindowManager.UnregisterClient(c.Request.Context(), workspaceID, clientID); err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	c.Status(http.StatusNoContent)
}
