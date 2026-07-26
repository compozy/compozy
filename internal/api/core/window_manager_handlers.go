package core

import (
	"fmt"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

// GetWindowManagerSnapshot returns the complete authorized workspace aggregate.
func (h *BaseHandlers) GetWindowManagerSnapshot(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	snapshot, err := h.WindowManager.Snapshot(c.Request.Context(), workspaceID)
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
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
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
		preview, previewErr := h.WindowManager.Preview(c.Request.Context(), request)
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
	result, err := h.WindowManager.Execute(c.Request.Context(), request)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	payload, err := contract.WindowManagerResultFromDomain(result)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, fmt.Errorf("encode command result: %w", err))
		return
	}
	c.JSON(http.StatusOK, payload)
}
