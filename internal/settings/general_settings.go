package settings

import (
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

// GeneralSettings groups the editable general section config.
type GeneralSettings struct {
	Defaults       aghconfig.DefaultsConfig
	Limits         aghconfig.LimitsConfig
	Permissions    aghconfig.PermissionsConfig
	SessionTimeout time.Duration
	HTTP           aghconfig.HTTPConfig
	Daemon         aghconfig.DaemonConfig
	Redact         aghconfig.RedactConfig
}

func generalSettingsFromConfig(cfg *aghconfig.Config) GeneralSettings {
	return GeneralSettings{
		Defaults:       cfg.Defaults,
		Limits:         cfg.Limits,
		Permissions:    cfg.Permissions,
		SessionTimeout: cfg.Session.Limits.Timeout,
		HTTP:           cfg.HTTP,
		Daemon:         cfg.Daemon,
		Redact:         cfg.Redact,
	}
}
