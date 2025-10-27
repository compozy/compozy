package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	engineruntime "github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// Builder constructs engine runtime configurations for the Bun runtime while collecting validation errors.
type Builder struct {
	config *engineruntime.Config
	errors []error
}

var cloneRuntimeConfig = core.DeepCopy[*engineruntime.Config]

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
	runtimeType := strings.TrimSpace(b.config.RuntimeType)
	if runtimeType != "" && runtimeType != engineruntime.RuntimeTypeBun {
		b.errors = append(
			b.errors,
			fmt.Errorf("bun permissions can only be used with bun runtime, got %q", runtimeType),
		)
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
		if !isValidBunPermission(trimmed) {
			b.errors = append(b.errors, fmt.Errorf("invalid bun permission %q", trimmed))
			continue
		}
		normalizedValue := normalizeBunPermission(trimmed)
		if normalizedValue == "" {
			b.errors = append(b.errors, fmt.Errorf("invalid bun permission %q", trimmed))
			continue
		}
		if _, exists := seen[normalizedValue]; exists {
			continue
		}
		seen[normalizedValue] = struct{}{}
		normalized = append(normalized, normalizedValue)
	}
	if len(normalized) > 0 {
		b.config.BunPermissions = normalized
	}
	return b
}

// WithToolTimeout configures the maximum duration allowed for individual tool executions within the runtime.
func (b *Builder) WithToolTimeout(timeout time.Duration) *Builder {
	if b == nil {
		return nil
	}
	if timeout <= 0 {
		b.errors = append(b.errors, fmt.Errorf("tool timeout must be positive: got %s", timeout))
		return b
	}
	b.config.ToolExecutionTimeout = timeout
	return b
}

// WithNativeTools configures builtin native tool enablement for the runtime.
func (b *Builder) WithNativeTools(tools *engineruntime.NativeToolsConfig) *Builder {
	if b == nil {
		return nil
	}
	if tools == nil {
		b.config.NativeTools = nil
		return b
	}
	clone := *tools
	b.config.NativeTools = &clone
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
	collected := append(make([]error, 0, len(b.errors)+4), b.errors...)
	collected = append(
		collected,
		b.validateEntrypoint(ctx),
		b.validatePermissions(),
		b.validateToolTimeout(),
		b.validateMaxMemory(),
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
	cloned, err := cloneRuntimeConfig(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone runtime config: %w", err)
	}
	return cloned, nil
}

func (b *Builder) validateEntrypoint(ctx context.Context) error {
	b.config.EntrypointPath = strings.TrimSpace(b.config.EntrypointPath)
	if err := validate.NonEmpty(ctx, "runtime entrypoint", b.config.EntrypointPath); err != nil {
		return err
	}
	return nil
}

func (b *Builder) validatePermissions() error {
	runtimeType := strings.TrimSpace(b.config.RuntimeType)
	if runtimeType == "" {
		runtimeType = engineruntime.RuntimeTypeBun
	}
	switch runtimeType {
	case engineruntime.RuntimeTypeBun:
		return b.validateBunPermissions()
	default:
		if len(b.config.BunPermissions) > 0 {
			return fmt.Errorf("runtime type %q does not support bun permissions", runtimeType)
		}
		return nil
	}
}

func (b *Builder) validateBunPermissions() error {
	if len(b.config.BunPermissions) == 0 {
		return fmt.Errorf("at least one bun permission is required")
	}
	for _, permission := range b.config.BunPermissions {
		if !isValidBunPermission(permission) {
			return fmt.Errorf("invalid bun permission %q", permission)
		}
	}
	return nil
}

func (b *Builder) validateToolTimeout() error {
	if b.config.ToolExecutionTimeout <= 0 {
		return fmt.Errorf("tool timeout must be positive: got %s", b.config.ToolExecutionTimeout)
	}
	return nil
}

func (b *Builder) validateMaxMemory() error {
	if b.config.MaxMemoryMB <= 0 {
		return fmt.Errorf("max memory must be positive: got %d", b.config.MaxMemoryMB)
	}
	return nil
}

var allowedBunPermissionBases = map[string]struct{}{
	"--allow-read":  {},
	"--allow-write": {},
	"--allow-net":   {},
	"--allow-env":   {},
}

func isValidBunPermission(permission string) bool {
	trimmed := strings.TrimSpace(permission)
	if trimmed == "" {
		return false
	}
	if strings.EqualFold(trimmed, "--allow-all") {
		return true
	}
	idx := strings.Index(trimmed, "=")
	base := trimmed
	if idx >= 0 {
		base = trimmed[:idx]
		value := strings.TrimSpace(trimmed[idx+1:])
		if value == "" {
			return false
		}
	}
	_, ok := allowedBunPermissionBases[strings.ToLower(strings.TrimSpace(base))]
	return ok
}

func normalizeBunPermission(permission string) string {
	trimmed := strings.TrimSpace(permission)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(trimmed, "--allow-all") {
		return "--allow-all"
	}
	idx := strings.Index(trimmed, "=")
	if idx == -1 {
		return strings.ToLower(trimmed)
	}
	base := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
	value := strings.TrimSpace(trimmed[idx+1:])
	if value == "" {
		return ""
	}
	return base + "=" + value
}
