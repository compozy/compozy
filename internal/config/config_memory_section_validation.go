package config

import (
	"errors"
	"fmt"

	"strings"
)

// Validate ensures the dream configuration is internally consistent.
func (c DreamConfig) Validate() error {
	if c.MinHours <= 0 {
		return fmt.Errorf("memory.dream.min_hours must be positive: %v", c.MinHours)
	}
	if c.MinSessions <= 0 {
		return fmt.Errorf("memory.dream.min_sessions must be positive: %d", c.MinSessions)
	}
	if c.CheckInterval <= 0 {
		return fmt.Errorf("memory.dream.check_interval must be positive: %s", c.CheckInterval)
	}
	if c.Debounce <= 0 {
		return fmt.Errorf("memory.dream.debounce must be positive: %s", c.Debounce)
	}
	if strings.TrimSpace(c.PromptVersion) == "" {
		return errors.New("memory.dream.prompt_version is required")
	}
	if err := c.Gates.Validate(); err != nil {
		return err
	}
	return c.Scoring.Validate()
}

// Validate ensures the controller configuration is internally consistent.
func (c *MemoryControllerConfig) Validate() error {
	mode, err := validateEnum("memory.controller.mode", c.Mode, configHybridKey, "rules", configLLMKey)
	if err != nil {
		return err
	}
	c.Mode = mode
	if c.MaxLatency <= 0 {
		return fmt.Errorf("memory.controller.max_latency must be positive: %s", c.MaxLatency)
	}
	defaultOpOnFail, err := validateEnum("memory.controller.default_op_on_fail", c.DefaultOpOnFail, "noop", "reject")
	if err != nil {
		return err
	}
	c.DefaultOpOnFail = defaultOpOnFail
	return c.Policy.Validate()
}

// Validate ensures the controller policy configuration is internally consistent.
func (c *MemoryControllerPolicyConfig) Validate() error {
	if c.MaxContentChars <= 0 {
		return fmt.Errorf("memory.controller.policy.max_content_chars must be positive: %d", c.MaxContentChars)
	}
	if c.MaxWritesPerMin <= 0 {
		return fmt.Errorf("memory.controller.policy.max_writes_per_min must be positive: %d", c.MaxWritesPerMin)
	}
	allowedOrigins := map[string]struct{}{
		configCLIKey:                            {},
		marketplaceSchemeHTTP:                   {},
		configUDSKey:                            {},
		configToolKey:                           {},
		configExtractorKey:                      {},
		configDreamingKey:                       {},
		string(capabilityCatalogLayoutModeFile): {},
		configProviderKey:                       {},
	}
	if len(c.AllowOrigins) == 0 {
		return errors.New("memory.controller.policy.allow_origins must not be empty")
	}
	seen := make(map[string]struct{}, len(c.AllowOrigins))
	canonical := make([]string, len(c.AllowOrigins))
	for i, origin := range c.AllowOrigins {
		normalized := strings.ToLower(strings.TrimSpace(origin))
		if _, ok := allowedOrigins[normalized]; !ok {
			return fmt.Errorf("memory.controller.policy.allow_origins[%d] is invalid: %q", i, origin)
		}
		if _, ok := seen[normalized]; ok {
			return fmt.Errorf("memory.controller.policy.allow_origins[%d] duplicates %q", i, origin)
		}
		seen[normalized] = struct{}{}
		canonical[i] = normalized
	}
	c.AllowOrigins = canonical
	return nil
}

// Validate ensures the recall configuration is internally consistent.
func (c *MemoryRecallConfig) Validate() error {
	if c.TopK <= 0 {
		return fmt.Errorf("memory.recall.top_k must be positive: %d", c.TopK)
	}
	if c.RawCandidates < c.TopK {
		return fmt.Errorf(
			"memory.recall.raw_candidates must be >= memory.recall.top_k: %d < %d",
			c.RawCandidates,
			c.TopK,
		)
	}
	fusion, err := validateEnum("memory.recall.fusion", c.Fusion, "weighted", "rrf")
	if err != nil {
		return err
	}
	c.Fusion = fusion
	if err := c.Weights.Validate(); err != nil {
		return err
	}
	if c.Freshness.BannerAfterDays < 0 {
		return fmt.Errorf(
			"memory.recall.freshness.banner_after_days must be zero or positive: %d",
			c.Freshness.BannerAfterDays,
		)
	}
	return c.Signals.Validate()
}

// Validate ensures recall weights are usable.
func (c MemoryRecallWeightsConfig) Validate() error {
	weights := map[string]float64{
		configMemoryRecallWeightsBm25UnicodePath: c.BM25Unicode,
		"memory.recall.weights.bm25_trigram":     c.BM25Trigram,
		"memory.recall.weights.recency":          c.Recency,
		"memory.recall.weights.recall_signal":    c.RecallSignal,
	}
	var sum float64
	for path, weight := range weights {
		if err := validateWeight(path, weight); err != nil {
			return err
		}
		sum += weight
	}
	return validateWeightSum("memory.recall.weights", sum)
}

// Validate ensures recall signal settings are usable.
func (c MemoryRecallSignalsConfig) Validate() error {
	if c.QueueCapacity <= 0 {
		return fmt.Errorf("memory.recall.signals.queue_capacity must be positive: %d", c.QueueCapacity)
	}
	if c.WorkerRetryMax < 0 {
		return fmt.Errorf("memory.recall.signals.worker_retry_max must be zero or positive: %d", c.WorkerRetryMax)
	}
	return nil
}

// Validate ensures Decision WAL retention settings are usable.
func (c MemoryDecisionsConfig) Validate() error {
	if c.PruneAfterAppliedDays < 0 {
		return fmt.Errorf(
			"memory.decisions.prune_after_applied_days must be zero or positive: %d",
			c.PruneAfterAppliedDays,
		)
	}
	if c.MaxPostContentBytes <= 0 {
		return fmt.Errorf("memory.decisions.max_post_content_bytes must be positive: %d", c.MaxPostContentBytes)
	}
	return nil
}

// Validate ensures extractor settings are internally consistent.
func (c *MemoryExtractorConfig) Validate() error {
	mode, err := validateEnum("memory.extractor.mode", c.Mode, configExtractorModePostMessage)
	if err != nil {
		return err
	}
	c.Mode = mode
	if c.ThrottleTurns <= 0 {
		return fmt.Errorf("memory.extractor.throttle_turns must be positive: %d", c.ThrottleTurns)
	}
	if c.Deadline <= 0 {
		return fmt.Errorf("memory.extractor.deadline must be positive: %s", c.Deadline)
	}
	if strings.TrimSpace(c.InboxPath) == "" {
		return errors.New("memory.extractor.inbox_path is required")
	}
	if strings.TrimSpace(c.DLQPath) == "" {
		return errors.New("memory.extractor.dlq_path is required")
	}
	if err := c.Queue.Validate(); err != nil {
		return err
	}
	if c.Queue.CoalesceMax < c.ThrottleTurns {
		return fmt.Errorf(
			"%s must be greater than or equal to memory.extractor.throttle_turns: %d < %d",
			configExtractorQueueCoalesceMaxPath,
			c.Queue.CoalesceMax,
			c.ThrottleTurns,
		)
	}
	return nil
}

// Validate ensures extractor queue settings are usable.
func (c MemoryExtractorQueueConfig) Validate() error {
	if c.Capacity <= 0 {
		return fmt.Errorf("memory.extractor.queue.capacity must be positive: %d", c.Capacity)
	}
	if c.CoalesceMax <= 0 {
		return fmt.Errorf("%s must be positive: %d", configExtractorQueueCoalesceMaxPath, c.CoalesceMax)
	}
	return nil
}

// Validate ensures dreaming promotion gates are usable.
func (c MemoryDreamGatesConfig) Validate() error {
	if c.MinUnpromoted <= 0 {
		return fmt.Errorf("memory.dream.gates.min_unpromoted must be positive: %d", c.MinUnpromoted)
	}
	if c.MinRecallCount <= 0 {
		return fmt.Errorf("memory.dream.gates.min_recall_count must be positive: %d", c.MinRecallCount)
	}
	if c.MinScore < 0 || c.MinScore > 1 {
		return fmt.Errorf("memory.dream.gates.min_score must be between 0 and 1: %v", c.MinScore)
	}
	return nil
}

// Validate ensures dreaming scoring settings are usable.
func (c MemoryDreamScoringConfig) Validate() error {
	if c.RecencyHalfLifeDays <= 0 {
		return fmt.Errorf("memory.dream.scoring.recency_half_life_days must be positive: %d", c.RecencyHalfLifeDays)
	}
	return c.Weights.Validate()
}

// Validate ensures dreaming scoring weights are usable.
func (c MemoryDreamScoringWeightsConfig) Validate() error {
	weights := map[string]float64{
		"memory.dream.scoring.weights.frequency": c.Frequency,
		"memory.dream.scoring.weights.relevance": c.Relevance,
		"memory.dream.scoring.weights.recency":   c.Recency,
		"memory.dream.scoring.weights.freshness": c.Freshness,
	}
	var sum float64
	for path, weight := range weights {
		if err := validateWeight(path, weight); err != nil {
			return err
		}
		sum += weight
	}
	return validateWeightSum("memory.dream.scoring.weights", sum)
}

// Validate ensures session ledger settings are usable.
func (c *MemorySessionConfig) Validate() error {
	ledgerFormat, err := validateEnum("memory.session.ledger_format", c.LedgerFormat, configJsonlKey)
	if err != nil {
		return err
	}
	c.LedgerFormat = ledgerFormat
	if strings.TrimSpace(c.LedgerRoot) == "" {
		return errors.New("memory.session.ledger_root is required")
	}
	if c.EventsPurgeGrace <= 0 {
		return fmt.Errorf("memory.session.events_purge_grace must be positive: %s", c.EventsPurgeGrace)
	}
	if c.ColdArchiveDays < 0 {
		return fmt.Errorf("memory.session.cold_archive_days must be zero or positive: %d", c.ColdArchiveDays)
	}
	if c.HardDeleteDays < 0 {
		return fmt.Errorf("memory.session.hard_delete_days must be zero or positive: %d", c.HardDeleteDays)
	}
	if c.MaxArchiveBytes <= 0 {
		return fmt.Errorf("memory.session.max_archive_bytes must be positive: %d", c.MaxArchiveBytes)
	}
	return validateSafePathSegment("memory.session.unbound_partition", c.UnboundPartition)
}

// Validate ensures daily note settings are usable.
func (c MemoryDailyConfig) Validate() error {
	if c.MaxBytes <= 0 {
		return fmt.Errorf("memory.daily.max_bytes must be positive: %d", c.MaxBytes)
	}
	if c.MaxLines <= 0 {
		return fmt.Errorf("memory.daily.max_lines must be positive: %d", c.MaxLines)
	}
	if strings.TrimSpace(c.RotateFormat) == "" {
		return errors.New("memory.daily.rotate_format is required")
	}
	if c.DreamingWindow <= 0 {
		return fmt.Errorf("memory.daily.dreaming_window must be positive: %d", c.DreamingWindow)
	}
	if c.ColdArchiveDays < 0 {
		return fmt.Errorf("memory.daily.cold_archive_days must be zero or positive: %d", c.ColdArchiveDays)
	}
	if c.HardDeleteDays < 0 {
		return fmt.Errorf("memory.daily.hard_delete_days must be zero or positive: %d", c.HardDeleteDays)
	}
	if c.MaxArchiveBytes <= 0 {
		return fmt.Errorf("memory.daily.max_archive_bytes must be positive: %d", c.MaxArchiveBytes)
	}
	if c.SweepHour < 0 || c.SweepHour > 23 {
		return fmt.Errorf("memory.daily.sweep_hour must be between 0 and 23: %d", c.SweepHour)
	}
	if strings.TrimSpace(c.ArchivePath) == "" {
		return errors.New("memory.daily.archive_path is required")
	}
	return nil
}

// Validate ensures memory file limits are usable.
func (c MemoryFileConfig) Validate() error {
	if c.MaxLines <= 0 {
		return fmt.Errorf("memory.file.max_lines must be positive: %d", c.MaxLines)
	}
	if c.MaxBytes <= 0 {
		return fmt.Errorf("memory.file.max_bytes must be positive: %d", c.MaxBytes)
	}
	return nil
}

// Validate ensures provider settings are usable.
func (c MemoryProviderConfig) Validate() error {
	if c.Timeout <= 0 {
		return fmt.Errorf("memory.provider.timeout must be positive: %s", c.Timeout)
	}
	if c.FailureThreshold <= 0 {
		return fmt.Errorf("memory.provider.failure_threshold must be positive: %d", c.FailureThreshold)
	}
	if c.Cooldown <= 0 {
		return fmt.Errorf("memory.provider.cooldown must be positive: %s", c.Cooldown)
	}
	return nil
}

// Validate ensures workspace memory settings are usable.
func (c MemoryWorkspaceConfig) Validate() error {
	if strings.TrimSpace(c.TOMLPath) != defaultMemoryWorkspaceTOMLPath {
		return fmt.Errorf(
			"memory.workspace.toml_path is informational and must remain %q",
			defaultMemoryWorkspaceTOMLPath,
		)
	}
	return nil
}
