package core

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsAttentionSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Attention == nil {
		return nil, errors.New("settings attention section is required")
	}
	return contract.SettingsAttentionResponse{
		SettingsLayeredSectionResponseMetaPayload: settingsLayeredSectionMetaPayload(envelope),
		Config: settingsAttentionPayload(envelope.Attention.Config),
	}, nil
}

func settingsAttentionPayload(cfg settingspkg.AttentionSettings) contract.SettingsAttentionPayload {
	return contract.SettingsAttentionPayload{
		Toasts:          cfg.Toasts,
		Sound:           cfg.Sound,
		System:          cfg.System,
		MutedWorkspaces: append([]string{}, cfg.MutedWorkspaces...),
	}
}

func attentionConfigFromPayload(
	payload contract.UpdateSettingsAttentionPayload,
) (settingspkg.AttentionSettings, error) {
	value := settingspkg.AttentionSettings{
		Toasts: payload.Toasts,
		Sound:  payload.Sound,
		System: payload.System,
	}
	if payload.MutedWorkspaces != nil {
		value.MutedWorkspaces = append([]string{}, (*payload.MutedWorkspaces)...)
	}
	if err := value.Validate(); err != nil {
		return settingspkg.AttentionSettings{}, NewSettingsValidationError(err)
	}
	return value, nil
}
