package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	aghconfig "github.com/compozy/agh/internal/config"

	settingspkg "github.com/compozy/agh/internal/settings"
)

func settingsGlobalScopeKindsPayload(scopes []settingspkg.ScopeKind) []contract.SettingsGlobalScopeKind {
	if len(scopes) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsGlobalScopeKind, 0, len(scopes))
	for _, scope := range scopes {
		payloads = append(payloads, contract.SettingsGlobalScopeKind(scope))
	}
	return payloads
}

func settingsAgentScopeKindsPayload(scopes []settingspkg.ScopeKind) []contract.SettingsAgentScopeKind {
	if len(scopes) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsAgentScopeKind, 0, len(scopes))
	for _, scope := range scopes {
		payloads = append(payloads, contract.SettingsAgentScopeKind(scope))
	}
	return payloads
}

func settingsWorkspaceScopeKindsPayload(
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

func settingsMemoryConfigPayload(value *aghconfig.MemoryConfig) contract.SettingsMemoryConfigPayload {
	if value == nil {
		return contract.SettingsMemoryConfigPayload{}
	}
	return contract.SettingsMemoryConfigPayload{
		Enabled:    value.Enabled,
		GlobalDir:  strings.TrimSpace(value.GlobalDir),
		Controller: settingsMemoryControllerPayload(value.Controller),
		Recall:     settingsMemoryRecallPayload(value.Recall),
		Decisions:  settingsMemoryDecisionsPayload(value.Decisions),
		Extractor:  settingsMemoryExtractorPayload(value.Extractor),
		Dream:      settingsMemoryDreamPayload(value.Dream),
		Session:    settingsMemorySessionPayload(value.Session),
		Daily:      settingsMemoryDailyPayload(value.Daily),
		File:       contract.SettingsMemoryFilePayload{MaxLines: value.File.MaxLines, MaxBytes: value.File.MaxBytes},
		Provider:   settingsMemoryProviderPayload(value.Provider),
		Workspace: contract.SettingsMemoryWorkspacePayload{
			TOMLPath:   strings.TrimSpace(value.Workspace.TOMLPath),
			AutoCreate: value.Workspace.AutoCreate,
		},
	}
}

func settingsMemoryControllerPayload(value aghconfig.MemoryControllerConfig) contract.SettingsMemoryControllerPayload {
	return contract.SettingsMemoryControllerPayload{
		Mode:            strings.TrimSpace(value.Mode),
		MaxLatency:      value.MaxLatency.String(),
		DefaultOpOnFail: strings.TrimSpace(value.DefaultOpOnFail),
		Policy: contract.SettingsMemoryControllerPolicyPayload{
			MaxContentChars: value.Policy.MaxContentChars,
			MaxWritesPerMin: value.Policy.MaxWritesPerMin,
			AllowOrigins:    cloneStrings(value.Policy.AllowOrigins),
		},
	}
}

func settingsMemoryRecallPayload(value aghconfig.MemoryRecallConfig) contract.SettingsMemoryRecallPayload {
	return contract.SettingsMemoryRecallPayload{
		TopK:                   value.TopK,
		RawCandidates:          value.RawCandidates,
		Fusion:                 strings.TrimSpace(value.Fusion),
		IncludeAlreadySurfaced: value.IncludeAlreadySurfaced,
		IncludeSystem:          value.IncludeSystem,
		Weights: contract.SettingsMemoryRecallWeightsPayload{
			BM25Unicode:  value.Weights.BM25Unicode,
			BM25Trigram:  value.Weights.BM25Trigram,
			Recency:      value.Weights.Recency,
			RecallSignal: value.Weights.RecallSignal,
		},
		Freshness: contract.SettingsMemoryRecallFreshnessPayload{
			BannerAfterDays: value.Freshness.BannerAfterDays,
		},
		Signals: contract.SettingsMemoryRecallSignalsPayload{
			QueueCapacity:  value.Signals.QueueCapacity,
			WorkerRetryMax: value.Signals.WorkerRetryMax,
			MetricsEnabled: value.Signals.MetricsEnabled,
		},
	}
}

func settingsMemoryDecisionsPayload(value aghconfig.MemoryDecisionsConfig) contract.SettingsMemoryDecisionsPayload {
	return contract.SettingsMemoryDecisionsPayload{
		PruneAfterAppliedDays: value.PruneAfterAppliedDays,
		KeepAuditSummary:      value.KeepAuditSummary,
		MaxPostContentBytes:   value.MaxPostContentBytes,
	}
}

func settingsMemoryExtractorPayload(value aghconfig.MemoryExtractorConfig) contract.SettingsMemoryExtractorPayload {
	return contract.SettingsMemoryExtractorPayload{
		Mode:             strings.TrimSpace(value.Mode),
		ThrottleTurns:    value.ThrottleTurns,
		Deadline:         value.Deadline.String(),
		SandboxInboxOnly: value.SandboxInboxOnly,
		InboxPath:        strings.TrimSpace(value.InboxPath),
		DLQPath:          strings.TrimSpace(value.DLQPath),
		Queue: contract.SettingsMemoryExtractorQueuePayload{
			Capacity:    value.Queue.Capacity,
			CoalesceMax: value.Queue.CoalesceMax,
		},
	}
}

func settingsMemoryDreamPayload(value aghconfig.DreamConfig) contract.SettingsMemoryDreamPayload {
	return contract.SettingsMemoryDreamPayload{
		MinHours:      value.MinHours,
		MinSessions:   value.MinSessions,
		Debounce:      value.Debounce.String(),
		PromptVersion: strings.TrimSpace(value.PromptVersion),
		CheckInterval: value.CheckInterval.String(),
		Gates: contract.SettingsMemoryDreamGatesPayload{
			MinUnpromoted:  value.Gates.MinUnpromoted,
			MinRecallCount: value.Gates.MinRecallCount,
			MinScore:       value.Gates.MinScore,
		},
		Scoring: settingsMemoryDreamScoringPayload(value.Scoring),
	}
}

func settingsMemoryDreamScoringPayload(
	value aghconfig.MemoryDreamScoringConfig,
) contract.SettingsMemoryDreamScoringPayload {
	return contract.SettingsMemoryDreamScoringPayload{
		RecencyHalfLifeDays: value.RecencyHalfLifeDays,
		Weights: contract.SettingsMemoryDreamScoringWeightsPayload{
			Frequency: value.Weights.Frequency,
			Relevance: value.Weights.Relevance,
			Recency:   value.Weights.Recency,
			Freshness: value.Weights.Freshness,
		},
	}
}

func settingsMemorySessionPayload(value aghconfig.MemorySessionConfig) contract.SettingsMemorySessionPayload {
	return contract.SettingsMemorySessionPayload{
		LedgerFormat:     strings.TrimSpace(value.LedgerFormat),
		LedgerRoot:       strings.TrimSpace(value.LedgerRoot),
		EventsPurgeGrace: value.EventsPurgeGrace.String(),
		ColdArchiveDays:  value.ColdArchiveDays,
		HardDeleteDays:   value.HardDeleteDays,
		MaxArchiveBytes:  value.MaxArchiveBytes,
		UnboundPartition: strings.TrimSpace(value.UnboundPartition),
	}
}

func settingsMemoryDailyPayload(value aghconfig.MemoryDailyConfig) contract.SettingsMemoryDailyPayload {
	return contract.SettingsMemoryDailyPayload{
		MaxBytes:        value.MaxBytes,
		MaxLines:        value.MaxLines,
		RotateFormat:    strings.TrimSpace(value.RotateFormat),
		DreamingWindow:  value.DreamingWindow,
		ColdArchiveDays: value.ColdArchiveDays,
		HardDeleteDays:  value.HardDeleteDays,
		MaxArchiveBytes: value.MaxArchiveBytes,
		SweepHour:       value.SweepHour,
		ArchivePath:     strings.TrimSpace(value.ArchivePath),
	}
}

func settingsMemoryProviderPayload(value aghconfig.MemoryProviderConfig) contract.SettingsMemoryProviderPayload {
	return contract.SettingsMemoryProviderPayload{
		Name:             strings.TrimSpace(value.Name),
		Timeout:          value.Timeout.String(),
		FailureThreshold: value.FailureThreshold,
		Cooldown:         value.Cooldown.String(),
	}
}

func settingsSkillsConfigPayload(value aghconfig.SkillsConfig) contract.SettingsSkillsConfigPayload {
	return contract.SettingsSkillsConfigPayload{
		Enabled:                 value.Enabled,
		DisabledSkills:          cloneStrings(value.DisabledSkills),
		PollInterval:            value.PollInterval.String(),
		AllowedMarketplaceMCP:   cloneStrings(value.AllowedMarketplaceMCP),
		AllowedMarketplaceHooks: cloneStrings(value.AllowedMarketplaceHooks),
		Marketplace: contract.SettingsMarketplacePayload{
			Registry: strings.TrimSpace(value.Marketplace.Registry),
			BaseURL:  strings.TrimSpace(value.Marketplace.BaseURL),
		},
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
	value aghconfig.ObservabilityConfig,
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

func settingsMemoryHealthPayload(value settingspkg.MemoryHealthStatus) contract.SettingsMemoryHealthPayload {
	return contract.SettingsMemoryHealthPayload{
		Available:          value.Available,
		FileCount:          value.FileCount,
		DreamEnabled:       value.DreamEnabled,
		LastConsolidatedAt: cloneTimePointer(value.LastConsolidatedAt),
	}
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
