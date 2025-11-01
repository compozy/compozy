package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engineruntime "github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

// New creates a runtime configuration using functional options
func New(ctx context.Context, runtimeType string, opts ...Option) (*engineruntime.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating runtime configuration", "runtime_type", runtimeType)
	normalizedType := strings.ToLower(strings.TrimSpace(runtimeType))
	cfg := engineruntime.DefaultConfig()
	cfg.RuntimeType = normalizedType
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0, 3)
	if err := validateRuntimeType(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateEntrypoint(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone runtime config: %w", err)
	}
	return cloned, nil
}

func validateRuntimeType(ctx context.Context, cfg *engineruntime.Config) error {
	runtimeType := strings.ToLower(strings.TrimSpace(cfg.RuntimeType))
	if err := validate.NonEmpty(ctx, "runtime type", runtimeType); err != nil {
		return err
	}
	if !engineruntime.IsValidRuntimeType(runtimeType) {
		return fmt.Errorf(
			"runtime type %q is not supported; must be one of %v",
			runtimeType,
			engineruntime.SupportedRuntimeTypes,
		)
	}
	cfg.RuntimeType = runtimeType
	return nil
}

func validateEntrypoint(ctx context.Context, cfg *engineruntime.Config) error {
	entrypoint := strings.TrimSpace(cfg.EntrypointPath)
	cfg.EntrypointPath = entrypoint
	if entrypoint == "" {
		return nil
	}
	if err := validate.NonEmpty(ctx, "entrypoint path", entrypoint); err != nil {
		return err
	}
	return nil
}
