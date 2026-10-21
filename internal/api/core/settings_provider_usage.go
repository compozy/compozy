package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerusage"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) GetSettingsProviderUsage(c *gin.Context) {
	h.getSettingsSection(c, settingspkg.SectionProviderUsage)
}

func (h *BaseHandlers) UpdateSettingsProviderUsage(c *gin.Context) {
	req, err := parseUpdateSettingsProviderUsageRequest(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	if err := h.prepareProviderUsageConsent(c); err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	h.updateSettingsSection(c, req)
}

func settingsProviderUsageSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.ProviderUsage == nil {
		return nil, errors.New("settings provider-usage section is required")
	}
	cfg := envelope.ProviderUsage.Config
	providers := make(map[string]contract.SettingsProviderUsageProviderPayload, len(cfg.Providers))
	for id, provider := range cfg.Providers {
		value := contract.SettingsProviderUsageProviderPayload{Enabled: new(provider.Enabled)}
		if id == "claude" {
			value.KeychainAccess = new(provider.KeychainAccess)
		}
		providers[id] = value
	}
	return contract.SettingsProviderUsageResponse{
		KeychainSupported:                      providerusage.KeychainSupported(),
		SettingsUserSectionResponseMetaPayload: settingsUserSectionMetaPayload(envelope),
		Config: contract.SettingsProviderUsagePayload{
			Enabled: cfg.Enabled, RefreshInterval: cfg.RefreshInterval.String(), Providers: providers,
		},
	}, nil
}

func parseUpdateSettingsProviderUsageRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsProviderUsagePayload `json:"config"`
	}
	if err := decodeStrictJSONBody(c, &body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode provider-usage settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			errors.New("provider-usage.config is required"),
		)
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionProviderUsage)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	interval, err := time.ParseDuration(body.Config.RefreshInterval)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			errors.New("provider_usage.refresh_interval must be between 1m and 1h"),
		)
	}
	cfg := compozyconfig.ProviderUsageConfig{
		Enabled: body.Config.Enabled, RefreshInterval: interval,
		Providers: make(map[string]compozyconfig.ProviderUsageProviderConfig, len(body.Config.Providers)),
	}
	for id, value := range body.Config.Providers {
		if id != "claude" && value.KeychainAccess != nil {
			return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
				fmt.Errorf("provider_usage.providers.%s.keychain_access is only valid for claude", id),
			)
		}
		provider := compozyconfig.ProviderUsageProviderConfig{Enabled: true}
		if value.Enabled != nil {
			provider.Enabled = *value.Enabled
		}
		if value.KeychainAccess != nil {
			provider.KeychainAccess = *value.KeychainAccess
		}
		cfg.Providers[id] = provider
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, ProviderUsage: &cfg}, nil
}
