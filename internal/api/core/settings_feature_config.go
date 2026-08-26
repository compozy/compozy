package core

import (
	"fmt"

	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"

	compozyconfig "github.com/compozy/compozy/internal/config"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func parseSettingsDuration(path string, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, NewSettingsValidationError(fmt.Errorf("%s: %w", path, err))
	}
	return duration, nil
}

func skillsConfigFromPayload(payload contract.SettingsSkillsConfigPayload) (compozyconfig.SkillsConfig, error) {
	pollInterval, err := time.ParseDuration(strings.TrimSpace(payload.PollInterval))
	if err != nil {
		return compozyconfig.SkillsConfig{}, NewSettingsValidationError(
			fmt.Errorf("skills.config.poll_interval: %w", err),
		)
	}

	value := compozyconfig.SkillsConfig{
		Enabled:                 payload.Enabled,
		Sources:                 cloneStrings(payload.Sources),
		CustomSources:           cloneStrings(payload.CustomSources),
		DisabledSkills:          cloneStrings(payload.DisabledSkills),
		PollInterval:            pollInterval,
		AllowedMarketplaceMCP:   cloneStrings(payload.AllowedMarketplaceMCP),
		AllowedMarketplaceHooks: cloneStrings(payload.AllowedMarketplaceHooks),
		Marketplace: compozyconfig.MarketplaceConfig{
			Registry: strings.TrimSpace(payload.Marketplace.Registry),
			BaseURL:  strings.TrimSpace(payload.Marketplace.BaseURL),
		},
	}
	if err := value.Validate(); err != nil {
		return compozyconfig.SkillsConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func observabilityConfigFromPayload(
	payload contract.SettingsObservabilityConfigPayload,
) (compozyconfig.ObservabilityConfig, error) {
	value := compozyconfig.ObservabilityConfig{
		Enabled:        payload.Enabled,
		RetentionDays:  payload.RetentionDays,
		MaxGlobalBytes: payload.MaxGlobalBytes,
		Transcripts: compozyconfig.ObservabilityTranscriptConfig{
			Enabled:            payload.Transcripts.Enabled,
			SegmentBytes:       payload.Transcripts.SegmentBytes,
			MaxBytesPerSession: payload.Transcripts.MaxBytesPerSession,
		},
	}
	if err := value.Validate(); err != nil {
		return compozyconfig.ObservabilityConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func sandboxProfileFromPayload(
	payload contract.SettingsSandboxProfilePayload,
) (compozyconfig.SandboxProfile, error) {
	value := compozyconfig.SandboxProfile{
		Backend:     strings.TrimSpace(payload.Backend),
		SyncMode:    strings.TrimSpace(payload.SyncMode),
		Persistence: strings.TrimSpace(payload.Persistence),
		RuntimeRoot: strings.TrimSpace(payload.RuntimeRoot),
		Env:         cloneStringMap(payload.Env),
		SecretEnv:   cloneStringMap(payload.SecretEnv),
	}
	if payload.Network != nil {
		value.Network = compozyconfig.NetworkProfile{
			AllowPublicIngress: payload.Network.AllowPublicIngress,
			AllowOutbound:      payload.Network.AllowOutbound,
			AllowList:          cloneStrings(payload.Network.AllowList),
			DenyList:           cloneStrings(payload.Network.DenyList),
			Required:           payload.Network.Required,
		}
	}
	if payload.Daytona != nil {
		value.Daytona = compozyconfig.DaytonaProfile{
			APIURL:      strings.TrimSpace(payload.Daytona.APIURL),
			Target:      strings.TrimSpace(payload.Daytona.Target),
			Image:       strings.TrimSpace(payload.Daytona.Image),
			Snapshot:    strings.TrimSpace(payload.Daytona.Snapshot),
			Class:       strings.TrimSpace(payload.Daytona.Class),
			AutoStop:    strings.TrimSpace(payload.Daytona.AutoStop),
			AutoArchive: strings.TrimSpace(payload.Daytona.AutoArchive),
		}
	}
	if err := value.Validate("sandbox.profile"); err != nil {
		return compozyconfig.SandboxProfile{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func hookDeclarationFromPayload(
	payload contract.SettingsHookDeclarationPayload,
) (hookspkg.HookDecl, error) {
	timeout, err := parseOptionalDuration(payload.Timeout, "hooks.declaration.timeout")
	if err != nil {
		return hookspkg.HookDecl{}, err
	}
	priority, err := hookspkg.PriorityFromInt(payload.Priority)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}

	value := hookspkg.HookDecl{
		Name:         strings.TrimSpace(payload.Name),
		Event:        payload.Event,
		Source:       hookspkg.HookSourceConfig,
		Mode:         payload.Mode,
		Required:     payload.Required,
		Enabled:      payload.Enabled,
		Priority:     priority,
		PrioritySet:  payload.Priority != 0,
		Timeout:      timeout,
		Matcher:      payload.Matcher,
		ExecutorKind: payload.ExecutorKind,
		Command:      strings.TrimSpace(payload.Command),
		Args:         cloneStrings(payload.Args),
		Env:          cloneStringMap(payload.Env),
		SecretEnv:    cloneStringMap(payload.SecretEnv),
		Metadata:     cloneStringMap(payload.Metadata),
	}
	if err := hookspkg.ValidateHookDecl(value); err != nil {
		return hookspkg.HookDecl{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func parseOptionalDuration(raw string, path string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, NewSettingsValidationError(fmt.Errorf("%s: %w", path, err))
	}
	return duration, nil
}
