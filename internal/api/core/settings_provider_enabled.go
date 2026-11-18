package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
)

// SetSettingsProviderEnabled toggles providers.<name>.enabled and applies it live.
func (h *BaseHandlers) SetSettingsProviderEnabled(c *gin.Context) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%w: invalid enabled payload: %w", ErrSettingsValidation, err),
		)
		return
	}
	if body.Enabled == nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: enabled is required", ErrSettingsValidation))
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		h.respondError(
			c,
			http.StatusBadRequest,
			errors.Join(ErrSettingsValidation, errors.New("provider name is required")),
		)
		return
	}
	item, result, err := h.Settings.SetProviderEnabled(h.settingsMutationContext(c), name, *body.Enabled)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SettingsProviderEnabledResponse{
		Provider: settingsProviderItemPayload(&item),
		Apply:    SettingsApplyResponseFromResult(result),
	})
}
