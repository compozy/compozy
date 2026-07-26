package config

import (
	"path/filepath"

	"time"
)

// DefaultMemoryConfig returns the approved Memory v2 Slice 1 defaults.
func DefaultMemoryConfig(homePaths HomePaths) MemoryConfig {
	return MemoryConfig{
		Enabled:    true,
		GlobalDir:  homePaths.MemoryDir,
		Controller: defaultMemoryControllerConfig(),
		Recall:     defaultMemoryRecallConfig(),
		Decisions: MemoryDecisionsConfig{
			PruneAfterAppliedDays: 90,
			KeepAuditSummary:      true,
			MaxPostContentBytes:   65536,
		},
		Extractor: defaultMemoryExtractorConfig(homePaths),
		Dream:     defaultMemoryDreamConfig(),
		Session:   defaultMemorySessionConfig(homePaths),
		Daily:     defaultMemoryDailyConfig(),
		File:      MemoryFileConfig{MaxLines: 200, MaxBytes: 25600},
		Provider: MemoryProviderConfig{
			Timeout:          2 * time.Second,
			FailureThreshold: 5,
			Cooldown:         30 * time.Second,
		},
		Workspace: MemoryWorkspaceConfig{
			TOMLPath:   defaultMemoryWorkspaceTOMLPath,
			AutoCreate: true,
		},
	}
}

func defaultMemoryControllerConfig() MemoryControllerConfig {
	return MemoryControllerConfig{
		Mode:            configHybridKey,
		MaxLatency:      300 * time.Millisecond,
		DefaultOpOnFail: "noop",
		Policy: MemoryControllerPolicyConfig{
			MaxContentChars: 4096,
			MaxWritesPerMin: 60,
			AllowOrigins: []string{
				configCLIKey,
				marketplaceSchemeHTTP,
				configUDSKey,
				configToolKey,
				configExtractorKey,
				configDreamingKey,
				string(capabilityCatalogLayoutModeFile),
				configProviderKey,
			},
		},
	}
}

func defaultMemoryRecallConfig() MemoryRecallConfig {
	return MemoryRecallConfig{
		TopK:          5,
		RawCandidates: 50,
		Fusion:        "weighted",
		Weights: MemoryRecallWeightsConfig{
			BM25Unicode:  0.55,
			BM25Trigram:  0.20,
			Recency:      0.15,
			RecallSignal: 0.10,
		},
		Freshness: MemoryRecallFreshnessConfig{BannerAfterDays: 1},
		Signals: MemoryRecallSignalsConfig{
			QueueCapacity:  256,
			WorkerRetryMax: 3,
			MetricsEnabled: true,
		},
	}
}

func defaultMemoryExtractorConfig(homePaths HomePaths) MemoryExtractorConfig {
	return MemoryExtractorConfig{
		Mode:             configExtractorModePostMessage,
		ThrottleTurns:    1,
		Deadline:         60 * time.Second,
		SandboxInboxOnly: true,
		InboxPath:        filepath.Join(homePaths.MemoryDir, "_inbox"),
		DLQPath:          filepath.Join(homePaths.MemoryDir, "_system", configExtractorKey, "failures"),
		Queue:            MemoryExtractorQueueConfig{Capacity: 1, CoalesceMax: 16},
	}
}

func defaultMemoryDreamConfig() DreamConfig {
	return DreamConfig{
		MinHours:      24,
		MinSessions:   3,
		Debounce:      10 * time.Minute,
		PromptVersion: "v1",
		CheckInterval: 30 * time.Minute,
		Gates:         MemoryDreamGatesConfig{MinUnpromoted: 5, MinRecallCount: 2, MinScore: 0.75},
		Scoring: MemoryDreamScoringConfig{
			RecencyHalfLifeDays: 14,
			Weights: MemoryDreamScoringWeightsConfig{
				Frequency: 0.30,
				Relevance: 0.35,
				Recency:   0.20,
				Freshness: 0.15,
			},
		},
	}
}

func defaultMemorySessionConfig(homePaths HomePaths) MemorySessionConfig {
	return MemorySessionConfig{
		LedgerFormat:     configJsonlKey,
		LedgerRoot:       homePaths.SessionsDir,
		EventsPurgeGrace: 24 * time.Hour,
		ColdArchiveDays:  30,
		MaxArchiveBytes:  10737418240,
		UnboundPartition: "_unbound",
	}
}

func defaultMemoryDailyConfig() MemoryDailyConfig {
	return MemoryDailyConfig{
		MaxBytes:        1048576,
		MaxLines:        5000,
		RotateFormat:    "{date}.{seq}.md",
		DreamingWindow:  7,
		ColdArchiveDays: 30,
		MaxArchiveBytes: 1073741824,
		SweepHour:       3,
		ArchivePath:     "_system/archive",
	}
}
