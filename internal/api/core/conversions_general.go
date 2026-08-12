package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsGeneralConfigPayload(value settingspkg.GeneralSettings) contract.SettingsGeneralConfigPayload {
	return contract.SettingsGeneralConfigPayload{
		Defaults: contract.SettingsDefaultsPayload{
			Agent:    strings.TrimSpace(value.Defaults.Agent),
			Provider: strings.TrimSpace(value.Defaults.Provider),
			Sandbox:  strings.TrimSpace(value.Defaults.Sandbox),
		},
		Limits: contract.SettingsLimitsPayload{MaxConcurrentAgents: value.Limits.MaxConcurrentAgents},
		Permissions: contract.SettingsPermissionsPayload{
			Mode: contract.SettingsPermissionMode(value.Permissions.Mode),
		},
		SessionTimeout: value.SessionTimeout.String(),
		HTTP: contract.SettingsHTTPPayload{
			Host:              strings.TrimSpace(value.HTTP.Host),
			Port:              value.HTTP.Port,
			AllowRemoteAccess: value.HTTP.AllowRemoteAccess,
			AllowedIPs:        append([]string(nil), value.HTTP.AllowedIPs...),
		},
		Daemon: settingsDaemonPayload(value.Daemon),
		Redact: contract.SettingsRedactPayload{Enabled: value.Redact.Enabled},
	}
}
