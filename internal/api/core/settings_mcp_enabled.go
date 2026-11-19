package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

type mcpSettingsEnablement interface {
	SetMCPServerEnabled(context.Context, settingspkg.MCPAuthTargetRequest, bool) (settingspkg.MCPServerItem, error)
}

func (h *BaseHandlers) SetSettingsMCPServerEnabled(c *gin.Context) {
	service, ok := h.Settings.(mcpSettingsEnablement)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	target, err := parseSettingsMCPAuthTarget(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeStrictJSONBody(c, &body); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if body.Enabled == nil {
		h.respondError(c, http.StatusBadRequest, NewSettingsValidationError(errors.New("enabled is required")))
		return
	}
	item, err := service.SetMCPServerEnabled(h.settingsMutationContext(c), target, *body.Enabled)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SettingsMCPServerResponse{
		Server: settingsMCPServerItemPayloads([]settingspkg.MCPServerItem{item})[0],
	})
}
