package workflow

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// ValidateSchedule validates the schedule configuration
func ValidateSchedule(cfg *Schedule) error {
	// Validate cron expression
	if _, err := cron.ParseStandard(cfg.Cron); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	// Validate timezone if specified
	if cfg.Timezone != "" {
		if _, err := time.LoadLocation(cfg.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %w", err)
		}
	}
	// Validate overlap policy
	switch cfg.OverlapPolicy {
	case "", OverlapSkip, OverlapAllow, OverlapBufferOne, OverlapCancelOther:
		// valid
	default:
		return fmt.Errorf("unsupported overlap_policy: %s", cfg.OverlapPolicy)
	}
	// Validate jitter duration format if provided
	if cfg.Jitter != "" {
		if _, err := time.ParseDuration(cfg.Jitter); err != nil {
			return fmt.Errorf("invalid jitter duration: %w", err)
		}
	}
	// Validate start and end times
	if cfg.StartAt != nil && cfg.EndAt != nil {
		if cfg.StartAt.After(*cfg.EndAt) {
			return fmt.Errorf("start_at must be before end_at")
		}
	}
	return nil
}
