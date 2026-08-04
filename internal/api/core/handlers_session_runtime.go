package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// SetSessionRuntime durably selects the default runtime for future prompts.
func (h *BaseHandlers) SetSessionRuntime(c *gin.Context) {
	manager, ok := h.Sessions.(SessionRuntimeSelectionManager)
	if !ok {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			errors.New("api: session runtime selection manager is required"),
		)
		return
	}
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	var req contract.SetSessionRuntimeRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode session runtime request: %w", h.transportName(), err),
		)
		return
	}
	if req.ExpectedRevision == nil || *req.ExpectedRevision < 0 {
		h.respondError(c, http.StatusBadRequest, errors.New("expected_revision is required and must not be negative"))
		return
	}
	selection := contract.PromptRuntimeSelectionFromPayload(&req.Runtime)
	info, err := manager.SetRuntimeSelection(c.Request.Context(), sessionID, *selection, *req.ExpectedRevision)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionResponse{Session: SessionPayloadFromInfo(info)})
}

// ClearSessionRuntime removes the durable runtime preference for future prompts.
func (h *BaseHandlers) ClearSessionRuntime(c *gin.Context) {
	manager, ok := h.Sessions.(SessionRuntimeSelectionManager)
	if !ok {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			errors.New("api: session runtime selection manager is required"),
		)
		return
	}
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	expectedRevision, err := requiredRuntimeSelectionRevision(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	info, err := manager.ClearRuntimeSelection(c.Request.Context(), sessionID, expectedRevision)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionResponse{Session: SessionPayloadFromInfo(info)})
}

func requiredRuntimeSelectionRevision(c *gin.Context) (int64, error) {
	raw, ok := c.GetQuery("expected_revision")
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, errors.New("expected_revision query is required")
	}
	revision, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || revision < 0 {
		return 0, errors.New("expected_revision query must be a non-negative integer")
	}
	return revision, nil
}
