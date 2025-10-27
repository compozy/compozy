package schedule

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	engineschedule "github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// Builder constructs workflow schedule registrations using a fluent API while accumulating validation issues.
type Builder struct {
	config *engineschedule.Config
	errors []error
}

var cloneScheduleConfig = func(cfg *engineschedule.Config) (*engineschedule.Config, error) {
	return core.DeepCopy(cfg)
}

// New creates a schedule builder initialized with the provided identifier.
func New(id string) *Builder {
	trimmed := strings.TrimSpace(id)
	return &Builder{
		config: &engineschedule.Config{
			ID: trimmed,
		},
		errors: make([]error, 0),
	}
}

// WithCron sets the cron expression that determines when the schedule executes.
func (b *Builder) WithCron(cron string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(cron)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("cron expression cannot be empty"))
		return b
	}
	b.config.Cron = trimmed
	return b
}

// WithWorkflow associates the schedule with a workflow identifier.
func (b *Builder) WithWorkflow(workflowID string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(workflowID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("workflow id cannot be empty"))
		return b
	}
	b.config.WorkflowID = trimmed
	return b
}

// WithInput provides default input values for scheduled workflow executions.
func (b *Builder) WithInput(input map[string]any) *Builder {
	if b == nil {
		return nil
	}
	if input == nil {
		b.config.Input = nil
		return b
	}
	cloned := core.CloneMap(input)
	b.config.Input = cloned
	return b
}

// WithRetry configures retry behaviour for failed scheduled executions.
func (b *Builder) WithRetry(maxAttempts int, backoff time.Duration) *Builder {
	if b == nil {
		return nil
	}
	if maxAttempts <= 0 {
		b.errors = append(b.errors, fmt.Errorf("retry attempts must be positive"))
	}
	if backoff <= 0 {
		b.errors = append(b.errors, fmt.Errorf("retry backoff must be positive"))
	}
	b.config.Retry = &engineschedule.RetryPolicy{
		MaxAttempts: maxAttempts,
		Backoff:     backoff,
	}
	return b
}

// WithTimezone sets the timezone used to evaluate the cron expression.
func (b *Builder) WithTimezone(timezone string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(timezone)
	if trimmed == "" {
		b.config.Timezone = ""
		return b
	}
	b.config.Timezone = trimmed
	return b
}

// WithDescription sets a human-readable description for the schedule.
func (b *Builder) WithDescription(description string) *Builder {
	if b == nil {
		return nil
	}
	b.config.Description = strings.TrimSpace(description)
	return b
}

// WithEnabled toggles whether the schedule should start active.
func (b *Builder) WithEnabled(enabled bool) *Builder {
	if b == nil {
		return nil
	}
	value := enabled
	b.config.Enabled = &value
	return b
}

// Build validates the accumulated configuration and returns a schedule registration.
func (b *Builder) Build(ctx context.Context) (*engineschedule.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("schedule builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building schedule configuration", "schedule", b.config.ID, "workflow", b.config.WorkflowID)
	collected := make([]error, 0, len(b.errors)+4)
	collected = append(collected, b.errors...)
	if err := validate.ValidateID(ctx, b.config.ID); err != nil {
		collected = append(collected, fmt.Errorf("schedule id is invalid: %w", err))
	}
	if err := validate.ValidateNonEmpty(ctx, "workflow id", b.config.WorkflowID); err != nil {
		collected = append(collected, err)
	} else if err := validate.ValidateID(ctx, b.config.WorkflowID); err != nil {
		collected = append(collected, fmt.Errorf("workflow id is invalid: %w", err))
	}
	if err := validate.ValidateCron(ctx, b.config.Cron); err != nil {
		collected = append(collected, err)
	}
	if b.config.Retry != nil {
		if err := validate.ValidateRange(ctx, "retry attempts", b.config.Retry.MaxAttempts, 1, 100); err != nil {
			collected = append(collected, err)
		}
		if err := validate.ValidateDuration(ctx, b.config.Retry.Backoff); err != nil {
			collected = append(collected, err)
		}
	}
	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := cloneScheduleConfig(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone schedule config: %w", err)
	}
	return cloned, nil
}
