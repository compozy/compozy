package core

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsShellSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Shell == nil {
		return nil, errors.New("settings shell section is required")
	}
	return contract.SettingsShellResponse{
		SettingsUserSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                 settingsShellPayload(envelope.Shell.Config),
	}, nil
}

func settingsShellPayload(cfg compozyconfig.ShellConfig) contract.SettingsShellPayload {
	return contract.SettingsShellPayload{Sessions: contract.SettingsShellSessionsPayload{
		Sort:  contract.SettingsShellSessionSort(cfg.Sessions.Sort),
		Scope: contract.SettingsShellSessionScope(cfg.Sessions.Scope),
	}}
}

func shellConfigFromPayload(payload contract.SettingsShellPayload) (compozyconfig.ShellConfig, error) {
	value := compozyconfig.ShellConfig{Sessions: compozyconfig.ShellSessionsConfig{
		Sort:  compozyconfig.ShellSessionSort(payload.Sessions.Sort),
		Scope: compozyconfig.ShellSessionScope(payload.Sessions.Scope),
	}}
	if err := value.Validate(); err != nil {
		return compozyconfig.ShellConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}
