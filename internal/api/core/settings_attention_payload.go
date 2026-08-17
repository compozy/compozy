package core

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsAttentionSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Attention == nil {
		return nil, errors.New("settings attention section is required")
	}
	return contract.SettingsAttentionResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                   settingsAttentionPayload(envelope.Attention.Config),
	}, nil
}

func settingsAttentionPayload(cfg compozyconfig.AttentionConfig) contract.SettingsAttentionPayload {
	return contract.SettingsAttentionPayload{
		Toasts:          cfg.Toasts,
		Sound:           cfg.Sound,
		System:          cfg.System,
		MutedWorkspaces: append([]string{}, cfg.MutedWorkspaces...),
	}
}

func attentionConfigFromPayload(
	payload contract.SettingsAttentionPayload,
) (compozyconfig.AttentionConfig, error) {
	value := compozyconfig.AttentionConfig{
		Toasts:          payload.Toasts,
		Sound:           payload.Sound,
		System:          payload.System,
		MutedWorkspaces: append([]string{}, payload.MutedWorkspaces...),
	}
	if err := value.Validate(); err != nil {
		return compozyconfig.AttentionConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}
