package core

import (
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

func parseUpdateSettingsRolesRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsRolesConfigPayload `json:"config"`
	}
	if err := decodeStrictJSONBody(c, &body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode roles settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			errSettingsRolesConfigRequired,
		)
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionRoles)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := rolesConfigFromPayload(body.Config)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, Roles: &config}, nil
}
