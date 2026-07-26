package config

import (
	"fmt"
	"math"
	"time"
)

const (
	defaultGoalMaxTurns           = 20
	defaultGoalContextNudgeRatio  = 0.8
	defaultGoalOutboxBatchSize    = 50
	defaultGoalOutboxPollInterval = 100 * time.Millisecond
	goalMaxTurnsPath              = "goals.max_turns"
	goalContextNudgeRatioPath     = "goals.context_nudge_ratio"
	goalOutboxBatchSizePath       = "goals.outbox_batch_size"
	goalOutboxPollIntervalPath    = "goals.outbox_poll_interval"
)

// GoalsConfig controls defaults applied to newly started Goal runs.
type GoalsConfig struct {
	MaxTurns           int           `toml:"max_turns"`
	ContextNudgeRatio  float64       `toml:"context_nudge_ratio"`
	OutboxBatchSize    int           `toml:"outbox_batch_size"`
	OutboxPollInterval time.Duration `toml:"outbox_poll_interval"`
}

// DefaultGoalsConfig returns the built-in Goal defaults.
func DefaultGoalsConfig() GoalsConfig {
	return GoalsConfig{
		MaxTurns:           defaultGoalMaxTurns,
		ContextNudgeRatio:  defaultGoalContextNudgeRatio,
		OutboxBatchSize:    defaultGoalOutboxBatchSize,
		OutboxPollInterval: defaultGoalOutboxPollInterval,
	}
}

// Validate enforces Goal default bounds without changing explicit values.
func (c GoalsConfig) Validate() error {
	if c.MaxTurns < 1 {
		return ValidationError{
			Path:    goalMaxTurnsPath,
			Message: fmt.Sprintf("must be >= 1: %d", c.MaxTurns),
		}
	}
	if math.IsNaN(c.ContextNudgeRatio) || math.IsInf(c.ContextNudgeRatio, 0) ||
		c.ContextNudgeRatio < 0 || c.ContextNudgeRatio > 1 {
		return ValidationError{
			Path:    goalContextNudgeRatioPath,
			Message: fmt.Sprintf("must be between 0 and 1: %v", c.ContextNudgeRatio),
		}
	}
	if c.OutboxBatchSize < 1 || c.OutboxBatchSize > 200 {
		return ValidationError{
			Path:    goalOutboxBatchSizePath,
			Message: fmt.Sprintf("must be between 1 and 200: %d", c.OutboxBatchSize),
		}
	}
	if c.OutboxPollInterval <= 0 {
		return ValidationError{
			Path:    goalOutboxPollIntervalPath,
			Message: fmt.Sprintf("must be positive: %s", c.OutboxPollInterval),
		}
	}
	return nil
}
