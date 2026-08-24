package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

// ListWindowManagerClients returns client-local views in one workspace partition.
func (h *BaseHandlers) ListWindowManagerClients(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	service, ok := h.windowManagerService(c, workspaceID)
	if !ok {
		return
	}
	clients, err := service.Clients(c.Request.Context(), workspaceID)
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
	profileID, ok := h.windowManagerProfile(c, workspaceID)
	if !ok {
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
	// One client id belongs to one profile at a time. The claim is one operation so
	// that two registrations racing for different profiles cannot both succeed and
	// leave an attachment nobody can disambiguate (US-026).
	client, err := h.WindowManager.ClaimClient(
		c.Request.Context(),
		profileID,
		windowmanager.ClientRegistration{
			WorkspaceID: workspaceID, ClientID: canonicalClientID, Kind: request.Kind,
			ActiveDesktopID: request.ActiveDesktopID,
			Context:         windowManagerRegistrationContext(request.Context),
		},
	)
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
	service, ok := h.windowManagerService(c, workspaceID)
	if !ok {
		return
	}
	clientID, err := windowManagerClientID(c)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	if err := service.UnregisterClient(c.Request.Context(), workspaceID, clientID); err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func windowManagerRegistrationContext(
	input contract.WindowManagerClientContextInput,
) windowmanager.ClientContextInput {
	return windowmanager.ClientContextInput{
		ScopeGlobal:         input.ScopeGlobal,
		FocusedSessionState: input.FocusedSessionState,
		WorkspaceTrusted:    input.WorkspaceTrusted,
		DestinationIntent:   input.DestinationIntent,
		GlobalShortcuts:     windowmanager.CloneGlobalShortcutRegistrations(input.GlobalShortcuts),
	}
}
