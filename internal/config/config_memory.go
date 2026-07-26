package config

import "time"

// ObservabilityTranscriptConfig configures transcript capture and retention.
type ObservabilityTranscriptConfig struct {
	Enabled            bool  `toml:"enabled"`
	SegmentBytes       int   `toml:"segment_bytes"`
	MaxBytesPerSession int64 `toml:"max_bytes_per_session"`
}

// MemoryConfig controls persistent memory features.
type MemoryConfig struct {
	Enabled    bool                   `toml:"enabled"`
	GlobalDir  string                 `toml:"global_dir,omitempty"`
	Controller MemoryControllerConfig `toml:"controller"`
	Recall     MemoryRecallConfig     `toml:"recall"`
	Decisions  MemoryDecisionsConfig  `toml:"decisions"`
	Extractor  MemoryExtractorConfig  `toml:"extractor"`
	Dream      DreamConfig            `toml:"dream"`
	Session    MemorySessionConfig    `toml:"session"`
	Daily      MemoryDailyConfig      `toml:"daily"`
	File       MemoryFileConfig       `toml:"file"`
	Provider   MemoryProviderConfig   `toml:"provider"`
	Workspace  MemoryWorkspaceConfig  `toml:"workspace"`
}

// MemoryControllerConfig controls the durable write controller.
type MemoryControllerConfig struct {
	Mode            string                       `toml:"mode"`
	MaxLatency      time.Duration                `toml:"max_latency"`
	DefaultOpOnFail string                       `toml:"default_op_on_fail"`
	Policy          MemoryControllerPolicyConfig `toml:"policy"`
}

// MemoryControllerPolicyConfig controls controller safety limits.
type MemoryControllerPolicyConfig struct {
	MaxContentChars int      `toml:"max_content_chars"`
	MaxWritesPerMin int      `toml:"max_writes_per_min"`
	AllowOrigins    []string `toml:"allow_origins"`
}

// MemoryRecallConfig controls deterministic recall.
type MemoryRecallConfig struct {
	TopK                   int                         `toml:"top_k"`
	RawCandidates          int                         `toml:"raw_candidates"`
	Fusion                 string                      `toml:"fusion"`
	IncludeAlreadySurfaced bool                        `toml:"include_already_surfaced"`
	IncludeSystem          bool                        `toml:"include_system"`
	Weights                MemoryRecallWeightsConfig   `toml:"weights"`
	Freshness              MemoryRecallFreshnessConfig `toml:"freshness"`
	Signals                MemoryRecallSignalsConfig   `toml:"signals"`
}

// MemoryRecallWeightsConfig controls deterministic recall scoring weights.
type MemoryRecallWeightsConfig struct {
	BM25Unicode  float64 `toml:"bm25_unicode"`
	BM25Trigram  float64 `toml:"bm25_trigram"`
	Recency      float64 `toml:"recency"`
	RecallSignal float64 `toml:"recall_signal"`
}

// MemoryRecallFreshnessConfig controls recall freshness banners.
type MemoryRecallFreshnessConfig struct {
	BannerAfterDays int `toml:"banner_after_days"`
}

// MemoryRecallSignalsConfig controls recall signal recording.
type MemoryRecallSignalsConfig struct {
	QueueCapacity  int  `toml:"queue_capacity"`
	WorkerRetryMax int  `toml:"worker_retry_max"`
	MetricsEnabled bool `toml:"metrics_enabled"`
}

// MemoryDecisionsConfig controls Decision WAL retention and content caps.
type MemoryDecisionsConfig struct {
	PruneAfterAppliedDays int   `toml:"prune_after_applied_days"`
	KeepAuditSummary      bool  `toml:"keep_audit_summary"`
	MaxPostContentBytes   int64 `toml:"max_post_content_bytes"`
}

// MemoryExtractorConfig controls the post-message extractor queue.
type MemoryExtractorConfig struct {
	Mode             string                     `toml:"mode"`
	ThrottleTurns    int                        `toml:"throttle_turns"`
	Deadline         time.Duration              `toml:"deadline"`
	SandboxInboxOnly bool                       `toml:"sandbox_inbox_only"`
	InboxPath        string                     `toml:"inbox_path"`
	DLQPath          string                     `toml:"dlq_path"`
	Queue            MemoryExtractorQueueConfig `toml:"queue"`
}

// MemoryExtractorQueueConfig controls bounded extractor work.
type MemoryExtractorQueueConfig struct {
	Capacity    int `toml:"capacity"`
	CoalesceMax int `toml:"coalesce_max"`
}

// DreamConfig controls background dream consolidation.
type DreamConfig struct {
	MinHours      float64                  `toml:"min_hours"`
	MinSessions   int                      `toml:"min_sessions"`
	Debounce      time.Duration            `toml:"debounce"`
	PromptVersion string                   `toml:"prompt_version"`
	CheckInterval time.Duration            `toml:"check_interval"`
	Gates         MemoryDreamGatesConfig   `toml:"gates"`
	Scoring       MemoryDreamScoringConfig `toml:"scoring"`
}

// MemoryDreamGatesConfig controls promotion gates for dreaming candidates.
type MemoryDreamGatesConfig struct {
	MinUnpromoted  int     `toml:"min_unpromoted"`
	MinRecallCount int     `toml:"min_recall_count"`
	MinScore       float64 `toml:"min_score"`
}

// MemoryDreamScoringConfig controls dreaming candidate scoring.
type MemoryDreamScoringConfig struct {
	RecencyHalfLifeDays int                             `toml:"recency_half_life_days"`
	Weights             MemoryDreamScoringWeightsConfig `toml:"weights"`
}

// MemoryDreamScoringWeightsConfig controls dreaming score factors.
type MemoryDreamScoringWeightsConfig struct {
	Frequency float64 `toml:"frequency"`
	Relevance float64 `toml:"relevance"`
	Recency   float64 `toml:"recency"`
	Freshness float64 `toml:"freshness"`
}

// MemorySessionConfig controls forensic session ledger retention.
type MemorySessionConfig struct {
	LedgerFormat     string        `toml:"ledger_format"`
	LedgerRoot       string        `toml:"ledger_root"`
	EventsPurgeGrace time.Duration `toml:"events_purge_grace"`
	ColdArchiveDays  int           `toml:"cold_archive_days"`
	HardDeleteDays   int           `toml:"hard_delete_days"`
	MaxArchiveBytes  int64         `toml:"max_archive_bytes"`
	UnboundPartition string        `toml:"unbound_partition"`
}

// MemoryDailyConfig controls daily note retention and rotation.
type MemoryDailyConfig struct {
	MaxBytes        int64  `toml:"max_bytes"`
	MaxLines        int    `toml:"max_lines"`
	RotateFormat    string `toml:"rotate_format"`
	DreamingWindow  int    `toml:"dreaming_window"`
	ColdArchiveDays int    `toml:"cold_archive_days"`
	HardDeleteDays  int    `toml:"hard_delete_days"`
	MaxArchiveBytes int64  `toml:"max_archive_bytes"`
	SweepHour       int    `toml:"sweep_hour"`
	ArchivePath     string `toml:"archive_path"`
}

// MemoryFileConfig controls individual memory file limits.
type MemoryFileConfig struct {
	MaxLines int   `toml:"max_lines"`
	MaxBytes int64 `toml:"max_bytes"`
}

// MemoryProviderConfig controls the active memory provider registry entry.
type MemoryProviderConfig struct {
	Name             string        `toml:"name"`
	Timeout          time.Duration `toml:"timeout"`
	FailureThreshold int           `toml:"failure_threshold"`
	Cooldown         time.Duration `toml:"cooldown"`
}

// MemoryWorkspaceConfig controls workspace memory file lifecycle.
type MemoryWorkspaceConfig struct {
	TOMLPath   string `toml:"toml_path"`
	AutoCreate bool   `toml:"auto_create"`
}

// MarketplaceConfig controls the external skill registry used by CLI skill commands.
type MarketplaceConfig struct {
	Registry string `toml:"registry"`
	BaseURL  string `toml:"base_url,omitempty"`
}
