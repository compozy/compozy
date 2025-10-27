package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetool "github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

var supportedRuntimes = map[string]struct{}{
	"bun": {},
}

// Builder constructs engine tool configurations using a fluent API while accumulating validation errors.
type Builder struct {
	config *enginetool.Config
	errors []error
}

var cloneToolConfig = func(cfg *enginetool.Config) (*enginetool.Config, error) {
	return core.DeepCopy(cfg)
}

// New creates a tool builder initialized with the provided identifier.
func New(id string) *Builder {
	trimmed := strings.TrimSpace(id)
	return &Builder{
		config: &enginetool.Config{
			Resource: string(core.ConfigTool),
			ID:       trimmed,
		},
		errors: make([]error, 0),
	}
}

// WithName assigns a human-readable name for the tool used in UIs and logs.
func (b *Builder) WithName(name string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("name cannot be empty"))
		return b
	}
	b.config.Name = trimmed
	return b
}

// WithDescription sets a detailed explanation of the tool capabilities.
func (b *Builder) WithDescription(desc string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("description cannot be empty"))
		return b
	}
	b.config.Description = trimmed
	return b
}

// WithRuntime configures the runtime environment used to execute the tool.
func (b *Builder) WithRuntime(runtime string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.ToLower(strings.TrimSpace(runtime))
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("runtime cannot be empty"))
		return b
	}
	if _, ok := supportedRuntimes[trimmed]; !ok {
		b.errors = append(b.errors, fmt.Errorf("runtime must be bun"))
	}
	b.config.Runtime = trimmed
	return b
}

// WithCode stores the runtime source code executed when the tool runs.
func (b *Builder) WithCode(code string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("code cannot be empty"))
		return b
	}
	b.config.Code = trimmed
	return b
}

// WithInput attaches the input schema used to validate tool invocations.
func (b *Builder) WithInput(schema *engineschema.Schema) *Builder {
	if b == nil {
		return nil
	}
	b.config.InputSchema = schema
	return b
}

// WithOutput attaches the output schema used to validate tool responses.
func (b *Builder) WithOutput(schema *engineschema.Schema) *Builder {
	if b == nil {
		return nil
	}
	b.config.OutputSchema = schema
	return b
}

// Build validates the configuration and returns an engine tool config.
func (b *Builder) Build(ctx context.Context) (*enginetool.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("tool builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	log := logger.FromContext(ctx)
	log.Debug("building tool configuration", "tool", b.config.ID)

	collected := make([]error, 0, len(b.errors)+5)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateID(ctx))
	collected = append(collected, b.validateName(ctx))
	collected = append(collected, b.validateDescription(ctx))
	collected = append(collected, b.validateRuntime(ctx))
	collected = append(collected, b.validateCode(ctx))

	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	clone, err := cloneToolConfig(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone tool config: %w", err)
	}
	return clone, nil
}

func (b *Builder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ValidateID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("tool id is invalid: %w", err)
	}
	return nil
}

func (b *Builder) validateName(ctx context.Context) error {
	b.config.Name = strings.TrimSpace(b.config.Name)
	if err := validate.ValidateNonEmpty(ctx, "tool name", b.config.Name); err != nil {
		return err
	}
	return nil
}

func (b *Builder) validateDescription(ctx context.Context) error {
	b.config.Description = strings.TrimSpace(b.config.Description)
	if err := validate.ValidateNonEmpty(ctx, "tool description", b.config.Description); err != nil {
		return err
	}
	return nil
}

func (b *Builder) validateRuntime(ctx context.Context) error {
	b.config.Runtime = strings.TrimSpace(b.config.Runtime)
	if err := validate.ValidateNonEmpty(ctx, "tool runtime", b.config.Runtime); err != nil {
		return err
	}
	runtime := strings.ToLower(b.config.Runtime)
	if _, ok := supportedRuntimes[runtime]; !ok {
		return fmt.Errorf("tool runtime must be bun: got %s", b.config.Runtime)
	}
	b.config.Runtime = runtime
	return nil
}

func (b *Builder) validateCode(ctx context.Context) error {
	b.config.Code = strings.TrimSpace(b.config.Code)
	if err := validate.ValidateNonEmpty(ctx, "tool code", b.config.Code); err != nil {
		return err
	}
	return nil
}
