package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// ValidateSchedule validates the schedule configuration
func ValidateSchedule(cfg *Schedule) error {
	if err := validateScheduleCronExpression(cfg.Cron); err != nil {
		return err
	}
	if err := validateScheduleTimezone(cfg.Timezone); err != nil {
		return err
	}
	if err := validateScheduleOverlapPolicy(cfg.OverlapPolicy); err != nil {
		return err
	}
	if err := validateScheduleJitter(cfg.Jitter); err != nil {
		return err
	}
	return validateScheduleWindow(cfg.StartAt, cfg.EndAt)
}

func validateScheduleCronExpression(cronExpr string) error {
	if durationStr, ok := strings.CutPrefix(cronExpr, "@every "); ok {
		if _, err := time.ParseDuration(durationStr); err != nil {
			return fmt.Errorf("invalid @every duration '%s': %w", durationStr, err)
		}
		return nil
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(cronExpr); err != nil {
		fields := len(strings.Fields(cronExpr))
		if fields != 6 {
			return fmt.Errorf(
				"invalid cron expression: expected 6 fields (second minute hour day month weekday), got %d fields",
				fields,
			)
		}
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	return nil
}

func validateScheduleTimezone(tz string) error {
	if tz == "" {
		return nil
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	return nil
}

func validateScheduleOverlapPolicy(policy OverlapPolicy) error {
	switch policy {
	case "", OverlapSkip, OverlapAllow, OverlapBufferOne, OverlapCancelOther:
		return nil
	default:
		return fmt.Errorf("unsupported overlap_policy: %s", policy)
	}
}

func validateScheduleJitter(jitter string) error {
	if jitter == "" {
		return nil
	}
	if _, err := time.ParseDuration(jitter); err != nil {
		return fmt.Errorf("invalid jitter duration: %w", err)
	}
	return nil
}

func validateScheduleWindow(start, end *time.Time) error {
	if start == nil || end == nil {
		return nil
	}
	if start.After(*end) {
		return fmt.Errorf("start_at must be before end_at")
	}
	return nil
}
