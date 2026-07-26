package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

// ExportWindowManagerLayout returns a history-free declarative document.
func (h *BaseHandlers) ExportWindowManagerLayout(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	document, err := h.WindowManager.ExportLayout(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	payload, err := contract.WindowManagerLayoutFromDomain(document)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, fmt.Errorf("encode layout document: %w", err))
		return
	}
	c.JSON(http.StatusOK, payload)
}

// ValidateWindowManagerLayout validates a raw layout without normalization or writes.
func (h *BaseHandlers) ValidateWindowManagerLayout(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	var request contract.WindowManagerLayoutValidationRequest
	if err := decodeWindowManagerJSON(c, &request); err != nil {
		h.respondWindowManagerError(
			c,
			workspaceID,
			fmt.Errorf("decode layout validation request: %w: %v", windowmanager.ErrInvalidCommand, err),
		)
		return
	}
	if err := validateWindowManagerWorkspace(workspaceID, request.WorkspaceID); err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	if err := validateWindowManagerWorkspace(workspaceID, request.Document.WorkspaceID); err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	validation, err := h.WindowManager.ValidateLayout(
		c.Request.Context(),
		workspaceID,
		request.Document.Domain(),
	)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.WindowManagerLayoutValidationResponse{
		WorkspaceID: workspaceID, Valid: validation.Valid, Diagnostics: validation.Diagnostics,
	})
}

// ReplaceWindowManagerLayout applies a declarative document through revision CAS.
func (h *BaseHandlers) ReplaceWindowManagerLayout(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	var request contract.WindowManagerLayoutReplaceRequest
	if err := decodeWindowManagerJSON(c, &request); err != nil {
		h.respondWindowManagerError(
			c,
			workspaceID,
			fmt.Errorf("decode layout replacement request: %w: %v", windowmanager.ErrInvalidCommand, err),
		)
		return
	}
	if err := validateWindowManagerWorkspace(workspaceID, request.WorkspaceID); err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	if err := validateWindowManagerWorkspace(workspaceID, request.Document.WorkspaceID); err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	if request.ExpectedRevision == nil {
		h.respondWindowManagerError(
			c,
			workspaceID,
			fmt.Errorf("expected_revision is required: %w", windowmanager.ErrInvalidCommand),
		)
		return
	}
	if request.ClientID != nil && strings.TrimSpace(string(*request.ClientID)) == "" {
		h.respondWindowManagerError(
			c,
			workspaceID,
			fmt.Errorf("client_id must not be blank: %w", windowmanager.ErrInvalidCommand),
		)
		return
	}
	result, err := h.WindowManager.ReplaceLayout(c.Request.Context(), windowmanager.ReplaceLayoutRequest{
		WorkspaceID:      workspaceID,
		ExpectedRevision: windowmanager.Revision(*request.ExpectedRevision),
		ClientID:         request.ClientID,
		Actor: windowmanager.Actor{
			Kind: strings.TrimSpace(request.Actor.Kind),
			ID:   strings.TrimSpace(request.Actor.ID),
		},
		Origin:   strings.TrimSpace(request.Origin),
		Document: request.Document.Domain(),
	})
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	payload, err := contract.WindowManagerResultFromDomain(result)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, fmt.Errorf("encode layout replacement: %w", err))
		return
	}
	c.JSON(http.StatusOK, payload)
}
