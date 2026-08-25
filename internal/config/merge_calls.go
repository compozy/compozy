package config

type callsOverlay struct {
	MaxDepth         *int                 `toml:"max_depth"`
	MaxBatch         *int                 `toml:"max_batch"`
	MaxChildren      *int                 `toml:"max_children"`
	MaxActivePerRoot *int                 `toml:"max_active_per_root"`
	IdleTTL          *string              `toml:"idle_ttl"`
	Results          callsResultsOverlay  `toml:"results"`
	Messages         callsMessagesOverlay `toml:"messages"`
}

type callsResultsOverlay struct {
	DefaultBudget *string `toml:"default_budget"`
	MaxBudget     *string `toml:"max_budget"`
	Overflow      *string `toml:"overflow"`
}

type callsMessagesOverlay struct {
	RateLimitPerMinute *int    `toml:"rate_limit_per_minute"`
	DedupWindow        *string `toml:"dedup_window"`
	PendingCap         *int    `toml:"pending_cap"`
	MaxBytes           *string `toml:"max_bytes"`
}

func (o callsOverlay) Apply(dst *CallsConfig) {
	applyOptional(o.MaxDepth, &dst.MaxDepth)
	applyOptional(o.MaxBatch, &dst.MaxBatch)
	applyOptional(o.MaxChildren, &dst.MaxChildren)
	applyOptional(o.MaxActivePerRoot, &dst.MaxActivePerRoot)
	applyOptional(o.IdleTTL, &dst.IdleTTL)
	o.Results.Apply(&dst.Results)
	o.Messages.Apply(&dst.Messages)
}

func (o callsResultsOverlay) Apply(dst *CallsResultsConfig) {
	applyOptional(o.DefaultBudget, &dst.DefaultBudget)
	applyOptional(o.MaxBudget, &dst.MaxBudget)
	applyOptional(o.Overflow, &dst.Overflow)
}

func (o callsMessagesOverlay) Apply(dst *CallsMessagesConfig) {
	applyOptional(o.RateLimitPerMinute, &dst.RateLimitPerMinute)
	applyOptional(o.DedupWindow, &dst.DedupWindow)
	applyOptional(o.PendingCap, &dst.PendingCap)
	applyOptional(o.MaxBytes, &dst.MaxBytes)
}
