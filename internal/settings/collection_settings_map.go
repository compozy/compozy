package settings

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func providerSettingsMap(settings ProviderSettings) map[string]any {
	values := make(map[string]any)
	if settings.SteerCapability != "" {
		values["steer_capability"] = string(settings.SteerCapability)
	}
	if strings.TrimSpace(settings.Command) != "" {
		values["command"] = strings.TrimSpace(settings.Command)
	}
	if strings.TrimSpace(settings.DisplayName) != "" {
		values["display_name"] = strings.TrimSpace(settings.DisplayName)
	}
	if models := providerModelsSettingsMap(settings.Models); len(models) > 0 {
		values["models"] = models
	}
	if settings.Harness != "" {
		values["harness"] = string(settings.Harness)
	}
	if strings.TrimSpace(settings.RuntimeProvider) != "" {
		values["runtime_provider"] = strings.TrimSpace(settings.RuntimeProvider)
	}
	if strings.TrimSpace(settings.Transport) != "" {
		values["transport"] = strings.TrimSpace(settings.Transport)
	}
	if strings.TrimSpace(settings.BaseURL) != "" {
		values["base_url"] = strings.TrimSpace(settings.BaseURL)
	}
	if settings.AuthMode != "" {
		values["auth_mode"] = string(settings.AuthMode)
	}
	if settings.EnvPolicy != "" {
		values["env_policy"] = string(settings.EnvPolicy)
	}
	if settings.HomePolicy != "" {
		values["home_policy"] = string(settings.HomePolicy)
	}
	if strings.TrimSpace(settings.AuthStatusCmd) != "" {
		values["auth_status_command"] = strings.TrimSpace(settings.AuthStatusCmd)
	}
	if settings.AuthLoginCmdSet || strings.TrimSpace(settings.AuthLoginCmd) != "" {
		values["auth_login_command"] = strings.TrimSpace(settings.AuthLoginCmd)
	}
	if len(settings.CredentialSlots) > 0 {
		values["credential_slots"] = providerCredentialSlotMaps(settings.CredentialSlots)
	}
	return values
}

func providerCredentialSlotMaps(slots []compozyconfig.ProviderCredentialSlot) []map[string]any {
	values := make([]map[string]any, 0, len(slots))
	for _, slot := range slots {
		value := make(map[string]any)
		if strings.TrimSpace(slot.Name) != "" {
			value["name"] = strings.TrimSpace(slot.Name)
		}
		if strings.TrimSpace(slot.TargetEnv) != "" {
			value["target_env"] = strings.TrimSpace(slot.TargetEnv)
		}
		if strings.TrimSpace(slot.SecretRef) != "" {
			value["secret_ref"] = strings.TrimSpace(slot.SecretRef)
		}
		if strings.TrimSpace(slot.Kind) != "" {
			value["kind"] = strings.TrimSpace(slot.Kind)
		}
		value["required"] = slot.Required
		if len(value) > 1 {
			values = append(values, value)
		}
	}
	return values
}
