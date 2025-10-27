package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const defaultWaitCondition = "true"

// WaitBuilder constructs wait task configurations that either pause for a fixed
// duration or continue when a condition becomes true. It aggregates validation
// errors collected during fluently chained configuration helpers.
type WaitBuilder struct {
	config       *enginetask.Config
	errors       []error
	duration     *time.Duration
	timeout      *time.Duration
	conditionSet bool
}

// NewWait creates a wait task builder initialized with the provided identifier.
// By default the wait task listens for a signal matching the trimmed identifier.
func NewWait(id string) *WaitBuilder {
	trimmed := strings.TrimSpace(id)
	return &WaitBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeWait,
			},
			WaitTask: enginetask.WaitTask{
				WaitFor: trimmed,
			},
		},
		errors: make([]error, 0),
	}
}

// WithSignal configures the wait task to listen for the provided signal name.
func (b *WaitBuilder) WithSignal(signal string) *WaitBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(signal)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("signal name cannot be empty"))
		return b
	}
	b.config.WaitFor = trimmed
	return b
}

// WithDuration configures the wait task to pause for the specified duration.
func (b *WaitBuilder) WithDuration(duration time.Duration) *WaitBuilder {
	if b == nil {
		return nil
	}
	if duration <= 0 {
		b.errors = append(b.errors, fmt.Errorf("duration must be positive: got %s", duration))
		b.duration = nil
		return b
	}
	d := duration
	b.duration = &d
	return b
}

// WithCondition configures the wait task to continue when the CEL expression evaluates to true.
func (b *WaitBuilder) WithCondition(condition string) *WaitBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(condition)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("condition cannot be empty"))
		b.config.Condition = ""
		b.conditionSet = false
		return b
	}
	b.config.Condition = trimmed
	b.conditionSet = true
	return b
}

// WithTimeout sets the maximum amount of time the wait task should block while evaluating the condition.
func (b *WaitBuilder) WithTimeout(timeout time.Duration) *WaitBuilder {
	if b == nil {
		return nil
	}
	if timeout <= 0 {
		b.errors = append(b.errors, fmt.Errorf("timeout must be positive: got %s", timeout))
		b.timeout = nil
		return b
	}
	t := timeout
	b.timeout = &t
	return b
}

// Build validates the accumulated configuration and returns a cloned engine wait task config.
func (b *WaitBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("wait builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug(
		"building wait task configuration",
		"task",
		b.config.ID,
		"waitFor",
		b.config.WaitFor,
		"hasDuration",
		b.duration != nil,
		"hasCondition",
		b.conditionSet,
	)

	collected := append(make([]error, 0, len(b.errors)+6), b.errors...)
	collected = append(
		collected,
		b.ensureMode(),
		b.validateID(ctx),
		b.validateWaitFor(ctx),
		b.applyDuration(),
		b.validateCondition(ctx),
		b.ensureConditionHasTimeout(),
	)
	b.applyTimeout()

	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	b.config.Type = enginetask.TaskTypeWait
	b.config.Resource = string(core.ConfigTask)

	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone wait task config: %w", err)
	}
	return cloned, nil
}

func (b *WaitBuilder) ensureMode() error {
	hasDuration := b.duration != nil
	hasCondition := b.conditionSet
	switch {
	case hasDuration && hasCondition:
		return fmt.Errorf("wait tasks cannot specify both duration and condition")
	case !hasDuration && !hasCondition:
		return fmt.Errorf("wait tasks require either a duration or a condition")
	default:
		return nil
	}
}

func (b *WaitBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	return nil
}

func (b *WaitBuilder) validateWaitFor(ctx context.Context) error {
	trimmed := strings.TrimSpace(b.config.WaitFor)
	if err := validate.NonEmpty(ctx, "wait_for", trimmed); err != nil {
		return err
	}
	b.config.WaitFor = trimmed
	return nil
}

func (b *WaitBuilder) applyDuration() error {
	if b.duration == nil {
		return nil
	}
	if b.timeout != nil {
		return fmt.Errorf("duration waits cannot specify an explicit timeout")
	}
	duration := *b.duration
	b.config.Condition = defaultWaitCondition
	b.config.Timeout = duration.String()
	return nil
}

func (b *WaitBuilder) validateCondition(ctx context.Context) error {
	if !b.conditionSet {
		return nil
	}
	b.config.Condition = strings.TrimSpace(b.config.Condition)
	if err := validate.NonEmpty(ctx, "condition", b.config.Condition); err != nil {
		return err
	}
	return nil
}

func (b *WaitBuilder) ensureConditionHasTimeout() error {
	if !b.conditionSet {
		return nil
	}
	if b.timeout == nil && strings.TrimSpace(b.config.Timeout) == "" {
		return fmt.Errorf("conditional waits require a timeout")
	}
	return nil
}

func (b *WaitBuilder) applyTimeout() {
	if b.timeout == nil {
		return
	}
	if b.duration != nil {
		return
	}
	timeout := *b.timeout
	b.config.Timeout = timeout.String()
}
