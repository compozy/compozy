package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func settingsDefaultsPayload(value compozyconfig.DefaultsConfig) contract.SettingsDefaultsPayload {
	return contract.SettingsDefaultsPayload{
		Agent: strings.TrimSpace(value.Agent), Provider: strings.TrimSpace(value.Provider),
	}
}

func defaultsConfigFromPayload(payload contract.SettingsDefaultsPayload) (compozyconfig.DefaultsConfig, error) {
	value := compozyconfig.DefaultsConfig{
		Agent: strings.TrimSpace(payload.Agent), Provider: strings.TrimSpace(payload.Provider),
	}
	if err := value.Validate(); err != nil {
		return compozyconfig.DefaultsConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}
