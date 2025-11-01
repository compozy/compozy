package schedule

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	engineschedule "github.com/compozy/compozy/engine/project/schedule"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

// New creates a schedule configuration using functional options
func New(ctx context.Context, id string, opts ...Option) (*engineschedule.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating schedule configuration", "schedule", id)
	cfg := &engineschedule.Config{
		ID: strings.TrimSpace(id),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if err := validateScheduleConfig(ctx, cfg); err != nil {
		return nil, err
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone schedule config: %w", err)
	}
	return cloned, nil
}

func validateScheduleConfig(ctx context.Context, cfg *engineschedule.Config) error {
	collected := make([]error, 0, 8)
	cfg.ID = strings.TrimSpace(cfg.ID)
	if err := validate.ID(ctx, cfg.ID); err != nil {
		collected = append(collected, fmt.Errorf("schedule id is invalid: %w", err))
	}
	if err := validateWorkflowID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateCronExpression(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateTimezone(cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateRetry(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	cfg.Description = strings.TrimSpace(cfg.Description)
	return buildValidationError(collected)
}

func validateWorkflowID(ctx context.Context, cfg *engineschedule.Config) error {
	cfg.WorkflowID = strings.TrimSpace(cfg.WorkflowID)
	if err := validate.NonEmpty(ctx, "workflow id", cfg.WorkflowID); err != nil {
		return err
	}
	if err := validate.ID(ctx, cfg.WorkflowID); err != nil {
		return fmt.Errorf("workflow id is invalid: %w", err)
	}
	return nil
}

func validateCronExpression(ctx context.Context, cfg *engineschedule.Config) error {
	cfg.Cron = strings.TrimSpace(cfg.Cron)
	return validate.Cron(ctx, cfg.Cron)
}

func validateTimezone(cfg *engineschedule.Config) error {
	if cfg.Timezone == "" {
		return nil
	}
	cfg.Timezone = strings.TrimSpace(cfg.Timezone)
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return fmt.Errorf("timezone is invalid: %w", err)
	}
	return nil
}

func validateRetry(ctx context.Context, cfg *engineschedule.Config) error {
	if cfg.Retry == nil {
		return nil
	}
	if cfg.Retry.MaxAttempts <= 0 {
		return fmt.Errorf("retry max attempts must be positive: got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.MaxAttempts > 100 {
		return fmt.Errorf("retry max attempts must not exceed 100: got %d", cfg.Retry.MaxAttempts)
	}
	if err := validate.Duration(ctx, cfg.Retry.Backoff); err != nil {
		return fmt.Errorf("retry backoff %w", err)
	}
	return nil
}

func buildValidationError(collected []error) error {
	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return &sdkerrors.BuildError{Errors: filtered}
	}
	return nil
}
