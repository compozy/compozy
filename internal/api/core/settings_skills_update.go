package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// UpdateSettingsSkills persists the skills settings section and returns its refreshed read model.
func (h *BaseHandlers) UpdateSettingsSkills(c *gin.Context) {
	req, err := parseUpdateSettingsSkillsRequest(c)
	if err != nil {
		h.respondSettingsSkillsError(c, err)
		return
	}
	result, ok := h.applySettingsSection(c, req, h.respondSettingsSkillsTypedError)
	if !ok {
		return
	}
	envelope, err := h.Settings.GetSection(c.Request.Context(), req.SectionRequest)
	if err != nil {
		h.respondSettingsSkillsError(c, err)
		return
	}
	payload, err := SettingsSectionResponseFromEnvelope(envelope)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	skillsPayload, ok := payload.(contract.SettingsSkillsResponse)
	if !ok {
		h.respondError(c, http.StatusInternalServerError, errors.New("settings skills response is invalid"))
		return
	}
	c.JSON(http.StatusOK, SettingsSkillsMutationResponseFromResult(result, skillsPayload))
}
