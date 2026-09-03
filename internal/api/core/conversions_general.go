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
		Terminal: contract.SettingsTerminalPayload{
			DefaultShell:           value.Terminal.DefaultShell,
			ShellIntegration:       value.Terminal.ShellIntegration,
			ScrollbackBytes:        value.Terminal.ScrollbackBytes,
			DetachedTTL:            value.Terminal.DetachedTTL.String(),
			ExitRetention:          value.Terminal.ExitRetention.String(),
			Recording:              value.Terminal.Recording,
			RecordingRetentionDays: value.Terminal.RecordingRetentionDays,
			MaxPerWorkspace:        value.Terminal.MaxPerWorkspace,
			MaxPerDaemon:           value.Terminal.MaxPerDaemon,
			MaxSubscribers:         value.Terminal.MaxSubscribers,
		},
	}
}
