package settings

import aghconfig "github.com/compozy/agh/internal/config"

func applyMemorySettings(editor *aghconfig.OverlayEditor, settings *aghconfig.MemoryConfig) error {
	return applyValueUpdates(editor, memorySettingsUpdates(settings))
}

func memorySettingsUpdates(settings *aghconfig.MemoryConfig) []struct {
	path  []string
	value any
} {
	if settings == nil {
		return nil
	}
	updates := []struct {
		path  []string
		value any
	}{
		{path: []string{string(SectionMemory), sectionsEnabledKey}, value: settings.Enabled},
		{path: []string{string(SectionMemory), "global_dir"}, value: settings.GlobalDir},
	}
	updates = append(updates, memoryControllerSettingsUpdates(settings)...)
	updates = append(updates, memoryRecallSettingsUpdates(settings)...)
	updates = append(updates, memoryExtractorSettingsUpdates(settings)...)
	updates = append(updates, memoryDreamSettingsUpdates(settings)...)
	updates = append(updates, memoryRetentionSettingsUpdates(settings)...)
	return append(updates, memoryProviderSettingsUpdates(settings)...)
}

func memoryControllerSettingsUpdates(settings *aghconfig.MemoryConfig) []struct {
	path  []string
	value any
} {
	return []struct {
		path  []string
		value any
	}{
		{
			path:  []string{string(SectionMemory), sectionsControllerKey, sectionsModeKey},
			value: settings.Controller.Mode,
		},
		{
			path:  []string{string(SectionMemory), sectionsControllerKey, "max_latency"},
			value: settings.Controller.MaxLatency.String(),
		},
		{
			path:  []string{string(SectionMemory), sectionsControllerKey, "default_op_on_fail"},
			value: settings.Controller.DefaultOpOnFail,
		},
		{
			path:  []string{string(SectionMemory), sectionsControllerKey, sectionsPolicyKey, "max_content_chars"},
			value: settings.Controller.Policy.MaxContentChars,
		},
		{
			path:  []string{string(SectionMemory), sectionsControllerKey, sectionsPolicyKey, "max_writes_per_min"},
			value: settings.Controller.Policy.MaxWritesPerMin,
		},
		{
			path:  []string{string(SectionMemory), sectionsControllerKey, sectionsPolicyKey, "allow_origins"},
			value: append([]string(nil), settings.Controller.Policy.AllowOrigins...),
		},
	}
}

func memoryRecallSettingsUpdates(settings *aghconfig.MemoryConfig) []struct {
	path  []string
	value any
} {
	return []struct {
		path  []string
		value any
	}{
		{path: []string{string(SectionMemory), sectionsRecallKey, "top_k"}, value: settings.Recall.TopK},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, "raw_candidates"},
			value: settings.Recall.RawCandidates,
		},
		{path: []string{string(SectionMemory), sectionsRecallKey, "fusion"}, value: settings.Recall.Fusion},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, "include_already_surfaced"},
			value: settings.Recall.IncludeAlreadySurfaced,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, "include_system"},
			value: settings.Recall.IncludeSystem,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, sectionsWeightsKey, "bm25_unicode"},
			value: settings.Recall.Weights.BM25Unicode,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, sectionsWeightsKey, "bm25_trigram"},
			value: settings.Recall.Weights.BM25Trigram,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, sectionsWeightsKey, "recency"},
			value: settings.Recall.Weights.Recency,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, sectionsWeightsKey, "recall_signal"},
			value: settings.Recall.Weights.RecallSignal,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, "freshness", "banner_after_days"},
			value: settings.Recall.Freshness.BannerAfterDays,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, sectionsSignalsKey, "queue_capacity"},
			value: settings.Recall.Signals.QueueCapacity,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, sectionsSignalsKey, "worker_retry_max"},
			value: settings.Recall.Signals.WorkerRetryMax,
		},
		{
			path:  []string{string(SectionMemory), sectionsRecallKey, sectionsSignalsKey, "metrics_enabled"},
			value: settings.Recall.Signals.MetricsEnabled,
		},
		{
			path:  []string{string(SectionMemory), sectionsDecisionsKey, "prune_after_applied_days"},
			value: settings.Decisions.PruneAfterAppliedDays,
		},
		{
			path:  []string{string(SectionMemory), sectionsDecisionsKey, "keep_audit_summary"},
			value: settings.Decisions.KeepAuditSummary,
		},
		{
			path:  []string{string(SectionMemory), sectionsDecisionsKey, "max_post_content_bytes"},
			value: settings.Decisions.MaxPostContentBytes,
		},
	}
}

func memoryExtractorSettingsUpdates(settings *aghconfig.MemoryConfig) []struct {
	path  []string
	value any
} {
	return []struct {
		path  []string
		value any
	}{
		{path: []string{string(SectionMemory), sectionsExtractorKey, sectionsModeKey}, value: settings.Extractor.Mode},
		{
			path:  []string{string(SectionMemory), sectionsExtractorKey, "throttle_turns"},
			value: settings.Extractor.ThrottleTurns,
		},
		{
			path:  []string{string(SectionMemory), sectionsExtractorKey, "deadline"},
			value: settings.Extractor.Deadline.String(),
		},
		{
			path:  []string{string(SectionMemory), sectionsExtractorKey, "sandbox_inbox_only"},
			value: settings.Extractor.SandboxInboxOnly,
		},
		{
			path:  []string{string(SectionMemory), sectionsExtractorKey, "inbox_path"},
			value: settings.Extractor.InboxPath,
		},
		{path: []string{string(SectionMemory), sectionsExtractorKey, "dlq_path"}, value: settings.Extractor.DLQPath},
		{
			path:  []string{string(SectionMemory), sectionsExtractorKey, sectionsQueueKey, "capacity"},
			value: settings.Extractor.Queue.Capacity,
		},
		{
			path:  []string{string(SectionMemory), sectionsExtractorKey, sectionsQueueKey, "coalesce_max"},
			value: settings.Extractor.Queue.CoalesceMax,
		},
	}
}

func memoryDreamSettingsUpdates(settings *aghconfig.MemoryConfig) []struct {
	path  []string
	value any
} {
	return []struct {
		path  []string
		value any
	}{
		{path: []string{string(SectionMemory), sectionsDreamKey, "min_hours"}, value: settings.Dream.MinHours},
		{path: []string{string(SectionMemory), sectionsDreamKey, "min_sessions"}, value: settings.Dream.MinSessions},
		{path: []string{string(SectionMemory), sectionsDreamKey, "debounce"}, value: settings.Dream.Debounce.String()},
		{
			path:  []string{string(SectionMemory), sectionsDreamKey, "prompt_version"},
			value: settings.Dream.PromptVersion,
		},
		{
			path:  []string{string(SectionMemory), sectionsDreamKey, "check_interval"},
			value: settings.Dream.CheckInterval.String(),
		},
		{
			path:  []string{string(SectionMemory), sectionsDreamKey, sectionsGatesKey, "min_unpromoted"},
			value: settings.Dream.Gates.MinUnpromoted,
		},
		{
			path:  []string{string(SectionMemory), sectionsDreamKey, sectionsGatesKey, "min_recall_count"},
			value: settings.Dream.Gates.MinRecallCount,
		},
		{
			path:  []string{string(SectionMemory), sectionsDreamKey, sectionsGatesKey, "min_score"},
			value: settings.Dream.Gates.MinScore,
		},
		{
			path:  []string{string(SectionMemory), sectionsDreamKey, sectionsScoringKey, "recency_half_life_days"},
			value: settings.Dream.Scoring.RecencyHalfLifeDays,
		},
		{
			path: []string{
				string(SectionMemory),
				sectionsDreamKey,
				sectionsScoringKey,
				sectionsWeightsKey,
				"frequency",
			},
			value: settings.Dream.Scoring.Weights.Frequency,
		},
		{
			path: []string{
				string(SectionMemory),
				sectionsDreamKey,
				sectionsScoringKey,
				sectionsWeightsKey,
				"relevance",
			},
			value: settings.Dream.Scoring.Weights.Relevance,
		},
		{
			path:  []string{string(SectionMemory), sectionsDreamKey, sectionsScoringKey, sectionsWeightsKey, "recency"},
			value: settings.Dream.Scoring.Weights.Recency,
		},
		{
			path: []string{
				string(SectionMemory),
				sectionsDreamKey,
				sectionsScoringKey,
				sectionsWeightsKey,
				"freshness",
			},
			value: settings.Dream.Scoring.Weights.Freshness,
		},
	}
}

func memoryRetentionSettingsUpdates(settings *aghconfig.MemoryConfig) []struct {
	path  []string
	value any
} {
	return []struct {
		path  []string
		value any
	}{
		{
			path:  []string{string(SectionMemory), sectionsSessionKey, "ledger_format"},
			value: settings.Session.LedgerFormat,
		},
		{path: []string{string(SectionMemory), sectionsSessionKey, "ledger_root"}, value: settings.Session.LedgerRoot},
		{
			path:  []string{string(SectionMemory), sectionsSessionKey, "events_purge_grace"},
			value: settings.Session.EventsPurgeGrace.String(),
		},
		{
			path:  []string{string(SectionMemory), sectionsSessionKey, "cold_archive_days"},
			value: settings.Session.ColdArchiveDays,
		},
		{
			path:  []string{string(SectionMemory), sectionsSessionKey, "hard_delete_days"},
			value: settings.Session.HardDeleteDays,
		},
		{
			path:  []string{string(SectionMemory), sectionsSessionKey, "max_archive_bytes"},
			value: settings.Session.MaxArchiveBytes,
		},
		{
			path:  []string{string(SectionMemory), sectionsSessionKey, "unbound_partition"},
			value: settings.Session.UnboundPartition,
		},
		{path: []string{string(SectionMemory), sectionsDailyKey, "max_bytes"}, value: settings.Daily.MaxBytes},
		{path: []string{string(SectionMemory), sectionsDailyKey, "max_lines"}, value: settings.Daily.MaxLines},
		{path: []string{string(SectionMemory), sectionsDailyKey, "rotate_format"}, value: settings.Daily.RotateFormat},
		{
			path:  []string{string(SectionMemory), sectionsDailyKey, "dreaming_window"},
			value: settings.Daily.DreamingWindow,
		},
		{
			path:  []string{string(SectionMemory), sectionsDailyKey, "cold_archive_days"},
			value: settings.Daily.ColdArchiveDays,
		},
		{
			path:  []string{string(SectionMemory), sectionsDailyKey, "hard_delete_days"},
			value: settings.Daily.HardDeleteDays,
		},
		{
			path:  []string{string(SectionMemory), sectionsDailyKey, "max_archive_bytes"},
			value: settings.Daily.MaxArchiveBytes,
		},
		{path: []string{string(SectionMemory), sectionsDailyKey, "sweep_hour"}, value: settings.Daily.SweepHour},
		{path: []string{string(SectionMemory), sectionsDailyKey, "archive_path"}, value: settings.Daily.ArchivePath},
		{path: []string{string(SectionMemory), "file", "max_lines"}, value: settings.File.MaxLines},
		{path: []string{string(SectionMemory), "file", "max_bytes"}, value: settings.File.MaxBytes},
	}
}

func memoryProviderSettingsUpdates(settings *aghconfig.MemoryConfig) []struct {
	path  []string
	value any
} {
	return []struct {
		path  []string
		value any
	}{
		{path: []string{string(SectionMemory), sectionsProviderKey, "name"}, value: settings.Provider.Name},
		{
			path:  []string{string(SectionMemory), sectionsProviderKey, sectionsTimeoutKey},
			value: settings.Provider.Timeout.String(),
		},
		{
			path:  []string{string(SectionMemory), sectionsProviderKey, "failure_threshold"},
			value: settings.Provider.FailureThreshold,
		},
		{
			path:  []string{string(SectionMemory), sectionsProviderKey, "cooldown"},
			value: settings.Provider.Cooldown.String(),
		},
		{
			path:  []string{string(SectionMemory), string(ScopeWorkspace), "auto_create"},
			value: settings.Workspace.AutoCreate,
		},
	}
}
