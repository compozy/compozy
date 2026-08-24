package core

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

// GetWindowManagerSnapshot returns the complete authorized workspace aggregate.
func (h *BaseHandlers) GetWindowManagerSnapshot(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	service, ok := h.windowManagerService(c, workspaceID)
	if !ok {
		return
	}
	snapshot, err := service.Snapshot(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	payload, err := contract.WindowManagerSnapshotFromDomain(snapshot)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, fmt.Errorf("encode window-manager snapshot: %w", err))
		return
	}
	c.JSON(http.StatusOK, payload)
}

// PreviewWindowManagerCommand validates one semantic command without persisting it.
func (h *BaseHandlers) PreviewWindowManagerCommand(c *gin.Context) {
	h.handleWindowManagerCommand(c, true)
}

// ExecuteWindowManagerCommand applies one semantic command through revision CAS.
func (h *BaseHandlers) ExecuteWindowManagerCommand(c *gin.Context) {
	h.handleWindowManagerCommand(c, false)
}

func (h *BaseHandlers) handleWindowManagerCommand(c *gin.Context, previewOnly bool) {
	workspaceID := windowManagerWorkspace(c)
	service, ok := h.windowManagerService(c, workspaceID)
	if !ok {
		return
	}
	var wire contract.WindowManagerCommandRequest
	if err := decodeWindowManagerJSON(c, &wire); err != nil {
		h.respondWindowManagerError(
			c,
			workspaceID,
			fmt.Errorf("decode command request: %w: %v", windowmanager.ErrInvalidCommand, err),
		)
		return
	}
	request, err := wire.Domain(workspaceID)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	if previewOnly {
		preview, previewErr := service.Preview(c.Request.Context(), request)
		if previewErr != nil {
			h.respondWindowManagerError(c, workspaceID, previewErr)
			return
		}
		payload, convertErr := contract.WindowManagerPreviewFromDomain(preview)
		if convertErr != nil {
			h.respondWindowManagerError(c, workspaceID, fmt.Errorf("encode command preview: %w", convertErr))
			return
		}
		c.JSON(http.StatusOK, payload)
		return
	}
	result, err := service.Execute(c.Request.Context(), request)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	payload, err := contract.WindowManagerResultFromDomain(&result)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, fmt.Errorf("encode command result: %w", err))
		return
	}
	c.JSON(http.StatusOK, payload)
}
