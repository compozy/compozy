package core

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

var errSettingsRolesConfigRequired = errors.New("roles.config is required")

func settingsRolesConfigPayload(value *compozyconfig.RolesConfig) contract.SettingsRolesConfigPayload {
	return contract.SettingsRolesConfigPayload{
		Coordinator: contract.SettingsCoordinatorRoleConfigPayload{
			SettingsRoleConfigPayload:     settingsRoleConfigPayload(value.Coordinator.RoleConfig),
			TTL:                           value.Coordinator.TTL.String(),
			MaxChildren:                   value.Coordinator.MaxChildren,
			MaxActiveSessionsPerWorkspace: value.Coordinator.MaxActiveSessionsPerWorkspace,
		},
		AutoTitle: settingsRoleConfigPayload(value.AutoTitle),
	}
}

func settingsRoleConfigPayload(value compozyconfig.RoleConfig) contract.SettingsRoleConfigPayload {
	return contract.SettingsRoleConfigPayload{
		Enabled:         value.Enabled,
		Agent:           strings.TrimSpace(value.Agent),
		Provider:        strings.TrimSpace(value.Provider),
		Model:           strings.TrimSpace(value.Model),
		ReasoningEffort: strings.TrimSpace(value.ReasoningEffort),
		Speed:           settingsRoleSpeedPayload(value.Speed),
		ACPOptions:      settingsACPOptionPayloads(value.ACPOptions),
		FallbackChain:   settingsRoleFallbackPayloads(value.FallbackChain),
	}
}

func settingsRoleFallbackPayloads(values []compozyconfig.RoleFallback) []contract.SettingsRoleFallbackPayload {
	payloads := make([]contract.SettingsRoleFallbackPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsRoleFallbackPayload{
			Provider:        strings.TrimSpace(value.Provider),
			Model:           strings.TrimSpace(value.Model),
			ReasoningEffort: strings.TrimSpace(value.ReasoningEffort),
			Speed:           settingsRoleSpeedPayload(value.Speed),
			ACPOptions:      settingsACPOptionPayloads(value.ACPOptions),
			Command:         strings.TrimSpace(value.Command),
		})
	}
	return payloads
}

func rolesConfigFromPayload(payload *contract.SettingsRolesConfigPayload) (compozyconfig.RolesConfig, error) {
	if payload == nil {
		return compozyconfig.RolesConfig{}, NewSettingsValidationError(
			errSettingsRolesConfigRequired,
		)
	}
	ttl, err := parseSettingsDuration("roles.config.coordinator.ttl", payload.Coordinator.TTL)
	if err != nil {
		return compozyconfig.RolesConfig{}, err
	}
	return compozyconfig.RolesConfig{
		Coordinator: compozyconfig.CoordinatorRoleConfig{
			RoleConfig:                    roleConfigFromSettingsPayload(payload.Coordinator.SettingsRoleConfigPayload),
			TTL:                           ttl,
			MaxChildren:                   payload.Coordinator.MaxChildren,
			MaxActiveSessionsPerWorkspace: payload.Coordinator.MaxActiveSessionsPerWorkspace,
		},
		AutoTitle: roleConfigFromSettingsPayload(payload.AutoTitle),
	}, nil
}

func roleConfigFromSettingsPayload(payload contract.SettingsRoleConfigPayload) compozyconfig.RoleConfig {
	return compozyconfig.RoleConfig{
		Enabled:         payload.Enabled,
		Agent:           strings.TrimSpace(payload.Agent),
		Provider:        strings.TrimSpace(payload.Provider),
		Model:           strings.TrimSpace(payload.Model),
		ReasoningEffort: strings.TrimSpace(payload.ReasoningEffort),
		Speed:           settingsRoleSpeedFromPayload(payload.Speed),
		ACPOptions:      settingsACPOptionsFromPayload(payload.ACPOptions),
		FallbackChain:   roleFallbacksFromSettingsPayload(payload.FallbackChain),
	}
}

func roleFallbacksFromSettingsPayload(values []contract.SettingsRoleFallbackPayload) []compozyconfig.RoleFallback {
	fallbacks := make([]compozyconfig.RoleFallback, 0, len(values))
	for _, value := range values {
		fallbacks = append(fallbacks, compozyconfig.RoleFallback{
			Provider:        strings.TrimSpace(value.Provider),
			Model:           strings.TrimSpace(value.Model),
			ReasoningEffort: strings.TrimSpace(value.ReasoningEffort),
			Speed:           settingsRoleSpeedFromPayload(value.Speed),
			ACPOptions:      settingsACPOptionsFromPayload(value.ACPOptions),
			Command:         strings.TrimSpace(value.Command),
		})
	}
	return fallbacks
}

func settingsRoleSpeedPayload(value contract.Speed) *contract.Speed {
	if strings.TrimSpace(string(value)) == "" {
		return nil
	}
	return new(value)
}

func settingsRoleSpeedFromPayload(value *contract.Speed) contract.Speed {
	if value == nil {
		return ""
	}
	return *value
}

func settingsACPOptionPayloads(
	options []compozyconfig.ACPOptionSelection,
) []contract.AgentACPOptionSelection {
	if len(options) == 0 {
		return []contract.AgentACPOptionSelection{}
	}
	payloads := make([]contract.AgentACPOptionSelection, 0, len(options))
	for _, option := range options {
		payload := contract.AgentACPOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			payload.BoolValue = new(*option.BoolValue)
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func settingsACPOptionsFromPayload(
	options []contract.AgentACPOptionSelection,
) []compozyconfig.ACPOptionSelection {
	if len(options) == 0 {
		return nil
	}
	converted := make([]compozyconfig.ACPOptionSelection, 0, len(options))
	for _, option := range options {
		selection := compozyconfig.ACPOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			selection.BoolValue = new(*option.BoolValue)
		}
		converted = append(converted, selection)
	}
	return converted
}
