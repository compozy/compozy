package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const defaultRouteKey = "default"

// RouterBuilder constructs router task configurations that perform conditional routing.
// It accumulates validation errors while exposing fluent helpers for branch registration
// and default fallbacks.
type RouterBuilder struct {
	config       *enginetask.Config
	errors       []error
	routes       map[string]string
	defaultRoute string
	hasDefault   bool
}

// NewRouter creates a new router task builder initialized with the provided identifier.
func NewRouter(id string) *RouterBuilder {
	trimmed := strings.TrimSpace(id)
	return &RouterBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeRouter,
			},
		},
		errors: make([]error, 0),
		routes: make(map[string]string),
	}
}

// WithCondition registers the CEL expression evaluated to determine the routing key.
func (b *RouterBuilder) WithCondition(condition string) *RouterBuilder {
	if b == nil {
		return nil
	}
	b.config.Condition = strings.TrimSpace(condition)
	return b
}

// AddRoute maps a routing key produced by the condition expression to the target task identifier.
func (b *RouterBuilder) AddRoute(condition string, taskID string) *RouterBuilder {
	if b == nil {
		return nil
	}
	key := strings.TrimSpace(condition)
	if key == "" {
		b.errors = append(b.errors, fmt.Errorf("route condition cannot be empty"))
		return b
	}
	if key == defaultRouteKey {
		b.errors = append(b.errors, fmt.Errorf("route condition %q is reserved for the default route", defaultRouteKey))
		return b
	}
	target := strings.TrimSpace(taskID)
	if target == "" {
		b.errors = append(b.errors, fmt.Errorf("task id cannot be empty for route %q", key))
		return b
	}
	if _, exists := b.routes[key]; exists {
		b.errors = append(b.errors, fmt.Errorf("duplicate route condition: %s", key))
		return b
	}
	b.routes[key] = target
	return b
}

// WithDefault sets the fallback task executed when no routes match the evaluated key.
func (b *RouterBuilder) WithDefault(taskID string) *RouterBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("default route task id cannot be empty"))
		return b
	}
	b.defaultRoute = trimmed
	b.hasDefault = true
	return b
}

// Build validates the accumulated configuration using the provided context and returns an engine task config.
func (b *RouterBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("router builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug(
		"building router task configuration",
		"task",
		b.config.ID,
		"condition",
		b.config.Condition,
		"routes",
		len(b.routes),
		"hasDefault",
		b.hasDefault,
	)

	collected := append(make([]error, 0, len(b.errors)+3), b.errors...)
	collected = append(
		collected,
		b.validateID(ctx),
		b.validateCondition(ctx),
		b.validateRoutes(ctx),
	)

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
		return nil, fmt.Errorf("failed to clone router task config: %w", err)
	}
	return cloned, nil
}

func (b *RouterBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	b.config.Resource = string(core.ConfigTask)
	b.config.Type = enginetask.TaskTypeRouter
	return nil
}

func (b *RouterBuilder) validateCondition(ctx context.Context) error {
	b.config.Condition = strings.TrimSpace(b.config.Condition)
	if err := validate.NonEmpty(ctx, "condition", b.config.Condition); err != nil {
		return err
	}
	return nil
}

func (b *RouterBuilder) validateRoutes(ctx context.Context) error {
	if len(b.routes) == 0 {
		return fmt.Errorf("router tasks require at least one route")
	}

	compiled := make(map[string]any, len(b.routes)+1)
	for key, target := range b.routes {
		trimmedKey := strings.TrimSpace(key)
		if err := validate.NonEmpty(ctx, "route condition", trimmedKey); err != nil {
			return err
		}
		if err := validate.ID(ctx, target); err != nil {
			return fmt.Errorf("route %s has invalid task id: %w", trimmedKey, err)
		}
		compiled[trimmedKey] = target
	}

	if b.hasDefault {
		if err := validate.ID(ctx, b.defaultRoute); err != nil {
			return fmt.Errorf("default route task id is invalid: %w", err)
		}
		compiled[defaultRouteKey] = b.defaultRoute
	}

	b.config.Routes = compiled
	return nil
}
