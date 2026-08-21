package core

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/gin-gonic/gin"
)

// GetAutomationRun returns one automation run by id.
func (h *BaseHandlers) GetAutomationRun(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	run, err := manager.GetRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if err := h.populateAutomationRunProfileID(c.Request.Context(), manager, &run); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !readScope.Matches(run.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrRunNotFound)
		return
	}
	payload := RunPayloadFromRun(run)
	if err := h.decorateAutomationRunOwner(c.Request.Context(), &payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RunResponse{Run: payload})
}
