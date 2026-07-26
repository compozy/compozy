package core

import (
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	aghconfig "github.com/compozy/agh/internal/config"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func parseSettingsDuration(path string, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, NewSettingsValidationError(fmt.Errorf("%s: %w", path, err))
	}
	return duration, nil
}

func skillsConfigFromPayload(payload contract.SettingsSkillsConfigPayload) (aghconfig.SkillsConfig, error) {
	pollInterval, err := time.ParseDuration(strings.TrimSpace(payload.PollInterval))
	if err != nil {
		return aghconfig.SkillsConfig{}, NewSettingsValidationError(
			fmt.Errorf("skills.config.poll_interval: %w", err),
		)
	}

	value := aghconfig.SkillsConfig{
		Enabled:                 payload.Enabled,
		DisabledSkills:          cloneStrings(payload.DisabledSkills),
		PollInterval:            pollInterval,
		AllowedMarketplaceMCP:   cloneStrings(payload.AllowedMarketplaceMCP),
		AllowedMarketplaceHooks: cloneStrings(payload.AllowedMarketplaceHooks),
		Marketplace: aghconfig.MarketplaceConfig{
			Registry: strings.TrimSpace(payload.Marketplace.Registry),
			BaseURL:  strings.TrimSpace(payload.Marketplace.BaseURL),
		},
	}
	if err := value.Validate(); err != nil {
		return aghconfig.SkillsConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func observabilityConfigFromPayload(
	payload contract.SettingsObservabilityConfigPayload,
) (aghconfig.ObservabilityConfig, error) {
	value := aghconfig.ObservabilityConfig{
		Enabled:        payload.Enabled,
		RetentionDays:  payload.RetentionDays,
		MaxGlobalBytes: payload.MaxGlobalBytes,
		Transcripts: aghconfig.ObservabilityTranscriptConfig{
			Enabled:            payload.Transcripts.Enabled,
			SegmentBytes:       payload.Transcripts.SegmentBytes,
			MaxBytesPerSession: payload.Transcripts.MaxBytesPerSession,
		},
	}
	if err := value.Validate(); err != nil {
		return aghconfig.ObservabilityConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func sandboxProfileFromPayload(
	payload contract.SettingsSandboxProfilePayload,
) (aghconfig.SandboxProfile, error) {
	value := aghconfig.SandboxProfile{
		Backend:     strings.TrimSpace(payload.Backend),
		SyncMode:    strings.TrimSpace(payload.SyncMode),
		Persistence: strings.TrimSpace(payload.Persistence),
		RuntimeRoot: strings.TrimSpace(payload.RuntimeRoot),
		Env:         cloneStringMap(payload.Env),
		SecretEnv:   cloneStringMap(payload.SecretEnv),
	}
	if payload.Network != nil {
		value.Network = aghconfig.NetworkProfile{
			AllowPublicIngress: payload.Network.AllowPublicIngress,
			AllowOutbound:      payload.Network.AllowOutbound,
			AllowList:          cloneStrings(payload.Network.AllowList),
			DenyList:           cloneStrings(payload.Network.DenyList),
			Required:           payload.Network.Required,
		}
	}
	if payload.Daytona != nil {
		value.Daytona = aghconfig.DaytonaProfile{
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
		return aghconfig.SandboxProfile{}, NewSettingsValidationError(err)
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
