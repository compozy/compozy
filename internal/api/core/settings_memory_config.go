package core

import (
	"errors"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	aghconfig "github.com/compozy/agh/internal/config"
)

func parseSettingsDurationOrDefault(raw string, defaultValue time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultValue, nil
	}
	return time.ParseDuration(trimmed)
}

func memoryConfigFromPayload(payload *contract.SettingsMemoryConfigPayload) (aghconfig.MemoryConfig, error) {
	if payload == nil {
		return aghconfig.MemoryConfig{}, NewSettingsValidationError(errors.New("memory.config is required"))
	}
	controller, err := memoryControllerConfigFromPayload(payload.Controller)
	if err != nil {
		return aghconfig.MemoryConfig{}, err
	}
	extractor, err := memoryExtractorConfigFromPayload(payload.Extractor)
	if err != nil {
		return aghconfig.MemoryConfig{}, err
	}
	dream, err := memoryDreamConfigFromPayload(payload.Dream)
	if err != nil {
		return aghconfig.MemoryConfig{}, err
	}
	session, err := memorySessionConfigFromPayload(payload.Session)
	if err != nil {
		return aghconfig.MemoryConfig{}, err
	}
	provider, err := memoryProviderConfigFromPayload(payload.Provider)
	if err != nil {
		return aghconfig.MemoryConfig{}, err
	}

	value := aghconfig.MemoryConfig{
		Enabled:    payload.Enabled,
		GlobalDir:  strings.TrimSpace(payload.GlobalDir),
		Controller: controller,
		Recall:     memoryRecallConfigFromPayload(payload.Recall),
		Decisions:  memoryDecisionsConfigFromPayload(payload.Decisions),
		Extractor:  extractor,
		Dream:      dream,
		Session:    session,
		Daily:      memoryDailyConfigFromPayload(payload.Daily),
		File: aghconfig.MemoryFileConfig{
			MaxLines: payload.File.MaxLines,
			MaxBytes: payload.File.MaxBytes,
		},
		Provider: provider,
		Workspace: aghconfig.MemoryWorkspaceConfig{
			TOMLPath:   strings.TrimSpace(payload.Workspace.TOMLPath),
			AutoCreate: payload.Workspace.AutoCreate,
		},
	}
	if err := value.Validate(); err != nil {
		return aghconfig.MemoryConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func memoryControllerConfigFromPayload(
	payload contract.SettingsMemoryControllerPayload,
) (aghconfig.MemoryControllerConfig, error) {
	maxLatency, err := parseSettingsDuration("memory.config.controller.max_latency", payload.MaxLatency)
	if err != nil {
		return aghconfig.MemoryControllerConfig{}, err
	}
	return aghconfig.MemoryControllerConfig{
		Mode:            strings.TrimSpace(payload.Mode),
		MaxLatency:      maxLatency,
		DefaultOpOnFail: strings.TrimSpace(payload.DefaultOpOnFail),
		Policy: aghconfig.MemoryControllerPolicyConfig{
			MaxContentChars: payload.Policy.MaxContentChars,
			MaxWritesPerMin: payload.Policy.MaxWritesPerMin,
			AllowOrigins:    cloneStrings(payload.Policy.AllowOrigins),
		},
	}, nil
}

func memoryRecallConfigFromPayload(payload contract.SettingsMemoryRecallPayload) aghconfig.MemoryRecallConfig {
	return aghconfig.MemoryRecallConfig{
		TopK:                   payload.TopK,
		RawCandidates:          payload.RawCandidates,
		Fusion:                 strings.TrimSpace(payload.Fusion),
		IncludeAlreadySurfaced: payload.IncludeAlreadySurfaced,
		IncludeSystem:          payload.IncludeSystem,
		Weights: aghconfig.MemoryRecallWeightsConfig{
			BM25Unicode:  payload.Weights.BM25Unicode,
			BM25Trigram:  payload.Weights.BM25Trigram,
			Recency:      payload.Weights.Recency,
			RecallSignal: payload.Weights.RecallSignal,
		},
		Freshness: aghconfig.MemoryRecallFreshnessConfig{
			BannerAfterDays: payload.Freshness.BannerAfterDays,
		},
		Signals: aghconfig.MemoryRecallSignalsConfig{
			QueueCapacity:  payload.Signals.QueueCapacity,
			WorkerRetryMax: payload.Signals.WorkerRetryMax,
			MetricsEnabled: payload.Signals.MetricsEnabled,
		},
	}
}

func memoryDecisionsConfigFromPayload(payload contract.SettingsMemoryDecisionsPayload) aghconfig.MemoryDecisionsConfig {
	return aghconfig.MemoryDecisionsConfig{
		PruneAfterAppliedDays: payload.PruneAfterAppliedDays,
		KeepAuditSummary:      payload.KeepAuditSummary,
		MaxPostContentBytes:   payload.MaxPostContentBytes,
	}
}

func memoryExtractorConfigFromPayload(
	payload contract.SettingsMemoryExtractorPayload,
) (aghconfig.MemoryExtractorConfig, error) {
	deadline, err := parseSettingsDuration("memory.config.extractor.deadline", payload.Deadline)
	if err != nil {
		return aghconfig.MemoryExtractorConfig{}, err
	}
	return aghconfig.MemoryExtractorConfig{
		Mode:             strings.TrimSpace(payload.Mode),
		ThrottleTurns:    payload.ThrottleTurns,
		Deadline:         deadline,
		SandboxInboxOnly: payload.SandboxInboxOnly,
		InboxPath:        strings.TrimSpace(payload.InboxPath),
		DLQPath:          strings.TrimSpace(payload.DLQPath),
		Queue: aghconfig.MemoryExtractorQueueConfig{
			Capacity:    payload.Queue.Capacity,
			CoalesceMax: payload.Queue.CoalesceMax,
		},
	}, nil
}

func memoryDreamConfigFromPayload(payload contract.SettingsMemoryDreamPayload) (aghconfig.DreamConfig, error) {
	debounce, err := parseSettingsDuration("memory.config.dream.debounce", payload.Debounce)
	if err != nil {
		return aghconfig.DreamConfig{}, err
	}
	checkInterval, err := parseSettingsDuration("memory.config.dream.check_interval", payload.CheckInterval)
	if err != nil {
		return aghconfig.DreamConfig{}, err
	}
	return aghconfig.DreamConfig{
		MinHours:      payload.MinHours,
		MinSessions:   payload.MinSessions,
		Debounce:      debounce,
		PromptVersion: strings.TrimSpace(payload.PromptVersion),
		CheckInterval: checkInterval,
		Gates: aghconfig.MemoryDreamGatesConfig{
			MinUnpromoted:  payload.Gates.MinUnpromoted,
			MinRecallCount: payload.Gates.MinRecallCount,
			MinScore:       payload.Gates.MinScore,
		},
		Scoring: memoryDreamScoringConfigFromPayload(payload.Scoring),
	}, nil
}

func memoryDreamScoringConfigFromPayload(
	payload contract.SettingsMemoryDreamScoringPayload,
) aghconfig.MemoryDreamScoringConfig {
	return aghconfig.MemoryDreamScoringConfig{
		RecencyHalfLifeDays: payload.RecencyHalfLifeDays,
		Weights: aghconfig.MemoryDreamScoringWeightsConfig{
			Frequency: payload.Weights.Frequency,
			Relevance: payload.Weights.Relevance,
			Recency:   payload.Weights.Recency,
			Freshness: payload.Weights.Freshness,
		},
	}
}

func memorySessionConfigFromPayload(
	payload contract.SettingsMemorySessionPayload,
) (aghconfig.MemorySessionConfig, error) {
	eventsPurgeGrace, err := parseSettingsDuration(
		"memory.config.session.events_purge_grace",
		payload.EventsPurgeGrace,
	)
	if err != nil {
		return aghconfig.MemorySessionConfig{}, err
	}
	return aghconfig.MemorySessionConfig{
		LedgerFormat:     strings.TrimSpace(payload.LedgerFormat),
		LedgerRoot:       strings.TrimSpace(payload.LedgerRoot),
		EventsPurgeGrace: eventsPurgeGrace,
		ColdArchiveDays:  payload.ColdArchiveDays,
		HardDeleteDays:   payload.HardDeleteDays,
		MaxArchiveBytes:  payload.MaxArchiveBytes,
		UnboundPartition: strings.TrimSpace(payload.UnboundPartition),
	}, nil
}

func memoryDailyConfigFromPayload(payload contract.SettingsMemoryDailyPayload) aghconfig.MemoryDailyConfig {
	return aghconfig.MemoryDailyConfig{
		MaxBytes:        payload.MaxBytes,
		MaxLines:        payload.MaxLines,
		RotateFormat:    strings.TrimSpace(payload.RotateFormat),
		DreamingWindow:  payload.DreamingWindow,
		ColdArchiveDays: payload.ColdArchiveDays,
		HardDeleteDays:  payload.HardDeleteDays,
		MaxArchiveBytes: payload.MaxArchiveBytes,
		SweepHour:       payload.SweepHour,
		ArchivePath:     strings.TrimSpace(payload.ArchivePath),
	}
}

func memoryProviderConfigFromPayload(
	payload contract.SettingsMemoryProviderPayload,
) (aghconfig.MemoryProviderConfig, error) {
	timeout, err := parseSettingsDuration("memory.config.provider.timeout", payload.Timeout)
	if err != nil {
		return aghconfig.MemoryProviderConfig{}, err
	}
	cooldown, err := parseSettingsDuration("memory.config.provider.cooldown", payload.Cooldown)
	if err != nil {
		return aghconfig.MemoryProviderConfig{}, err
	}
	return aghconfig.MemoryProviderConfig{
		Name:             strings.TrimSpace(payload.Name),
		Timeout:          timeout,
		FailureThreshold: payload.FailureThreshold,
		Cooldown:         cooldown,
	}, nil
}
