package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engineruntime "github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// Builder constructs engine runtime configurations for the Bun runtime while accumulating validation errors until Build is invoked.
type Builder struct {
	config *engineruntime.Config
	errors []error
}

// NewBun creates a Builder preconfigured for the Bun runtime using the engine defaults.
func NewBun() *Builder {
	cfg := engineruntime.DefaultConfig()
	cfg.RuntimeType = engineruntime.RuntimeTypeBun
	cfg.BunPermissions = append([]string{}, cfg.BunPermissions...)
	return &Builder{config: cfg, errors: make([]error, 0)}
}

// WithEntrypoint sets the Bun runtime entrypoint script path.
func (b *Builder) WithEntrypoint(path string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("entrypoint path cannot be empty"))
		return b
	}
	b.config.EntrypointPath = trimmed
	return b
}

// WithBunPermissions configures the Bun permission flags that will be passed to the runtime process.
func (b *Builder) WithBunPermissions(permissions ...string) *Builder {
	if b == nil {
		return nil
	}
	if len(permissions) == 0 {
		b.errors = append(b.errors, fmt.Errorf("at least one bun permission must be provided"))
		return b
	}
	normalized := make([]string, 0, len(permissions))
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		trimmed := strings.TrimSpace(permission)
		if trimmed == "" {
			b.errors = append(b.errors, fmt.Errorf("bun permission cannot be empty"))
			continue
		}
		lower := strings.ToLower(trimmed)
		if !isValidBunPermission(lower) {
			b.errors = append(b.errors, fmt.Errorf("invalid bun permission %q", trimmed))
			continue
		}
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		normalized = append(normalized, lower)
	}
	if len(normalized) > 0 {
		b.config.BunPermissions = normalized
	}
	return b
}

// WithMaxMemoryMB sets the maximum memory allocation for the Bun runtime process in megabytes.
func (b *Builder) WithMaxMemoryMB(mb int) *Builder {
	if b == nil {
		return nil
	}
	if mb <= 0 {
		b.errors = append(b.errors, fmt.Errorf("max memory must be positive: got %d", mb))
		return b
	}
	b.config.MaxMemoryMB = mb
	return b
}

// Build validates the accumulated runtime configuration and returns a cloned engine runtime config.
func (b *Builder) Build(ctx context.Context) (*engineruntime.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("runtime builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building runtime configuration", "runtime", b.config.RuntimeType)
	collected := make([]error, 0, len(b.errors)+3)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateEntrypoint(ctx))
	collected = append(collected, b.validateBunPermissions())
	collected = append(collected, b.validateMaxMemory())
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
		return nil, fmt.Errorf("failed to clone runtime config: %w", err)
	}
	return cloned, nil
}

func (b *Builder) validateEntrypoint(ctx context.Context) error {
	b.config.EntrypointPath = strings.TrimSpace(b.config.EntrypointPath)
	if err := validate.ValidateNonEmpty(ctx, "runtime entrypoint", b.config.EntrypointPath); err != nil {
		return err
	}
	return nil
}

func (b *Builder) validateBunPermissions() error {
	if len(b.config.BunPermissions) == 0 {
		return fmt.Errorf("at least one bun permission is required")
	}
	for _, permission := range b.config.BunPermissions {
		if !isValidBunPermission(strings.ToLower(strings.TrimSpace(permission))) {
			return fmt.Errorf("invalid bun permission %q", permission)
		}
	}
	return nil
}

func (b *Builder) validateMaxMemory() error {
	if b.config.MaxMemoryMB <= 0 {
		return fmt.Errorf("max memory must be positive: got %d", b.config.MaxMemoryMB)
	}
	return nil
}

func isValidBunPermission(permission string) bool {
	if permission == "--allow-all" {
		return true
	}
	return strings.HasPrefix(permission, "--allow-")
}
