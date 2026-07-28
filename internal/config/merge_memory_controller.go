package config

import "strings"

func (o *memoryOverlay) Apply(dst *MemoryConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.GlobalDir != nil && strings.TrimSpace(*o.GlobalDir) != "" {
		dst.GlobalDir = *o.GlobalDir
	}
	o.Controller.Apply(&dst.Controller)
	o.Recall.Apply(&dst.Recall)
	o.Decisions.Apply(&dst.Decisions)
	o.Extractor.Apply(&dst.Extractor)
	o.Dream.Apply(&dst.Dream)
	o.Session.Apply(&dst.Session)
	o.Daily.Apply(&dst.Daily)
	o.File.Apply(&dst.File)
	o.Provider.Apply(&dst.Provider)
	o.Workspace.Apply(&dst.Workspace)
}

func (o memoryControllerOverlay) Apply(dst *MemoryControllerConfig) {
	if o.Mode != nil {
		dst.Mode = *o.Mode
	}
	if o.MaxLatency != nil {
		dst.MaxLatency = *o.MaxLatency
	}
	if o.DefaultOpOnFail != nil {
		dst.DefaultOpOnFail = *o.DefaultOpOnFail
	}
	o.Policy.Apply(&dst.Policy)
}

func (o memoryControllerPolicyOverlay) Apply(dst *MemoryControllerPolicyConfig) {
	if o.MaxContentChars != nil {
		dst.MaxContentChars = *o.MaxContentChars
	}
	if o.MaxWritesPerMin != nil {
		dst.MaxWritesPerMin = *o.MaxWritesPerMin
	}
	if o.AllowOrigins != nil {
		dst.AllowOrigins = append([]string(nil), (*o.AllowOrigins)...)
	}
}

func (o memoryRecallOverlay) Apply(dst *MemoryRecallConfig) {
	if o.TopK != nil {
		dst.TopK = *o.TopK
	}
	if o.RawCandidates != nil {
		dst.RawCandidates = *o.RawCandidates
	}
	if o.Fusion != nil {
		dst.Fusion = *o.Fusion
	}
	if o.IncludeAlreadySurfaced != nil {
		dst.IncludeAlreadySurfaced = *o.IncludeAlreadySurfaced
	}
	if o.IncludeSystem != nil {
		dst.IncludeSystem = *o.IncludeSystem
	}
	o.Weights.Apply(&dst.Weights)
	o.Freshness.Apply(&dst.Freshness)
	o.Signals.Apply(&dst.Signals)
}

func (o memoryRecallWeightsOverlay) Apply(dst *MemoryRecallWeightsConfig) {
	if o.BM25Unicode != nil {
		dst.BM25Unicode = *o.BM25Unicode
	}
	if o.BM25Trigram != nil {
		dst.BM25Trigram = *o.BM25Trigram
	}
	if o.Recency != nil {
		dst.Recency = *o.Recency
	}
	if o.RecallSignal != nil {
		dst.RecallSignal = *o.RecallSignal
	}
}

func (o memoryRecallFreshnessOverlay) Apply(dst *MemoryRecallFreshnessConfig) {
	if o.BannerAfterDays != nil {
		dst.BannerAfterDays = *o.BannerAfterDays
	}
}

func (o memoryRecallSignalsOverlay) Apply(dst *MemoryRecallSignalsConfig) {
	if o.QueueCapacity != nil {
		dst.QueueCapacity = *o.QueueCapacity
	}
	if o.WorkerRetryMax != nil {
		dst.WorkerRetryMax = *o.WorkerRetryMax
	}
	if o.MetricsEnabled != nil {
		dst.MetricsEnabled = *o.MetricsEnabled
	}
}

func (o memoryDecisionsOverlay) Apply(dst *MemoryDecisionsConfig) {
	if o.PruneAfterAppliedDays != nil {
		dst.PruneAfterAppliedDays = *o.PruneAfterAppliedDays
	}
	if o.KeepAuditSummary != nil {
		dst.KeepAuditSummary = *o.KeepAuditSummary
	}
	if o.MaxPostContentBytes != nil {
		dst.MaxPostContentBytes = *o.MaxPostContentBytes
	}
}
