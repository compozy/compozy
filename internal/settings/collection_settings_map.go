package settings

import (
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

func providerSettingsMap(settings ProviderSettings) map[string]any {
	values := make(map[string]any)
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
	if strings.TrimSpace(settings.AuthLoginCmd) != "" {
		values["auth_login_command"] = strings.TrimSpace(settings.AuthLoginCmd)
	}
	if len(settings.CredentialSlots) > 0 {
		values["credential_slots"] = providerCredentialSlotMaps(settings.CredentialSlots)
	}
	return values
}

func providerCredentialSlotMaps(slots []aghconfig.ProviderCredentialSlot) []map[string]any {
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

func sandboxProfileMap(profile aghconfig.SandboxProfile) map[string]any {
	values := map[string]any{
		"backend": profile.Backend,
	}
	if strings.TrimSpace(profile.SyncMode) != "" {
		values["sync_mode"] = profile.SyncMode
	}
	if strings.TrimSpace(profile.Persistence) != "" {
		values["persistence"] = profile.Persistence
	}
	if strings.TrimSpace(profile.RuntimeRoot) != "" {
		values["runtime_root"] = profile.RuntimeRoot
	}
	if len(profile.Env) > 0 {
		values[settingsCredentialSourceEnv] = cloneStringMap(profile.Env)
	}
	if len(profile.SecretEnv) > 0 {
		values["secret_env"] = cloneStringMap(profile.SecretEnv)
	}
	if network := networkProfileMap(profile.Network); len(network) > 0 {
		values["network"] = network
	}
	if daytona := daytonaProfileMap(profile.Daytona); len(daytona) > 0 {
		values["daytona"] = daytona
	}
	return values
}

func networkProfileMap(profile aghconfig.NetworkProfile) map[string]any {
	if !profile.AllowPublicIngress &&
		!profile.AllowOutbound &&
		!profile.Required &&
		len(profile.AllowList) == 0 &&
		len(profile.DenyList) == 0 {
		return nil
	}

	network := map[string]any{
		"allow_public_ingress": profile.AllowPublicIngress,
		"allow_outbound":       profile.AllowOutbound,
		"required":             profile.Required,
	}
	if len(profile.AllowList) > 0 {
		network["allow_list"] = append([]string(nil), profile.AllowList...)
	}
	if len(profile.DenyList) > 0 {
		network["deny_list"] = append([]string(nil), profile.DenyList...)
	}
	return network
}

func daytonaProfileMap(profile aghconfig.DaytonaProfile) map[string]any {
	values := map[string]any{}
	if strings.TrimSpace(profile.APIURL) != "" {
		values["api_url"] = profile.APIURL
	}
	if strings.TrimSpace(profile.Target) != "" {
		values["target"] = profile.Target
	}
	if strings.TrimSpace(profile.Image) != "" {
		values["image"] = profile.Image
	}
	if strings.TrimSpace(profile.Snapshot) != "" {
		values["snapshot"] = profile.Snapshot
	}
	if strings.TrimSpace(profile.Class) != "" {
		values["class"] = profile.Class
	}
	if strings.TrimSpace(profile.AutoStop) != "" {
		values["auto_stop"] = profile.AutoStop
	}
	if strings.TrimSpace(profile.AutoArchive) != "" {
		values["auto_archive"] = profile.AutoArchive
	}
	return values
}
