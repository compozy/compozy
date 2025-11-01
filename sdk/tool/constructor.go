package tool

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	enginetool "github.com/compozy/compozy/engine/tool"
	nativeuser "github.com/compozy/compozy/engine/tool/nativeuser"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

// supportedRuntimes defines the list of valid runtime environments
var supportedRuntimes = map[string]struct{}{
	"bun": {},
}

var nativeHandlers sync.Map // map[*enginetool.Config]nativeuser.Handler

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
	cfg.Runtime = strings.TrimSpace(cfg.Runtime)
	cfg.Code = strings.TrimSpace(cfg.Code)
	nativeHandler, handlerProvided := extractNativeHandler(cfg)
	implementation, implErr := cfg.EffectiveImplementation()
	collected := collectValidationErrors(
		ctx,
		cfg,
		implementation,
		implErr,
		nativeHandler,
		handlerProvided,
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
	if implementation != "" {
		cfg.SetImplementation(implementation)
	}
	if implementation == enginetool.ImplementationNative {
		if err := nativeuser.Register(cfg.ID, nativeHandler); err != nil {
			return nil, fmt.Errorf("failed to register native handler: %w", err)
		}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone tool config: %w", err)
	}
	return cloned, nil
}

func extractNativeHandler(cfg *enginetool.Config) (nativeuser.Handler, bool) {
	value, provided := nativeHandlers.LoadAndDelete(cfg)
	if !provided {
		return nil, false
	}
	handler, ok := value.(nativeuser.Handler)
	if !ok {
		return nil, true
	}
	return handler, true
}

func collectValidationErrors(
	ctx context.Context,
	cfg *enginetool.Config,
	implementation string,
	implErr error,
	nativeHandler nativeuser.Handler,
	handlerProvided bool,
) []error {
	errors := []error{
		validateID(ctx, cfg),
		validateName(ctx, cfg),
		validateDescription(ctx, cfg),
	}
	if implErr != nil {
		errors = append(errors, implErr)
	}
	errors = append(
		errors,
		validateRuntime(ctx, cfg, implementation),
		validateCode(ctx, cfg, implementation),
		validateNativeHandler(cfg, implementation, nativeHandler, handlerProvided),
		validateTimeout(ctx, cfg),
	)
	return errors
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

func validateRuntime(ctx context.Context, cfg *enginetool.Config, implementation string) error {
	if implementation == "" {
		return nil
	}
	if implementation == enginetool.ImplementationNative {
		runtime := strings.ToLower(strings.TrimSpace(cfg.Runtime))
		if runtime == "" {
			runtime = enginetool.RuntimeGo
		}
		if runtime != enginetool.RuntimeGo {
			return fmt.Errorf("native tools must use runtime %s: got %s", enginetool.RuntimeGo, cfg.Runtime)
		}
		cfg.Runtime = runtime
		return nil
	}
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

func validateCode(ctx context.Context, cfg *enginetool.Config, implementation string) error {
	if implementation == "" {
		return nil
	}
	if implementation == enginetool.ImplementationNative {
		return nil
	}
	if err := validate.NonEmpty(ctx, "tool code", cfg.Code); err != nil {
		return err
	}
	return nil
}

func validateNativeHandler(
	cfg *enginetool.Config,
	implementation string,
	handler nativeuser.Handler,
	handlerProvided bool,
) error {
	if implementation == "" {
		return nil
	}
	if implementation == enginetool.ImplementationNative {
		if handler == nil {
			return fmt.Errorf("native handler is required for tool %s", cfg.ID)
		}
		return nil
	}
	if handlerProvided {
		return fmt.Errorf("native handler provided but implementation is %s", implementation)
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

// WithNativeHandler registers a Go-native handler for the tool configuration.
// The handler is bound to the tool ID during construction and executed in-process at runtime.
func WithNativeHandler(handler nativeuser.Handler) Option {
	return func(cfg *enginetool.Config) {
		nativeHandlers.Store(cfg, handler)
		cfg.SetImplementation(enginetool.ImplementationNative)
		cfg.Runtime = enginetool.RuntimeGo
	}
}
