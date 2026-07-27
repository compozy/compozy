package core

import (
	"errors"

	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func parseSettingsDurationOrDefault(raw string, defaultValue time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultValue, nil
	}
	return time.ParseDuration(trimmed)
}

func memoryConfigFromPayload(payload *contract.SettingsMemoryConfigPayload) (compozyconfig.MemoryConfig, error) {
	if payload == nil {
		return compozyconfig.MemoryConfig{}, NewSettingsValidationError(errors.New("memory.config is required"))
	}
	controller, err := memoryControllerConfigFromPayload(payload.Controller)
	if err != nil {
		return compozyconfig.MemoryConfig{}, err
	}
	extractor, err := memoryExtractorConfigFromPayload(payload.Extractor)
	if err != nil {
		return compozyconfig.MemoryConfig{}, err
	}
	dream, err := memoryDreamConfigFromPayload(payload.Dream)
	if err != nil {
		return compozyconfig.MemoryConfig{}, err
	}
	session, err := memorySessionConfigFromPayload(payload.Session)
	if err != nil {
		return compozyconfig.MemoryConfig{}, err
	}
	provider, err := memoryProviderConfigFromPayload(payload.Provider)
	if err != nil {
		return compozyconfig.MemoryConfig{}, err
	}

	value := compozyconfig.MemoryConfig{
		Enabled:    payload.Enabled,
		GlobalDir:  strings.TrimSpace(payload.GlobalDir),
		Controller: controller,
		Recall:     memoryRecallConfigFromPayload(payload.Recall),
		Decisions:  memoryDecisionsConfigFromPayload(payload.Decisions),
		Extractor:  extractor,
		Dream:      dream,
		Session:    session,
		Daily:      memoryDailyConfigFromPayload(payload.Daily),
		File: compozyconfig.MemoryFileConfig{
			MaxLines: payload.File.MaxLines,
			MaxBytes: payload.File.MaxBytes,
		},
		Provider: provider,
		Workspace: compozyconfig.MemoryWorkspaceConfig{
			TOMLPath:   strings.TrimSpace(payload.Workspace.TOMLPath),
			AutoCreate: payload.Workspace.AutoCreate,
		},
	}
	if err := value.Validate(); err != nil {
		return compozyconfig.MemoryConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func memoryControllerConfigFromPayload(
	payload contract.SettingsMemoryControllerPayload,
) (compozyconfig.MemoryControllerConfig, error) {
	maxLatency, err := parseSettingsDuration("memory.config.controller.max_latency", payload.MaxLatency)
	if err != nil {
		return compozyconfig.MemoryControllerConfig{}, err
	}
	return compozyconfig.MemoryControllerConfig{
		Mode:            strings.TrimSpace(payload.Mode),
		MaxLatency:      maxLatency,
		DefaultOpOnFail: strings.TrimSpace(payload.DefaultOpOnFail),
		Policy: compozyconfig.MemoryControllerPolicyConfig{
			MaxContentChars: payload.Policy.MaxContentChars,
			MaxWritesPerMin: payload.Policy.MaxWritesPerMin,
			AllowOrigins:    cloneStrings(payload.Policy.AllowOrigins),
		},
	}, nil
}

func memoryRecallConfigFromPayload(payload contract.SettingsMemoryRecallPayload) compozyconfig.MemoryRecallConfig {
	return compozyconfig.MemoryRecallConfig{
		TopK:                   payload.TopK,
		RawCandidates:          payload.RawCandidates,
		Fusion:                 strings.TrimSpace(payload.Fusion),
		IncludeAlreadySurfaced: payload.IncludeAlreadySurfaced,
		IncludeSystem:          payload.IncludeSystem,
		Weights: compozyconfig.MemoryRecallWeightsConfig{
			BM25Unicode:  payload.Weights.BM25Unicode,
			BM25Trigram:  payload.Weights.BM25Trigram,
			Recency:      payload.Weights.Recency,
			RecallSignal: payload.Weights.RecallSignal,
		},
		Freshness: compozyconfig.MemoryRecallFreshnessConfig{
			BannerAfterDays: payload.Freshness.BannerAfterDays,
		},
		Signals: compozyconfig.MemoryRecallSignalsConfig{
			QueueCapacity:  payload.Signals.QueueCapacity,
			WorkerRetryMax: payload.Signals.WorkerRetryMax,
			MetricsEnabled: payload.Signals.MetricsEnabled,
		},
	}
}

func memoryDecisionsConfigFromPayload(
	payload contract.SettingsMemoryDecisionsPayload,
) compozyconfig.MemoryDecisionsConfig {
	return compozyconfig.MemoryDecisionsConfig{
		PruneAfterAppliedDays: payload.PruneAfterAppliedDays,
		KeepAuditSummary:      payload.KeepAuditSummary,
		MaxPostContentBytes:   payload.MaxPostContentBytes,
	}
}

func memoryExtractorConfigFromPayload(
	payload contract.SettingsMemoryExtractorPayload,
) (compozyconfig.MemoryExtractorConfig, error) {
	deadline, err := parseSettingsDuration("memory.config.extractor.deadline", payload.Deadline)
	if err != nil {
		return compozyconfig.MemoryExtractorConfig{}, err
	}
	return compozyconfig.MemoryExtractorConfig{
		Mode:             strings.TrimSpace(payload.Mode),
		ThrottleTurns:    payload.ThrottleTurns,
		Deadline:         deadline,
		SandboxInboxOnly: payload.SandboxInboxOnly,
		InboxPath:        strings.TrimSpace(payload.InboxPath),
		DLQPath:          strings.TrimSpace(payload.DLQPath),
		Queue: compozyconfig.MemoryExtractorQueueConfig{
			Capacity:    payload.Queue.Capacity,
			CoalesceMax: payload.Queue.CoalesceMax,
		},
	}, nil
}

func memoryDreamConfigFromPayload(payload contract.SettingsMemoryDreamPayload) (compozyconfig.DreamConfig, error) {
	debounce, err := parseSettingsDuration("memory.config.dream.debounce", payload.Debounce)
	if err != nil {
		return compozyconfig.DreamConfig{}, err
	}
	checkInterval, err := parseSettingsDuration("memory.config.dream.check_interval", payload.CheckInterval)
	if err != nil {
		return compozyconfig.DreamConfig{}, err
	}
	return compozyconfig.DreamConfig{
		MinHours:      payload.MinHours,
		MinSessions:   payload.MinSessions,
		Debounce:      debounce,
		PromptVersion: strings.TrimSpace(payload.PromptVersion),
		CheckInterval: checkInterval,
		Gates: compozyconfig.MemoryDreamGatesConfig{
			MinUnpromoted:  payload.Gates.MinUnpromoted,
			MinRecallCount: payload.Gates.MinRecallCount,
			MinScore:       payload.Gates.MinScore,
		},
		Scoring: memoryDreamScoringConfigFromPayload(payload.Scoring),
	}, nil
}

func memoryDreamScoringConfigFromPayload(
	payload contract.SettingsMemoryDreamScoringPayload,
) compozyconfig.MemoryDreamScoringConfig {
	return compozyconfig.MemoryDreamScoringConfig{
		RecencyHalfLifeDays: payload.RecencyHalfLifeDays,
		Weights: compozyconfig.MemoryDreamScoringWeightsConfig{
			Frequency: payload.Weights.Frequency,
			Relevance: payload.Weights.Relevance,
			Recency:   payload.Weights.Recency,
			Freshness: payload.Weights.Freshness,
		},
	}
}

func memorySessionConfigFromPayload(
	payload contract.SettingsMemorySessionPayload,
) (compozyconfig.MemorySessionConfig, error) {
	eventsPurgeGrace, err := parseSettingsDuration(
		"memory.config.session.events_purge_grace",
		payload.EventsPurgeGrace,
	)
	if err != nil {
		return compozyconfig.MemorySessionConfig{}, err
	}
	return compozyconfig.MemorySessionConfig{
		LedgerFormat:     strings.TrimSpace(payload.LedgerFormat),
		LedgerRoot:       strings.TrimSpace(payload.LedgerRoot),
		EventsPurgeGrace: eventsPurgeGrace,
		ColdArchiveDays:  payload.ColdArchiveDays,
		HardDeleteDays:   payload.HardDeleteDays,
		MaxArchiveBytes:  payload.MaxArchiveBytes,
		UnboundPartition: strings.TrimSpace(payload.UnboundPartition),
	}, nil
}

func memoryDailyConfigFromPayload(payload contract.SettingsMemoryDailyPayload) compozyconfig.MemoryDailyConfig {
	return compozyconfig.MemoryDailyConfig{
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
) (compozyconfig.MemoryProviderConfig, error) {
	timeout, err := parseSettingsDuration("memory.config.provider.timeout", payload.Timeout)
	if err != nil {
		return compozyconfig.MemoryProviderConfig{}, err
	}
	cooldown, err := parseSettingsDuration("memory.config.provider.cooldown", payload.Cooldown)
	if err != nil {
		return compozyconfig.MemoryProviderConfig{}, err
	}
	return compozyconfig.MemoryProviderConfig{
		Name:             strings.TrimSpace(payload.Name),
		Timeout:          timeout,
		FailureThreshold: payload.FailureThreshold,
		Cooldown:         cooldown,
	}, nil
}
