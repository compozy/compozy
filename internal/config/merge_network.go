package config

type networkOverlay struct {
	Enabled      *bool              `toml:"enabled"`
	MaxReplayAge *int               `toml:"max_replay_age"`
	Live         networkLiveOverlay `toml:"live"`
}

type networkLiveOverlay struct {
	Defaults networkLiveDefaultsOverlay `toml:"defaults"`
	Limits   networkLiveLimitsOverlay   `toml:"limits"`
}

type networkLiveDefaultsOverlay struct {
	MaxWakes         *int    `toml:"max_wakes"`
	MaxWakeWallTime  *string `toml:"max_wake_wall_time"`
	MaxTotalWallTime *string `toml:"max_total_wall_time"`
	MaxInputTokens   *int64  `toml:"max_input_tokens"`
	MaxOutputTokens  *int64  `toml:"max_output_tokens"`
	MaxWakeDepth     *int    `toml:"max_wake_depth"`
	CoalesceWindow   *string `toml:"coalesce_window"`
}

type networkLiveLimitsOverlay struct {
	MaxWakes          *int    `toml:"max_wakes"`
	MaxWakeWallTime   *string `toml:"max_wake_wall_time"`
	MaxTotalWallTime  *string `toml:"max_total_wall_time"`
	MaxInputTokens    *int64  `toml:"max_input_tokens"`
	MaxOutputTokens   *int64  `toml:"max_output_tokens"`
	MaxWakeDepth      *int    `toml:"max_wake_depth"`
	MinCoalesceWindow *string `toml:"min_coalesce_window"`
	MaxCoalesceWindow *string `toml:"max_coalesce_window"`
}

func (o networkOverlay) Apply(dst *NetworkConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.MaxReplayAge != nil {
		dst.MaxReplayAge = *o.MaxReplayAge
	}
	o.Live.Apply(&dst.Live)
}

func (o networkLiveOverlay) Apply(dst *NetworkLiveConfig) {
	o.Defaults.Apply(&dst.Defaults)
	o.Limits.Apply(&dst.Limits)
}

func (o networkLiveDefaultsOverlay) Apply(dst *NetworkLiveDefaultsConfig) {
	applyOptional(o.MaxWakes, &dst.MaxWakes)
	applyOptional(o.MaxWakeWallTime, &dst.MaxWakeWallTime)
	applyOptional(o.MaxTotalWallTime, &dst.MaxTotalWallTime)
	applyOptional(o.MaxInputTokens, &dst.MaxInputTokens)
	applyOptional(o.MaxOutputTokens, &dst.MaxOutputTokens)
	applyOptional(o.MaxWakeDepth, &dst.MaxWakeDepth)
	applyOptional(o.CoalesceWindow, &dst.CoalesceWindow)
}

func (o networkLiveLimitsOverlay) Apply(dst *NetworkLiveLimitsConfig) {
	applyOptional(o.MaxWakes, &dst.MaxWakes)
	applyOptional(o.MaxWakeWallTime, &dst.MaxWakeWallTime)
	applyOptional(o.MaxTotalWallTime, &dst.MaxTotalWallTime)
	applyOptional(o.MaxInputTokens, &dst.MaxInputTokens)
	applyOptional(o.MaxOutputTokens, &dst.MaxOutputTokens)
	applyOptional(o.MaxWakeDepth, &dst.MaxWakeDepth)
	applyOptional(o.MinCoalesceWindow, &dst.MinCoalesceWindow)
	applyOptional(o.MaxCoalesceWindow, &dst.MaxCoalesceWindow)
}

func applyOptional[T any](source *T, target *T) {
	if source != nil {
		*target = *source
	}
}
