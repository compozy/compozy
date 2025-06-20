package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// ValidateSchedule validates the schedule configuration
func ValidateSchedule(cfg *Schedule) error {
	// Check if it's an @every expression
	if strings.HasPrefix(cfg.Cron, "@every ") {
		durationStr := strings.TrimPrefix(cfg.Cron, "@every ")
		if _, err := time.ParseDuration(durationStr); err != nil {
			return fmt.Errorf("invalid @every duration '%s': %w", durationStr, err)
		}
	} else {
		// Validate cron expression with seconds support
		// Compozy uses 6-field cron expressions:
		// Format: "second minute hour day-of-month month day-of-week"
		// Example: "0 0 9 * * 1-5" (Every weekday at 9:00:00 AM)
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(cfg.Cron); err != nil {
			// Provide more specific error message for field count issues
			fields := len(strings.Fields(cfg.Cron))
			if fields != 6 {
				return fmt.Errorf(
					"invalid cron expression: expected 6 fields (second minute hour day month weekday), got %d fields",
					fields,
				)
			}
			return fmt.Errorf("invalid cron expression: %w", err)
		}
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
