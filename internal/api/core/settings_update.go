package core

import (
	"errors"
	"fmt"
	"net/http"

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
	targets, err := settingsUpdateTargets(request.Targets)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	result, err := h.SettingsUpdate.ApplyUpdate(c.Request.Context(), targets)
	if err != nil && result.Status == "" {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, SettingsUpdateApplyResponseFromResult(result))
}

func settingsUpdateTargets(rawTargets []contract.SettingsUpdateTarget) ([]compozyupdate.Target, error) {
	if len(rawTargets) == 0 {
		return nil, errors.New("settings: update targets must contain at least one target")
	}
	if len(rawTargets) > 2 {
		return nil, errors.New("settings: update targets must contain at most two targets")
	}

	targets := make([]compozyupdate.Target, 0, len(rawTargets))
	seen := make(map[compozyupdate.Target]struct{}, len(rawTargets))
	for index, rawTarget := range rawTargets {
		target := compozyupdate.Target(rawTarget)
		if target != compozyupdate.TargetRuntime && target != compozyupdate.TargetApp {
			return nil, fmt.Errorf("settings: update target %q is invalid", target)
		}
		if _, exists := seen[target]; exists {
			return nil, fmt.Errorf("settings: update targets must not contain duplicate %q", target)
		}
		if target == compozyupdate.TargetRuntime && index != 0 {
			return nil, errors.New("settings: runtime must be the first update target")
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets, nil
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
