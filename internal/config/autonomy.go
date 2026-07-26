package config

import (
	"fmt"
	"time"

	configdefaults "github.com/compozy/agh/internal/config/defaults"
)

const (
	// DefaultSchedulerFanOutAfter is the wake count before the convergence ladder fans out.
	DefaultSchedulerFanOutAfter = 2
	// DefaultSchedulerSpawnAfter is the wake count before a capability-matched worker is spawned.
	DefaultSchedulerSpawnAfter = 4
	// DefaultSchedulerEventAfter is the wake count before the canonical starved event is emitted.
	DefaultSchedulerEventAfter = 6
	// DefaultSchedulerNeedsAttentionAfter is the wake count before a run is parked needs_attention.
	DefaultSchedulerNeedsAttentionAfter = 10
	// DefaultSchedulerMinQueuedAge is the queued age before a claimable run starts escalating.
	DefaultSchedulerMinQueuedAge = 2 * time.Minute
	// DefaultBlockRecurrenceLimit is the default same-kind re-block count before escalation.
	DefaultBlockRecurrenceLimit = configdefaults.BlockRecurrenceLimit
)

// AutonomyConfig controls opt-in autonomy features.
type AutonomyConfig struct {
	BlockRecurrenceLimit int             `toml:"block_recurrence_limit"`
	Scheduler            SchedulerConfig `toml:"scheduler"`
}

// SchedulerConfig bounds the mechanical scheduler's convergence escalation ladder. The counts are
// monotonic wake cycles a claimable run must remain queued before each tier fires.
type SchedulerConfig struct {
	FanOutAfter         int           `toml:"fan_out_after"`
	SpawnAfter          int           `toml:"spawn_after"`
	EventAfter          int           `toml:"event_after"`
	NeedsAttentionAfter int           `toml:"needs_attention_after"`
	MinQueuedAge        time.Duration `toml:"min_queued_age"`
}

// DefaultSchedulerConfig returns the built-in convergence ladder defaults.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		FanOutAfter:         DefaultSchedulerFanOutAfter,
		SpawnAfter:          DefaultSchedulerSpawnAfter,
		EventAfter:          DefaultSchedulerEventAfter,
		NeedsAttentionAfter: DefaultSchedulerNeedsAttentionAfter,
		MinQueuedAge:        DefaultSchedulerMinQueuedAge,
	}
}

// Validate ensures autonomy config is internally consistent.
func (c AutonomyConfig) Validate() error {
	if c.BlockRecurrenceLimit < 0 {
		return fmt.Errorf("autonomy.block_recurrence_limit must be >= 0: %d", c.BlockRecurrenceLimit)
	}
	return c.Scheduler.Validate("autonomy.scheduler")
}

// Validate ensures the convergence ladder thresholds are positive and monotonic.
func (c SchedulerConfig) Validate(path string) error {
	if c.FanOutAfter <= 0 {
		return fmt.Errorf("%s.fan_out_after must be positive: %d", path, c.FanOutAfter)
	}
	if c.SpawnAfter < c.FanOutAfter {
		return fmt.Errorf("%s.spawn_after must be >= fan_out_after: %d < %d", path, c.SpawnAfter, c.FanOutAfter)
	}
	if c.EventAfter < c.SpawnAfter {
		return fmt.Errorf("%s.event_after must be >= spawn_after: %d < %d", path, c.EventAfter, c.SpawnAfter)
	}
	if c.NeedsAttentionAfter < c.EventAfter {
		return fmt.Errorf(
			"%s.needs_attention_after must be >= event_after: %d < %d",
			path,
			c.NeedsAttentionAfter,
			c.EventAfter,
		)
	}
	if c.MinQueuedAge <= 0 {
		return fmt.Errorf("%s.min_queued_age must be positive: %s", path, c.MinQueuedAge)
	}
	return nil
}
