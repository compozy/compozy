package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsGeneralConfigPayload(value settingspkg.GeneralSettings) contract.SettingsGeneralConfigPayload {
	return contract.SettingsGeneralConfigPayload{
		Limits: contract.SettingsLimitsPayload{MaxConcurrentAgents: value.Limits.MaxConcurrentAgents},
		Permissions: contract.SettingsPermissionsPayload{
			Mode: contract.SettingsPermissionMode(value.Permissions.Mode),
		},
		SessionTimeout: value.SessionTimeout.String(),
		HTTP: contract.SettingsHTTPPayload{
			Host: strings.TrimSpace(value.HTTP.Host),
			Port: value.HTTP.Port,
		},
		Daemon: settingsDaemonPayload(value.Daemon),
		Redact: contract.SettingsRedactPayload{Enabled: value.Redact.Enabled},
	}
}
