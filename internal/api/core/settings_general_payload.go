package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

type settingsGeneralUpdateConfigPayload struct {
	contract.SettingsGeneralConfigPayload
	Redact   *settingsGeneralUpdateRedactPayload `json:"redact"`
	Terminal *contract.SettingsTerminalPayload   `json:"terminal"`
	// IdleReleaseAfter shadows the embedded string so a supplied blank value is
	// rejected instead of being mistaken for an omitted field.
	IdleReleaseAfter *string `json:"idle_release_after"`
}

type settingsGeneralUpdateRedactPayload struct {
	Enabled *bool `json:"enabled"`
}

func (p settingsGeneralUpdateConfigPayload) validatedPayload() (contract.SettingsGeneralConfigPayload, bool, error) {
	if p.Redact == nil {
		return contract.SettingsGeneralConfigPayload{}, false, NewSettingsValidationError(
			errors.New("general.config.redact is required"),
		)
	}
	if p.Redact.Enabled == nil {
		return contract.SettingsGeneralConfigPayload{}, false, NewSettingsValidationError(
			errors.New("general.config.redact.enabled is required"),
		)
	}
	p.SettingsGeneralConfigPayload.Redact.Enabled = *p.Redact.Enabled
	if p.IdleReleaseAfter != nil {
		if strings.TrimSpace(*p.IdleReleaseAfter) == "" {
			return contract.SettingsGeneralConfigPayload{}, false, NewSettingsValidationError(
				errors.New("general.config.idle_release_after must be a duration; omit it to keep the current value"),
			)
		}
		p.SettingsGeneralConfigPayload.IdleReleaseAfter = *p.IdleReleaseAfter
	}
	if p.Terminal != nil {
		p.SettingsGeneralConfigPayload.Terminal = *p.Terminal
	}
	return p.SettingsGeneralConfigPayload, p.Terminal != nil, nil
}

func generalSettingsFromPayload(
	payload contract.SettingsGeneralConfigPayload,
	terminalProvided bool,
) (settingspkg.GeneralSettings, error) {
	sessionTimeout, err := time.ParseDuration(strings.TrimSpace(payload.SessionTimeout))
	if err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(
			fmt.Errorf("general.config.session_timeout: %w", err),
		)
	}
	daemonConfig, err := daemonConfigFromPayload(payload.Daemon)
	if err != nil {
		return settingspkg.GeneralSettings{}, err
	}
	value := settingspkg.GeneralSettings{
		Limits:         compozyconfig.LimitsConfig{MaxConcurrentAgents: payload.Limits.MaxConcurrentAgents},
		Permissions:    compozyconfig.PermissionsConfig{Mode: compozyconfig.PermissionMode(payload.Permissions.Mode)},
		SessionTimeout: sessionTimeout,
		HTTP:           compozyconfig.HTTPConfig{Host: strings.TrimSpace(payload.HTTP.Host), Port: payload.HTTP.Port},
		Daemon:         daemonConfig,
		Redact:         compozyconfig.RedactConfig{Enabled: payload.Redact.Enabled},
	}
	value.FollowUpMode, err = followUpModeFromPayload(payload.BusyInput)
	if err != nil {
		return settingspkg.GeneralSettings{}, err
	}
	value.IdleReleaseAfter, err = idleReleaseAfterFromPayload(payload.IdleReleaseAfter)
	if err != nil {
		return settingspkg.GeneralSettings{}, err
	}

	if terminalProvided {
		value.Terminal, err = terminalSettingsFromPayload(payload.Terminal)
		if err != nil {
			return settingspkg.GeneralSettings{}, err
		}
	}

	if err := value.Limits.Validate(); err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(err)
	}
	if err := value.Permissions.Validate(); err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(err)
	}
	if err := (compozyconfig.SessionLimitsConfig{Timeout: value.SessionTimeout}).Validate(); err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(err)
	}
	if err := value.HTTP.Validate(); err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(err)
	}
	if err := value.Daemon.Validate(); err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(err)
	}
	if terminalProvided {
		if err := value.Terminal.Validate(); err != nil {
			return settingspkg.GeneralSettings{}, NewSettingsValidationError(err)
		}
	}
	value.Desktop, err = desktopSettingsFromPayload(payload.Desktop)
	if err != nil {
		return settingspkg.GeneralSettings{}, err
	}

	return value, nil
}

func terminalSettingsFromPayload(payload contract.SettingsTerminalPayload) (compozyconfig.TerminalConfig, error) {
	detachedTTL, err := time.ParseDuration(strings.TrimSpace(payload.DetachedTTL))
	if err != nil {
		return compozyconfig.TerminalConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.terminal.detached_ttl: %w", err),
		)
	}
	exitRetention, err := time.ParseDuration(strings.TrimSpace(payload.ExitRetention))
	if err != nil {
		return compozyconfig.TerminalConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.terminal.exit_retention: %w", err),
		)
	}
	return compozyconfig.TerminalConfig{
		DefaultShell:           strings.TrimSpace(payload.DefaultShell),
		ShellIntegration:       payload.ShellIntegration,
		ScrollbackBytes:        payload.ScrollbackBytes,
		DetachedTTL:            detachedTTL,
		ExitRetention:          exitRetention,
		Recording:              payload.Recording,
		RecordingRetentionDays: payload.RecordingRetentionDays,
		MaxPerWorkspace:        payload.MaxPerWorkspace,
		MaxPerDaemon:           payload.MaxPerDaemon,
		MaxSubscribers:         payload.MaxSubscribers,
	}, nil
}

// desktopSettingsFromPayload returns nil for an omitted block so the current value is kept.
func desktopSettingsFromPayload(payload *contract.SettingsDesktopPayload) (*compozyconfig.DesktopConfig, error) {
	if payload == nil {
		return nil, nil
	}
	desktop := compozyconfig.DesktopConfig{EditorCommand: strings.TrimSpace(payload.EditorCommand)}
	if err := desktop.Validate(); err != nil {
		return nil, NewSettingsValidationError(err)
	}
	return &desktop, nil
}

func followUpModeFromPayload(payload *contract.SettingsBusyInputPayload) (*string, error) {
	if payload == nil {
		return nil, nil
	}
	busy := compozyconfig.DefaultSessionBusyInputConfig()
	busy.DefaultMode = strings.TrimSpace(payload.DefaultMode)
	if busy.DefaultMode == "" {
		return nil, NewSettingsValidationError(errors.New("general.config.busy_input.default_mode is required"))
	}
	if err := busy.Validate(); err != nil {
		return nil, NewSettingsValidationError(err)
	}
	return new(busy.DefaultMode), nil
}

func idleReleaseAfterFromPayload(raw string) (*time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	after, err := time.ParseDuration(trimmed)
	if err != nil {
		return nil, NewSettingsValidationError(fmt.Errorf("general.config.idle_release_after: %w", err))
	}
	if after < 0 {
		return nil, NewSettingsValidationError(
			fmt.Errorf("general.config.idle_release_after must be zero or positive: %s", after),
		)
	}
	return &after, nil
}
