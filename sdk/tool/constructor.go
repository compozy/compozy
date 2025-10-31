package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	enginetool "github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

// supportedRuntimes defines the list of valid runtime environments
var supportedRuntimes = map[string]struct{}{
	"bun": {},
}

// New creates a tool configuration using functional options
func New(ctx context.Context, id string, opts ...Option) (*enginetool.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating tool configuration", "tool", id)
	cfg := &enginetool.Config{
		Resource: string(core.ConfigTool),
		ID:       strings.TrimSpace(id),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateName(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateDescription(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateRuntime(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateCode(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateTimeout(ctx, cfg); err != nil {
		collected = append(collected, err)
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
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone tool config: %w", err)
	}
	return cloned, nil
}

func validateID(ctx context.Context, cfg *enginetool.Config) error {
	cfg.ID = strings.TrimSpace(cfg.ID)
	if err := validate.ID(ctx, cfg.ID); err != nil {
		return fmt.Errorf("tool id is invalid: %w", err)
	}
	return nil
}

func validateName(ctx context.Context, cfg *enginetool.Config) error {
	cfg.Name = strings.TrimSpace(cfg.Name)
	if err := validate.NonEmpty(ctx, "tool name", cfg.Name); err != nil {
		return err
	}
	return nil
}

func validateDescription(ctx context.Context, cfg *enginetool.Config) error {
	cfg.Description = strings.TrimSpace(cfg.Description)
	if err := validate.NonEmpty(ctx, "tool description", cfg.Description); err != nil {
		return err
	}
	return nil
}

func validateRuntime(ctx context.Context, cfg *enginetool.Config) error {
	cfg.Runtime = strings.TrimSpace(cfg.Runtime)
	if err := validate.NonEmpty(ctx, "tool runtime", cfg.Runtime); err != nil {
		return err
	}
	runtime := strings.ToLower(cfg.Runtime)
	if _, ok := supportedRuntimes[runtime]; !ok {
		return fmt.Errorf("tool runtime must be bun: got %s", cfg.Runtime)
	}
	cfg.Runtime = runtime
	return nil
}

func validateCode(ctx context.Context, cfg *enginetool.Config) error {
	cfg.Code = strings.TrimSpace(cfg.Code)
	if err := validate.NonEmpty(ctx, "tool code", cfg.Code); err != nil {
		return err
	}
	return nil
}

func validateTimeout(_ context.Context, cfg *enginetool.Config) error {
	if cfg.Timeout == "" {
		return nil
	}
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout format '%s': %w", cfg.Timeout, err)
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", timeout)
	}
	return nil
}
