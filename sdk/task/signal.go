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

// SignalMode describes whether a signal task should dispatch or wait for a signal.
type SignalMode string

const (
	// SignalModeSend dispatches a signal with an optional payload.
	SignalModeSend SignalMode = "send"
	// SignalModeWait pauses execution until the signal is received.
	SignalModeWait SignalMode = "wait"
)

// SignalBuilder constructs unified signal task configurations supporting both send and wait operations.
type SignalBuilder struct {
	config       *enginetask.Config
	errors       []error
	mode         SignalMode
	modeSet      bool
	signalID     string
	payload      map[string]any
	dataProvided bool
	timeout      *time.Duration
	timeoutSet   bool
}

// NewSignal creates a signal builder initialized with the provided identifier and defaults the signal ID to that value.
func NewSignal(id string) *SignalBuilder {
	trimmed := strings.TrimSpace(id)
	return &SignalBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeSignal,
			},
			SignalTask: enginetask.SignalTask{
				Signal: &enginetask.SignalConfig{ID: trimmed},
			},
		},
		errors:   make([]error, 0),
		mode:     SignalModeSend,
		modeSet:  false,
		signalID: trimmed,
	}
}

// WithSignalID overrides the signal identifier used for both send and wait operations.
func (b *SignalBuilder) WithSignalID(signalID string) *SignalBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(signalID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("signal id cannot be empty"))
		return b
	}
	b.signalID = trimmed
	return b
}

// WithMode selects whether the builder dispatches or waits for a signal.
func (b *SignalBuilder) WithMode(mode SignalMode) *SignalBuilder {
	if b == nil {
		return nil
	}
	normalized := SignalMode(strings.ToLower(strings.TrimSpace(string(mode))))
	switch normalized {
	case SignalModeSend, SignalModeWait:
		b.mode = normalized
		b.modeSet = true
	default:
		b.errors = append(b.errors, fmt.Errorf("invalid signal mode: %s", mode))
		b.modeSet = false
		b.mode = ""
	}
	return b
}

// WithData assigns the payload dispatched with the signal in send mode.
func (b *SignalBuilder) WithData(data map[string]any) *SignalBuilder {
	if b == nil {
		return nil
	}
	if data == nil {
		b.errors = append(b.errors, fmt.Errorf("signal data cannot be nil"))
		b.dataProvided = false
		b.payload = nil
		return b
	}
	normalized := make(map[string]any, len(data))
	for key, value := range data {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			b.errors = append(b.errors, fmt.Errorf("signal data key cannot be empty"))
			continue
		}
		normalized[trimmed] = value
	}
	b.payload = core.CloneMap(normalized)
	b.dataProvided = true
	return b
}

// WithTimeout configures the maximum time the wait mode should block before timing out.
func (b *SignalBuilder) WithTimeout(timeout time.Duration) *SignalBuilder {
	if b == nil {
		return nil
	}
	if timeout <= 0 {
		b.errors = append(b.errors, fmt.Errorf("timeout must be positive: got %s", timeout))
		b.timeout = nil
		b.timeoutSet = false
		return b
	}
	t := timeout
	b.timeout = &t
	b.timeoutSet = true
	return b
}

// OnSuccess configures the next task executed after a successful signal operation.
func (b *SignalBuilder) OnSuccess(taskID string) *SignalBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("success transition task id cannot be empty"))
		return b
	}
	next := trimmed
	b.config.OnSuccess = &core.SuccessTransition{Next: &next}
	return b
}

// OnError configures the next task executed when the signal operation fails.
func (b *SignalBuilder) OnError(taskID string) *SignalBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("error transition task id cannot be empty"))
		return b
	}
	next := trimmed
	b.config.OnError = &core.ErrorTransition{Next: &next}
	return b
}

// Send is a convenience helper that configures the builder for sending a signal with payload data.
func (b *SignalBuilder) Send(signalID string, payload map[string]any) *SignalBuilder {
	if b == nil {
		return nil
	}
	return b.WithMode(SignalModeSend).WithSignalID(signalID).WithData(payload)
}

// Wait is a convenience helper that configures the builder for waiting on a signal.
func (b *SignalBuilder) Wait(signalID string) *SignalBuilder {
	if b == nil {
		return nil
	}
	return b.WithMode(SignalModeWait).WithSignalID(signalID)
}

// Build validates the accumulated configuration and returns a cloned engine task configuration.
func (b *SignalBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("signal builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	log := logger.FromContext(ctx)
	log.Debug(
		"building signal task configuration",
		"task",
		b.config.ID,
		"mode",
		string(b.mode),
		"hasPayload",
		b.payload != nil && len(b.payload) > 0,
		"timeoutSet",
		b.timeoutSet,
	)

	collected := make([]error, 0, len(b.errors)+6)
	collected = append(collected, b.errors...)
	collected = append(collected, b.applyTaskID(ctx))
	collected = append(collected, b.ensureSignalID(ctx))
	collected = append(collected, b.ensureModeSelected())

	switch b.mode {
	case SignalModeSend:
		collected = append(collected, b.applySendConfig(ctx))
	case SignalModeWait:
		collected = append(collected, b.applyWaitConfig(ctx))
	default:
		if b.modeSet {
			collected = append(collected, fmt.Errorf("unsupported signal mode: %s", b.mode))
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

	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone signal task config: %w", err)
	}
	return cloned, nil
}

func (b *SignalBuilder) applyTaskID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ValidateID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	b.config.Resource = string(core.ConfigTask)
	return nil
}

func (b *SignalBuilder) ensureSignalID(ctx context.Context) error {
	b.signalID = strings.TrimSpace(b.signalID)
	if err := validate.ValidateNonEmpty(ctx, "signal_id", b.signalID); err != nil {
		return err
	}
	return nil
}

func (b *SignalBuilder) ensureModeSelected() error {
	if !b.modeSet && b.mode == "" {
		return fmt.Errorf("signal mode is required")
	}
	if b.mode != SignalModeSend && b.mode != SignalModeWait {
		return fmt.Errorf("invalid signal mode: %s", b.mode)
	}
	return nil
}

func (b *SignalBuilder) applySendConfig(ctx context.Context) error {
	if !b.dataProvided || b.payload == nil {
		return fmt.Errorf("send mode requires data payload")
	}
	if err := validate.ValidateNonEmpty(ctx, "signal_id", b.signalID); err != nil {
		return err
	}
	cloned := core.CloneMap(b.payload)
	if len(cloned) == 0 {
		cloned = make(map[string]any)
	}
	b.config.Signal = &enginetask.SignalConfig{
		ID:      b.signalID,
		Payload: cloned,
	}
	b.config.Type = enginetask.TaskTypeSignal
	b.config.WaitFor = ""
	b.config.Timeout = ""
	return nil
}

func (b *SignalBuilder) applyWaitConfig(ctx context.Context) error {
	if b.dataProvided {
		return fmt.Errorf("wait mode does not accept data payload")
	}
	if !b.timeoutSet || b.timeout == nil {
		return fmt.Errorf("wait mode requires a timeout")
	}
	if err := validate.ValidateDuration(ctx, *b.timeout); err != nil {
		return err
	}
	b.config.Signal = nil
	b.config.WaitFor = b.signalID
	timeout := *b.timeout
	b.config.Timeout = timeout.String()
	b.config.Type = enginetask.TaskTypeWait
	return nil
}
