package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/gin-gonic/gin"
)

var errSettingsUpdateUnavailable = errors.New("settings update controller is not configured")

// GetSettingsUpdate returns the current software update status snapshot.
func (h *BaseHandlers) GetSettingsUpdate(c *gin.Context) {
	if h.SettingsUpdate == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsUpdateUnavailable)
		return
	}

	status, err := h.SettingsUpdate.GetUpdate(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	c.JSON(http.StatusOK, SettingsUpdateResponseFromStatus(status))
}

// ApplySettingsUpdate durably acquires an asynchronous update operation.
func (h *BaseHandlers) ApplySettingsUpdate(c *gin.Context) {
	if h.SettingsUpdate == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsUpdateUnavailable)
		return
	}
	var request contract.SettingsUpdateApplyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	target := compozyupdate.Target(strings.TrimSpace(string(request.Target)))
	if target != compozyupdate.TargetRuntime && target != compozyupdate.TargetApp {
		h.respondError(c, http.StatusBadRequest, errors.New("settings: update target must be runtime or app"))
		return
	}
	result, err := h.SettingsUpdate.ApplyUpdate(c.Request.Context(), target)
	if err != nil && result.Status == "" {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, SettingsUpdateApplyResponseFromResult(result))
}

// CancelSettingsUpdate cancels and archives a dormant update operation.
func (h *BaseHandlers) CancelSettingsUpdate(c *gin.Context) {
	if h.SettingsUpdate == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsUpdateUnavailable)
		return
	}
	result, err := h.SettingsUpdate.CancelUpdate(c.Request.Context())
	if err != nil && result.Status == "" {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, SettingsUpdateCancelResponseFromResult(result))
}
