package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	cron "github.com/robfig/cron/v3"
)

var standardCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// Validate ensures the schedule spec matches the selected mode and has a valid expression payload.
func (s ScheduleSpec) Validate(path string) error {
	if err := s.Mode.Validate(nestedPath(path, "mode")); err != nil {
		return err
	}
	if err := s.validateReliability(path); err != nil {
		return err
	}

	switch s.Mode {
	case ScheduleModeCron:
		return s.validateCron(path)
	case ScheduleModeEvery:
		return s.validateEvery(path)
	case ScheduleModeAt:
		return s.validateAt(path)
	default:
		return nil
	}
}

func (s ScheduleSpec) validateReliability(path string) error {
	if s.CatchUpPolicy != "" {
		if err := s.CatchUpPolicy.Validate(nestedPath(path, "catch_up_policy")); err != nil {
			return err
		}
	}
	if s.MisfireGraceSeconds < 0 {
		return fmt.Errorf(
			"%s must be zero or positive: %d",
			nestedPath(path, "misfire_grace_seconds"),
			s.MisfireGraceSeconds,
		)
	}
	if s.Mode != ScheduleModeAt {
		return nil
	}
	if s.CatchUpPolicy != "" {
		return errors.New(nestedPath(path, "catch_up_policy") + " must be empty when schedule.mode is \"at\"")
	}
	if s.MisfireGraceSeconds != 0 {
		return errors.New(nestedPath(path, "misfire_grace_seconds") + " must be zero when schedule.mode is \"at\"")
	}
	return nil
}

func (s ScheduleSpec) validateCron(path string) error {
	if strings.TrimSpace(s.Expr) == "" {
		return errors.New(nestedPath(path, "expr") + " is required when schedule.mode is \"cron\"")
	}
	if strings.TrimSpace(s.Interval) != "" {
		return errors.New(nestedPath(path, "interval") + " must be empty when schedule.mode is \"cron\"")
	}
	if strings.TrimSpace(s.Time) != "" {
		return errors.New(nestedPath(path, "time") + " must be empty when schedule.mode is \"cron\"")
	}
	if _, err := standardCronParser.Parse(strings.TrimSpace(s.Expr)); err != nil {
		return fmt.Errorf("%s is invalid: %w", nestedPath(path, "expr"), err)
	}
	return nil
}

func (s ScheduleSpec) validateEvery(path string) error {
	if strings.TrimSpace(s.Interval) == "" {
		return errors.New(nestedPath(path, "interval") + " is required when schedule.mode is \"every\"")
	}
	if strings.TrimSpace(s.Expr) != "" {
		return errors.New(nestedPath(path, "expr") + " must be empty when schedule.mode is \"every\"")
	}
	if strings.TrimSpace(s.Time) != "" {
		return errors.New(nestedPath(path, "time") + " must be empty when schedule.mode is \"every\"")
	}
	interval, err := time.ParseDuration(strings.TrimSpace(s.Interval))
	if err != nil {
		return fmt.Errorf("%s is invalid: %w", nestedPath(path, "interval"), err)
	}
	if interval <= 0 {
		return fmt.Errorf("%s must be positive: %s", nestedPath(path, "interval"), interval)
	}
	return nil
}

func (s ScheduleSpec) validateAt(path string) error {
	if strings.TrimSpace(s.Time) == "" {
		return errors.New(nestedPath(path, "time") + " is required when schedule.mode is \"at\"")
	}
	if strings.TrimSpace(s.Expr) != "" {
		return errors.New(nestedPath(path, "expr") + " must be empty when schedule.mode is \"at\"")
	}
	if strings.TrimSpace(s.Interval) != "" {
		return errors.New(nestedPath(path, "interval") + " must be empty when schedule.mode is \"at\"")
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(s.Time)); err != nil {
		return fmt.Errorf("%s is invalid: %w", nestedPath(path, "time"), err)
	}
	return nil
}
