package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"

	compozyconfig "github.com/compozy/compozy/internal/config"

	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsUserScopeKindsPayload(scopes []settingspkg.ScopeKind) []contract.SettingsUserScopeKind {
	if len(scopes) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsUserScopeKind, 0, len(scopes))
	for _, scope := range scopes {
		payloads = append(payloads, contract.SettingsUserScopeKind(scope))
	}
	return payloads
}

func settingsScopeKindsPayload(scopes []settingspkg.ScopeKind) []contract.SettingsScopeKind {
	if len(scopes) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsScopeKind, 0, len(scopes))
	for _, scope := range scopes {
		payloads = append(payloads, contract.SettingsScopeKind(scope))
	}
	return payloads
}

func settingsWorkspaceScopeKindsPayload(
	scopes []settingspkg.ScopeKind,
) []contract.SettingsLayeredScopeKind {
	if len(scopes) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsLayeredScopeKind, 0, len(scopes))
	for _, scope := range scopes {
		payloads = append(payloads, contract.SettingsLayeredScopeKind(scope))
	}
	return payloads
}

func settingsUserWorkspaceScopeKindsPayload(
	scopes []settingspkg.ScopeKind,
) []contract.SettingsWorkspaceScopeKind {
	if len(scopes) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsWorkspaceScopeKind, 0, len(scopes))
	for _, scope := range scopes {
		payloads = append(payloads, contract.SettingsWorkspaceScopeKind(scope))
	}
	return payloads
}

func settingsConfigPathsPayload(paths settingspkg.ConfigPaths) contract.SettingsConfigPathsPayload {
	return contract.SettingsConfigPathsPayload{
		HomeDir:          strings.TrimSpace(paths.HomeDir),
		GlobalConfig:     strings.TrimSpace(paths.GlobalConfig),
		GlobalMCPSidecar: strings.TrimSpace(paths.GlobalMCPSidecar),
		LogFile:          strings.TrimSpace(paths.LogFile),
		DaemonInfo:       strings.TrimSpace(paths.DaemonInfo),
	}
}

func settingsSkillsConfigPayload(value compozyconfig.SkillsConfig) contract.SettingsSkillsConfigPayload {
	return contract.SettingsSkillsConfigPayload{
		Enabled:                 value.Enabled,
		Sources:                 append([]string{}, value.Sources...),
		CustomSources:           append([]string{}, value.CustomSources...),
		DisabledSkills:          cloneStrings(value.DisabledSkills),
		PollInterval:            value.PollInterval.String(),
		AllowedMarketplaceHooks: cloneStrings(value.AllowedMarketplaceHooks),
	}
}

func settingsAutomationConfigPayload(value settingspkg.AutomationSettings) contract.SettingsAutomationConfigPayload {
	return contract.SettingsAutomationConfigPayload{
		Enabled:           value.Enabled,
		Timezone:          strings.TrimSpace(value.Timezone),
		MaxConcurrentJobs: value.MaxConcurrentJobs,
		DefaultFireLimit:  value.DefaultFireLimit,
	}
}

func settingsObservabilityConfigPayload(
	value compozyconfig.ObservabilityConfig,
) contract.SettingsObservabilityConfigPayload {
	return contract.SettingsObservabilityConfigPayload{
		Enabled:        value.Enabled,
		RetentionDays:  value.RetentionDays,
		MaxGlobalBytes: value.MaxGlobalBytes,
		Transcripts: contract.SettingsObservabilityTranscriptPayload{
			Enabled:            value.Transcripts.Enabled,
			SegmentBytes:       value.Transcripts.SegmentBytes,
			MaxBytesPerSession: value.Transcripts.MaxBytesPerSession,
		},
	}
}

func settingsDaemonRuntimePayload(value settingspkg.DaemonRuntimeStatus) contract.SettingsDaemonRuntimePayload {
	payload := contract.SettingsDaemonRuntimePayload{
		Available:      value.Available,
		Status:         strings.TrimSpace(value.Status),
		PID:            value.PID,
		UptimeSeconds:  value.UptimeSeconds,
		Socket:         strings.TrimSpace(value.Socket),
		HTTPHost:       strings.TrimSpace(value.HTTPHost),
		HTTPPort:       value.HTTPPort,
		ActiveSessions: value.ActiveSessions,
		ActiveAgents:   value.ActiveAgents,
		TotalSessions:  value.TotalSessions,
		Version:        strings.TrimSpace(value.Version),
	}
	if startedAt := optionalTime(value.StartedAt); startedAt != nil {
		payload.StartedAt = startedAt
	}
	return payload
}

func settingsAutomationRuntimePayload(
	value settingspkg.AutomationRuntimeStatus,
) contract.SettingsAutomationRuntimePayload {
	return contract.SettingsAutomationRuntimePayload{
		Available:        value.Available,
		Running:          value.Running,
		SchedulerRunning: value.SchedulerRunning,
		JobTotal:         value.JobTotal,
		JobEnabled:       value.JobEnabled,
		TriggerTotal:     value.TriggerTotal,
		TriggerEnabled:   value.TriggerEnabled,
		NextFire:         cloneTimePointer(value.NextFire),
		LastSyncedAt:     cloneTimePointer(value.LastSyncedAt),
	}
}

func settingsObservabilityRuntimePayload(
	value settingspkg.ObservabilityRuntimeStatus,
) contract.SettingsObservabilityRuntimePayload {
	return contract.SettingsObservabilityRuntimePayload{
		Available:          value.Available,
		Status:             strings.TrimSpace(value.Status),
		GlobalDBSizeBytes:  value.GlobalDBSizeBytes,
		SessionDBSizeBytes: value.SessionDBSizeBytes,
		ActiveSessions:     value.ActiveSessions,
		ActiveAgents:       value.ActiveAgents,
		UptimeSeconds:      value.UptimeSeconds,
	}
}

func settingsLogTailCapabilityPayload(value settingspkg.CapabilityStatus) contract.SettingsLogTailCapabilityPayload {
	payload := contract.SettingsLogTailCapabilityPayload{Available: value.Available}
	if value.Available {
		payload.StreamURL = settingsObservabilityLogTailPath
		payload.Transport = contract.SettingsStreamTransportSSE
	}
	return payload
}

func settingsActionMetadataPayload(value settingspkg.ActionMetadata) contract.SettingsActionMetadataPayload {
	return contract.SettingsActionMetadataPayload{
		Name:      strings.TrimSpace(value.Name),
		Available: value.Available,
		Behavior:  contract.SettingsMutationBehavior(value.Behavior),
	}
}

func settingsOperationalLinkPayloads(values []settingspkg.OperationalLink) []contract.SettingsOperationalLinkPayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsOperationalLinkPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsOperationalLinkPayload{
			Label: strings.TrimSpace(value.Label),
			Path:  strings.TrimSpace(value.Path),
		})
	}
	return payloads
}

func settingsTransportParityPayload(value settingspkg.TransportParityStatus) contract.SettingsTransportParityPayload {
	return contract.SettingsTransportParityPayload{
		Known:          value.Known,
		SettingsHTTP:   value.SettingsHTTP,
		SettingsUDS:    value.SettingsUDS,
		ExtensionsHTTP: value.ExtensionsHTTP,
		ExtensionsUDS:  value.ExtensionsUDS,
	}
}
