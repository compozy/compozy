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
	Redact *settingsGeneralUpdateRedactPayload `json:"redact"`
}

type settingsGeneralUpdateRedactPayload struct {
	Enabled *bool `json:"enabled"`
}

func (p settingsGeneralUpdateConfigPayload) validatedPayload() (contract.SettingsGeneralConfigPayload, error) {
	if p.Redact == nil {
		return contract.SettingsGeneralConfigPayload{}, NewSettingsValidationError(
			errors.New("general.config.redact is required"),
		)
	}
	if p.Redact.Enabled == nil {
		return contract.SettingsGeneralConfigPayload{}, NewSettingsValidationError(
			errors.New("general.config.redact.enabled is required"),
		)
	}
	p.SettingsGeneralConfigPayload.Redact.Enabled = *p.Redact.Enabled
	return p.SettingsGeneralConfigPayload, nil
}

func generalSettingsFromPayload(payload contract.SettingsGeneralConfigPayload) (settingspkg.GeneralSettings, error) {
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
	detachedTTL, err := time.ParseDuration(strings.TrimSpace(payload.Terminal.DetachedTTL))
	if err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(
			fmt.Errorf("general.config.terminal.detached_ttl: %w", err),
		)
	}
	exitRetention, err := time.ParseDuration(strings.TrimSpace(payload.Terminal.ExitRetention))
	if err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(
			fmt.Errorf("general.config.terminal.exit_retention: %w", err),
		)
	}

	value := settingspkg.GeneralSettings{
		Limits:         compozyconfig.LimitsConfig{MaxConcurrentAgents: payload.Limits.MaxConcurrentAgents},
		Permissions:    compozyconfig.PermissionsConfig{Mode: compozyconfig.PermissionMode(payload.Permissions.Mode)},
		SessionTimeout: sessionTimeout,
		HTTP:           compozyconfig.HTTPConfig{Host: strings.TrimSpace(payload.HTTP.Host), Port: payload.HTTP.Port},
		Daemon:         daemonConfig,
		Redact:         compozyconfig.RedactConfig{Enabled: payload.Redact.Enabled},
		Terminal: compozyconfig.TerminalConfig{
			DefaultShell:           strings.TrimSpace(payload.Terminal.DefaultShell),
			ShellIntegration:       payload.Terminal.ShellIntegration,
			ScrollbackBytes:        payload.Terminal.ScrollbackBytes,
			DetachedTTL:            detachedTTL,
			ExitRetention:          exitRetention,
			Recording:              payload.Terminal.Recording,
			RecordingRetentionDays: payload.Terminal.RecordingRetentionDays,
			MaxPerWorkspace:        payload.Terminal.MaxPerWorkspace,
			MaxPerDaemon:           payload.Terminal.MaxPerDaemon,
			MaxSubscribers:         payload.Terminal.MaxSubscribers,
		},
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
	if err := value.Terminal.Validate(); err != nil {
		return settingspkg.GeneralSettings{}, NewSettingsValidationError(err)
	}

	return value, nil
}
