package schedule

import "time"

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
