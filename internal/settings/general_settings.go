package settings

import (
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// GeneralSettings groups the editable general section config.
type GeneralSettings struct {
	FollowUpMode   *string
	Limits         compozyconfig.LimitsConfig
	Permissions    compozyconfig.PermissionsConfig
	SessionTimeout time.Duration
	// IdleReleaseAfter is nil when an update leaves the current value unchanged.
	IdleReleaseAfter *time.Duration
	HTTP             compozyconfig.HTTPConfig
	Daemon           compozyconfig.DaemonConfig
	Redact           compozyconfig.RedactConfig
	Terminal         compozyconfig.TerminalConfig
}

func generalSettingsFromConfig(cfg *compozyconfig.Config) GeneralSettings {
	return GeneralSettings{
		FollowUpMode:     new(cfg.Session.BusyInput.Normalize().DefaultMode),
		Limits:           cfg.Limits,
		Permissions:      cfg.Permissions,
		SessionTimeout:   cfg.Session.Limits.Timeout,
		IdleReleaseAfter: new(cfg.Session.Supervision.IdleReleaseAfter),
		HTTP:             cfg.HTTP,
		Daemon:           cfg.Daemon,
		Redact:           cfg.Redact,
		Terminal:         cfg.Terminal,
	}
}
