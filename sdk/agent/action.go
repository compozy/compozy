package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// ActionBuilder constructs agent action configurations with fluent helpers.
type ActionBuilder struct {
	config *engineagent.ActionConfig
	errors []error
}

// NewAction creates an action builder for the provided identifier.
func NewAction(id string) *ActionBuilder {
	trimmed := strings.TrimSpace(id)
	return &ActionBuilder{
		config: &engineagent.ActionConfig{
			ID:    trimmed,
			Tools: make([]tool.Config, 0),
		},
		errors: make([]error, 0),
	}
}

// WithPrompt sets the prompt executed when the action runs.
func (a *ActionBuilder) WithPrompt(prompt string) *ActionBuilder {
	if a == nil {
		return nil
	}
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		a.errors = append(a.errors, fmt.Errorf("prompt cannot be empty"))
		return a
	}
	a.config.Prompt = trimmed
	return a
}

// WithOutput assigns the output schema used to validate action responses.
func (a *ActionBuilder) WithOutput(output *schema.Schema) *ActionBuilder {
	if a == nil {
		return nil
	}
	a.config.OutputSchema = output
	return a
}

// AddTool registers a tool scoped to this action.
func (a *ActionBuilder) AddTool(toolID string) *ActionBuilder {
	if a == nil {
		return nil
	}
	trimmed := strings.TrimSpace(toolID)
	if trimmed == "" {
		a.errors = append(a.errors, fmt.Errorf("tool id cannot be empty"))
		return a
	}
	a.config.Tools = append(a.config.Tools, tool.Config{ID: trimmed, Resource: string(core.ConfigTool)})
	return a
}

// WithSuccessTransition configures the action success transition to another task.
func (a *ActionBuilder) WithSuccessTransition(taskID string) *ActionBuilder {
	if a == nil {
		return nil
	}
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		a.errors = append(a.errors, fmt.Errorf("success transition task id cannot be empty"))
		return a
	}
	next := trimmed
	a.config.OnSuccess = &core.SuccessTransition{Next: &next}
	return a
}

// WithErrorTransition configures the action error transition to another task.
func (a *ActionBuilder) WithErrorTransition(taskID string) *ActionBuilder {
	if a == nil {
		return nil
	}
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		a.errors = append(a.errors, fmt.Errorf("error transition task id cannot be empty"))
		return a
	}
	next := trimmed
	a.config.OnError = &core.ErrorTransition{Next: &next}
	return a
}

// WithRetry sets retry behavior for the action using maximum attempts and initial backoff.
func (a *ActionBuilder) WithRetry(maxAttempts int, backoff time.Duration) *ActionBuilder {
	if a == nil {
		return nil
	}
	if maxAttempts <= 0 {
		a.errors = append(a.errors, fmt.Errorf("max attempts must be positive"))
		return a
	}
	if backoff <= 0 {
		a.errors = append(a.errors, fmt.Errorf("backoff must be positive"))
		return a
	}
	a.config.RetryPolicy = &core.RetryPolicyConfig{
		MaximumAttempts:    int32(maxAttempts),
		InitialInterval:    backoff.String(),
		BackoffCoefficient: 2.0,
	}
	return a
}

// WithTimeout limits how long the action can execute before timing out.
func (a *ActionBuilder) WithTimeout(timeout time.Duration) *ActionBuilder {
	if a == nil {
		return nil
	}
	if timeout <= 0 {
		a.errors = append(a.errors, fmt.Errorf("timeout must be positive"))
		return a
	}
	a.config.Timeout = timeout.String()
	return a
}

// Build validates the configuration and returns the resulting action config.
func (a *ActionBuilder) Build(ctx context.Context) (*engineagent.ActionConfig, error) {
	if a == nil {
		return nil, fmt.Errorf("action builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building agent action", "action", a.config.ID)
	collected := make([]error, 0, len(a.errors)+4)
	collected = append(collected, a.errors...)
	collected = append(collected, a.validateID(ctx))
	collected = append(collected, a.validatePrompt(ctx))
	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	clone, err := core.DeepCopy(a.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone action config: %w", err)
	}
	return clone, nil
}

func (a *ActionBuilder) validateID(ctx context.Context) error {
	a.config.ID = strings.TrimSpace(a.config.ID)
	if err := validate.ValidateID(ctx, a.config.ID); err != nil {
		return fmt.Errorf("action id is invalid: %w", err)
	}
	return nil
}

func (a *ActionBuilder) validatePrompt(ctx context.Context) error {
	a.config.Prompt = strings.TrimSpace(a.config.Prompt)
	if err := validate.ValidateNonEmpty(ctx, "prompt", a.config.Prompt); err != nil {
		return err
	}
	return nil
}
