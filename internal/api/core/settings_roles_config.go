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
		Dream:             settingsRoleConfigPayload(value.Dream),
		CheckpointSummary: settingsRoleConfigPayload(value.CheckpointSummary),
		MemoryExtractor:   settingsRoleConfigPayload(value.MemoryExtractor),
		AutoTitle:         settingsRoleConfigPayload(value.AutoTitle),
		MemoryController: contract.SettingsMemoryControllerRoleConfigPayload{
			Enabled:         value.MemoryController.Enabled,
			Provider:        strings.TrimSpace(value.MemoryController.Provider),
			Model:           strings.TrimSpace(value.MemoryController.Model),
			ReasoningEffort: strings.TrimSpace(value.MemoryController.ReasoningEffort),
			Speed:           settingsRoleSpeedPayload(value.MemoryController.Speed),
			ACPOptions:      settingsACPOptionPayloads(value.MemoryController.ACPOptions),
			Timeout:         value.MemoryController.Timeout.String(),
			TopK:            value.MemoryController.TopK,
			PromptVersion:   strings.TrimSpace(value.MemoryController.PromptVersion),
			MaxTokensOut:    value.MemoryController.MaxTokensOut,
			FallbackChain:   settingsRoleFallbackPayloads(value.MemoryController.FallbackChain),
		},
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
	timeout, err := parseSettingsDuration("roles.config.memory_controller.timeout", payload.MemoryController.Timeout)
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
		Dream:             roleConfigFromSettingsPayload(payload.Dream),
		CheckpointSummary: roleConfigFromSettingsPayload(payload.CheckpointSummary),
		MemoryExtractor:   roleConfigFromSettingsPayload(payload.MemoryExtractor),
		AutoTitle:         roleConfigFromSettingsPayload(payload.AutoTitle),
		MemoryController: compozyconfig.MemoryControllerRoleConfig{
			Enabled:         payload.MemoryController.Enabled,
			Provider:        strings.TrimSpace(payload.MemoryController.Provider),
			Model:           strings.TrimSpace(payload.MemoryController.Model),
			ReasoningEffort: strings.TrimSpace(payload.MemoryController.ReasoningEffort),
			Speed:           settingsRoleSpeedFromPayload(payload.MemoryController.Speed),
			ACPOptions:      settingsACPOptionsFromPayload(payload.MemoryController.ACPOptions),
			Timeout:         timeout,
			TopK:            payload.MemoryController.TopK,
			PromptVersion:   strings.TrimSpace(payload.MemoryController.PromptVersion),
			MaxTokensOut:    payload.MemoryController.MaxTokensOut,
			FallbackChain:   roleFallbacksFromSettingsPayload(payload.MemoryController.FallbackChain),
		},
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
