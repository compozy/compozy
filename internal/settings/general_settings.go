package settings

import (
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// GeneralSettings groups the editable general section config.
type GeneralSettings struct {
	Limits         compozyconfig.LimitsConfig
	Permissions    compozyconfig.PermissionsConfig
	SessionTimeout time.Duration
	HTTP           compozyconfig.HTTPConfig
	Daemon         compozyconfig.DaemonConfig
	Redact         compozyconfig.RedactConfig
	Terminal       compozyconfig.TerminalConfig
}

func generalSettingsFromConfig(cfg *compozyconfig.Config) GeneralSettings {
	return GeneralSettings{
		Limits:         cfg.Limits,
		Permissions:    cfg.Permissions,
		SessionTimeout: cfg.Session.Limits.Timeout,
		HTTP:           cfg.HTTP,
		Daemon:         cfg.Daemon,
		Redact:         cfg.Redact,
		Terminal:       cfg.Terminal,
	}
}
