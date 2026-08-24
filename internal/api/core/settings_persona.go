package core

import (
	"github.com/gin-gonic/gin"

	settingspkg "github.com/compozy/compozy/internal/settings"
)

// GetSettingsPersona returns the profile-layerable persona defaults section.
func (h *BaseHandlers) GetSettingsPersona(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionPersona)
}

// UpdateSettingsPersona persists the profile-layerable persona defaults section.
func (h *BaseHandlers) UpdateSettingsPersona(c *gin.Context) {
	req, err := parseUpdateSettingsPersonaRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, req)
}
