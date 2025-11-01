package schedule

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Config describes a workflow schedule registration that runs a workflow on a cron cadence.
//
// Schedules are project-scoped resources that reference a workflow by identifier and define
// the cron expression, timezone, and optional retry policy used when launching executions.
type Config struct {
	// ID uniquely identifies the schedule within the project.
	ID string `json:"id"                    yaml:"id"                    mapstructure:"id"`
	// WorkflowID references the workflow that should be executed when the schedule fires.
	WorkflowID string `json:"workflow_id"           yaml:"workflow_id"           mapstructure:"workflow_id"`
	// Cron is the cron expression that determines when the schedule triggers.
	Cron string `json:"cron"                  yaml:"cron"                  mapstructure:"cron"`
	// Timezone provides the IANA timezone name used when evaluating the cron expression.
	Timezone string `json:"timezone,omitempty"    yaml:"timezone,omitempty"    mapstructure:"timezone,omitempty"`
	// Input contains default input values that are supplied to the workflow when triggered.
	Input map[string]any `json:"input,omitempty"       yaml:"input,omitempty"       mapstructure:"input,omitempty"`
	// Retry configures retry behavior for failed scheduled executions.
	Retry *RetryPolicy `json:"retry,omitempty"       yaml:"retry,omitempty"       mapstructure:"retry,omitempty"`
	// Enabled toggles whether the schedule is active.
	Enabled *bool `json:"enabled,omitempty"     yaml:"enabled,omitempty"     mapstructure:"enabled,omitempty"`
	// Description explains the schedule purpose for operators.
	Description string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`
}

// RetryPolicy defines retry behavior for scheduled workflow executions.
type RetryPolicy struct {
	// MaxAttempts is the number of retry attempts after the initial run fails.
	MaxAttempts int `json:"max_attempts" yaml:"max_attempts" mapstructure:"max_attempts"`
	// Backoff is the delay between retry attempts.
	Backoff time.Duration `json:"backoff"      yaml:"backoff"      mapstructure:"backoff"`
}

func (c *Config) Validate(ctx context.Context) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	c.ID = strings.TrimSpace(c.ID)
	if err := validateIdentifier(c.ID); err != nil {
		return fmt.Errorf("schedule id: %w", err)
	}
	c.WorkflowID = strings.TrimSpace(c.WorkflowID)
	if err := validateIdentifier(c.WorkflowID); err != nil {
		return fmt.Errorf("workflow_id: %w", err)
	}
	c.Cron = strings.TrimSpace(c.Cron)
	if err := validateCronExpression(c.Cron); err != nil {
		return fmt.Errorf("cron: %w", err)
	}
	if tz := strings.TrimSpace(c.Timezone); tz != "" {
		c.Timezone = tz
		if err := validateTimezone(tz); err != nil {
			return fmt.Errorf("timezone: %w", err)
		}
	}
	if c.Retry != nil {
		if err := c.Retry.Validate(ctx); err != nil {
			return fmt.Errorf("retry: %w", err)
		}
	}
	return nil
}

func (r *RetryPolicy) Validate(ctx context.Context) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if r.MaxAttempts <= 0 {
		return fmt.Errorf("max_attempts must be positive: got %d", r.MaxAttempts)
	}
	if err := validateDuration(r.Backoff); err != nil {
		return fmt.Errorf("backoff: %w", err)
	}
	return nil
}

var scheduleIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

func ensureContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	return nil
}

func validateIdentifier(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if !scheduleIDPattern.MatchString(id) {
		return fmt.Errorf("id must contain only letters, numbers, or hyphens")
	}
	return nil
}

func validateCronExpression(expr string) error {
	if expr == "" {
		return fmt.Errorf("cron expression is required")
	}
	if _, err := cron.ParseStandard(expr); err != nil {
		return fmt.Errorf("cron expression is invalid: %w", err)
	}
	return nil
}

func validateTimezone(name string) error {
	if _, err := time.LoadLocation(name); err != nil {
		return fmt.Errorf("timezone is invalid: %w", err)
	}
	return nil
}

func validateDuration(d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("duration must be positive: got %s", d)
	}
	return nil
}
