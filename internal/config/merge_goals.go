package config

import "time"

type goalsOverlay struct {
	MaxTurns           *int           `toml:"max_turns"`
	ContextNudgeRatio  *float64       `toml:"context_nudge_ratio"`
	OutboxBatchSize    *int           `toml:"outbox_batch_size"`
	OutboxPollInterval *time.Duration `toml:"outbox_poll_interval"`
}

func (o goalsOverlay) Apply(dst *GoalsConfig) {
	if o.MaxTurns != nil {
		dst.MaxTurns = *o.MaxTurns
	}
	if o.ContextNudgeRatio != nil {
		dst.ContextNudgeRatio = *o.ContextNudgeRatio
	}
	if o.OutboxBatchSize != nil {
		dst.OutboxBatchSize = *o.OutboxBatchSize
	}
	if o.OutboxPollInterval != nil {
		dst.OutboxPollInterval = *o.OutboxPollInterval
	}
}
