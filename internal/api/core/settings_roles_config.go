package core

import (
	"errors"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
)

var errSettingsRolesConfigRequired = errors.New("roles.config is required")

func settingsRolesConfigPayload(value *aghconfig.RolesConfig) contract.SettingsRolesConfigPayload {
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
			Timeout:         value.MemoryController.Timeout.String(),
			TopK:            value.MemoryController.TopK,
			PromptVersion:   strings.TrimSpace(value.MemoryController.PromptVersion),
			MaxTokensOut:    value.MemoryController.MaxTokensOut,
			FallbackChain:   settingsRoleFallbackPayloads(value.MemoryController.FallbackChain),
		},
	}
}

func settingsRoleConfigPayload(value aghconfig.RoleConfig) contract.SettingsRoleConfigPayload {
	return contract.SettingsRoleConfigPayload{
		Enabled:         value.Enabled,
		Agent:           strings.TrimSpace(value.Agent),
		Provider:        strings.TrimSpace(value.Provider),
		Model:           strings.TrimSpace(value.Model),
		ReasoningEffort: strings.TrimSpace(value.ReasoningEffort),
		FallbackChain:   settingsRoleFallbackPayloads(value.FallbackChain),
	}
}

func settingsRoleFallbackPayloads(values []aghconfig.RoleFallback) []contract.SettingsRoleFallbackPayload {
	payloads := make([]contract.SettingsRoleFallbackPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsRoleFallbackPayload{
			Provider:        strings.TrimSpace(value.Provider),
			Model:           strings.TrimSpace(value.Model),
			ReasoningEffort: strings.TrimSpace(value.ReasoningEffort),
		})
	}
	return payloads
}

func rolesConfigFromPayload(payload *contract.SettingsRolesConfigPayload) (aghconfig.RolesConfig, error) {
	if payload == nil {
		return aghconfig.RolesConfig{}, NewSettingsValidationError(
			errSettingsRolesConfigRequired,
		)
	}
	ttl, err := parseSettingsDuration("roles.config.coordinator.ttl", payload.Coordinator.TTL)
	if err != nil {
		return aghconfig.RolesConfig{}, err
	}
	timeout, err := parseSettingsDuration("roles.config.memory_controller.timeout", payload.MemoryController.Timeout)
	if err != nil {
		return aghconfig.RolesConfig{}, err
	}
	return aghconfig.RolesConfig{
		Coordinator: aghconfig.CoordinatorRoleConfig{
			RoleConfig:                    roleConfigFromSettingsPayload(payload.Coordinator.SettingsRoleConfigPayload),
			TTL:                           ttl,
			MaxChildren:                   payload.Coordinator.MaxChildren,
			MaxActiveSessionsPerWorkspace: payload.Coordinator.MaxActiveSessionsPerWorkspace,
		},
		Dream:             roleConfigFromSettingsPayload(payload.Dream),
		CheckpointSummary: roleConfigFromSettingsPayload(payload.CheckpointSummary),
		MemoryExtractor:   roleConfigFromSettingsPayload(payload.MemoryExtractor),
		AutoTitle:         roleConfigFromSettingsPayload(payload.AutoTitle),
		MemoryController: aghconfig.MemoryControllerRoleConfig{
			Enabled:         payload.MemoryController.Enabled,
			Provider:        strings.TrimSpace(payload.MemoryController.Provider),
			Model:           strings.TrimSpace(payload.MemoryController.Model),
			ReasoningEffort: strings.TrimSpace(payload.MemoryController.ReasoningEffort),
			Timeout:         timeout,
			TopK:            payload.MemoryController.TopK,
			PromptVersion:   strings.TrimSpace(payload.MemoryController.PromptVersion),
			MaxTokensOut:    payload.MemoryController.MaxTokensOut,
			FallbackChain:   roleFallbacksFromSettingsPayload(payload.MemoryController.FallbackChain),
		},
	}, nil
}

func roleConfigFromSettingsPayload(payload contract.SettingsRoleConfigPayload) aghconfig.RoleConfig {
	return aghconfig.RoleConfig{
		Enabled:         payload.Enabled,
		Agent:           strings.TrimSpace(payload.Agent),
		Provider:        strings.TrimSpace(payload.Provider),
		Model:           strings.TrimSpace(payload.Model),
		ReasoningEffort: strings.TrimSpace(payload.ReasoningEffort),
		FallbackChain:   roleFallbacksFromSettingsPayload(payload.FallbackChain),
	}
}

func roleFallbacksFromSettingsPayload(values []contract.SettingsRoleFallbackPayload) []aghconfig.RoleFallback {
	fallbacks := make([]aghconfig.RoleFallback, 0, len(values))
	for _, value := range values {
		fallbacks = append(fallbacks, aghconfig.RoleFallback{
			Provider:        strings.TrimSpace(value.Provider),
			Model:           strings.TrimSpace(value.Model),
			ReasoningEffort: strings.TrimSpace(value.ReasoningEffort),
		})
	}
	return fallbacks
}
