package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) GetSettingsMarketplace(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionMarketplace)
}

func (h *BaseHandlers) UpdateSettingsMarketplace(c *gin.Context) {
	var body struct {
		Config *contract.SettingsMarketplaceCatalogPayload `json:"config"`
	}
	if err := decodeStrictJSONBody(c, &body); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if body.Config == nil {
		h.respondError(c, http.StatusBadRequest, errors.New("marketplace.config is required"))
		return
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionMarketplace)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.updateSettingsSection(c, settingspkg.SectionUpdateRequest{
		SectionRequest: req,
		Marketplace: &compozyconfig.MarketplaceCatalogConfig{
			BaseURL: body.Config.BaseURL,
			TTL:     body.Config.TTL,
			Timeout: body.Config.Timeout,
		},
	})
}

func settingsMarketplaceSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Marketplace == nil {
		return nil, errors.New("settings marketplace section is required")
	}
	cfg := envelope.Marketplace
	return contract.SettingsMarketplaceResponse{
		SettingsUserSectionResponseMetaPayload: settingsUserSectionMetaPayload(envelope),
		Config: contract.SettingsMarketplaceCatalogPayload{
			BaseURL: cfg.EffectiveBaseURL(),
			TTL:     cfg.EffectiveTTL(),
			Timeout: cfg.EffectiveTimeout(),
		},
	}, nil
}
