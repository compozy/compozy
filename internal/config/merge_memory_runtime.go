package config

func (o memoryExtractorOverlay) Apply(dst *MemoryExtractorConfig) {
	if o.Mode != nil {
		dst.Mode = *o.Mode
	}
	if o.ThrottleTurns != nil {
		dst.ThrottleTurns = *o.ThrottleTurns
	}
	if o.Deadline != nil {
		dst.Deadline = *o.Deadline
	}
	if o.SandboxInboxOnly != nil {
		dst.SandboxInboxOnly = *o.SandboxInboxOnly
	}
	if o.InboxPath != nil {
		dst.InboxPath = *o.InboxPath
	}
	if o.DLQPath != nil {
		dst.DLQPath = *o.DLQPath
	}
	o.Queue.Apply(&dst.Queue)
}

func (o memoryExtractorQueueOverlay) Apply(dst *MemoryExtractorQueueConfig) {
	if o.Capacity != nil {
		dst.Capacity = *o.Capacity
	}
	if o.CoalesceMax != nil {
		dst.CoalesceMax = *o.CoalesceMax
	}
}

func (o dreamOverlay) Apply(dst *DreamConfig) {
	if o.MinHours != nil {
		dst.MinHours = *o.MinHours
	}
	if o.MinSessions != nil {
		dst.MinSessions = *o.MinSessions
	}
	if o.Debounce != nil {
		dst.Debounce = *o.Debounce
	}
	if o.PromptVersion != nil {
		dst.PromptVersion = *o.PromptVersion
	}
	if o.CheckInterval != nil {
		dst.CheckInterval = *o.CheckInterval
	}
	o.Gates.Apply(&dst.Gates)
	o.Scoring.Apply(&dst.Scoring)
}

func (o memoryDreamGatesOverlay) Apply(dst *MemoryDreamGatesConfig) {
	if o.MinUnpromoted != nil {
		dst.MinUnpromoted = *o.MinUnpromoted
	}
	if o.MinRecallCount != nil {
		dst.MinRecallCount = *o.MinRecallCount
	}
	if o.MinScore != nil {
		dst.MinScore = *o.MinScore
	}
}

func (o memoryDreamScoringOverlay) Apply(dst *MemoryDreamScoringConfig) {
	if o.RecencyHalfLifeDays != nil {
		dst.RecencyHalfLifeDays = *o.RecencyHalfLifeDays
	}
	o.Weights.Apply(&dst.Weights)
}

func (o memoryDreamScoringWeightsOverlay) Apply(dst *MemoryDreamScoringWeightsConfig) {
	if o.Frequency != nil {
		dst.Frequency = *o.Frequency
	}
	if o.Relevance != nil {
		dst.Relevance = *o.Relevance
	}
	if o.Recency != nil {
		dst.Recency = *o.Recency
	}
	if o.Freshness != nil {
		dst.Freshness = *o.Freshness
	}
}

func (o memorySessionOverlay) Apply(dst *MemorySessionConfig) {
	if o.LedgerFormat != nil {
		dst.LedgerFormat = *o.LedgerFormat
	}
	if o.LedgerRoot != nil {
		dst.LedgerRoot = *o.LedgerRoot
	}
	if o.EventsPurgeGrace != nil {
		dst.EventsPurgeGrace = *o.EventsPurgeGrace
	}
	if o.ColdArchiveDays != nil {
		dst.ColdArchiveDays = *o.ColdArchiveDays
	}
	if o.HardDeleteDays != nil {
		dst.HardDeleteDays = *o.HardDeleteDays
	}
	if o.MaxArchiveBytes != nil {
		dst.MaxArchiveBytes = *o.MaxArchiveBytes
	}
	if o.UnboundPartition != nil {
		dst.UnboundPartition = *o.UnboundPartition
	}
}

func (o memoryDailyOverlay) Apply(dst *MemoryDailyConfig) {
	if o.MaxBytes != nil {
		dst.MaxBytes = *o.MaxBytes
	}
	if o.MaxLines != nil {
		dst.MaxLines = *o.MaxLines
	}
	if o.RotateFormat != nil {
		dst.RotateFormat = *o.RotateFormat
	}
	if o.DreamingWindow != nil {
		dst.DreamingWindow = *o.DreamingWindow
	}
	if o.ColdArchiveDays != nil {
		dst.ColdArchiveDays = *o.ColdArchiveDays
	}
	if o.HardDeleteDays != nil {
		dst.HardDeleteDays = *o.HardDeleteDays
	}
	if o.MaxArchiveBytes != nil {
		dst.MaxArchiveBytes = *o.MaxArchiveBytes
	}
	if o.SweepHour != nil {
		dst.SweepHour = *o.SweepHour
	}
	if o.ArchivePath != nil {
		dst.ArchivePath = *o.ArchivePath
	}
}

func (o memoryFileOverlay) Apply(dst *MemoryFileConfig) {
	if o.MaxLines != nil {
		dst.MaxLines = *o.MaxLines
	}
	if o.MaxBytes != nil {
		dst.MaxBytes = *o.MaxBytes
	}
}

func (o memoryProviderOverlay) Apply(dst *MemoryProviderConfig) {
	if o.Name != nil {
		dst.Name = *o.Name
	}
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
	if o.FailureThreshold != nil {
		dst.FailureThreshold = *o.FailureThreshold
	}
	if o.Cooldown != nil {
		dst.Cooldown = *o.Cooldown
	}
}

func (o memoryWorkspaceOverlay) Apply(dst *MemoryWorkspaceConfig) {
	if o.TOMLPath != nil {
		dst.TOMLPath = *o.TOMLPath
	}
	if o.AutoCreate != nil {
		dst.AutoCreate = *o.AutoCreate
	}
}
