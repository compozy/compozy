package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// ReferenceBuilder constructs memory reference configurations attached to agents.
type ReferenceBuilder struct {
	config *ReferenceConfig
	errors []error
}

// NewReference initializes a builder for the provided memory identifier.
func NewReference(memoryID string) *ReferenceBuilder {
	trimmedID := strings.TrimSpace(memoryID)
	return &ReferenceBuilder{
		config: &ReferenceConfig{
			ID:   trimmedID,
			Mode: core.MemoryModeReadWrite,
		},
		errors: make([]error, 0),
	}
}

// WithKey assigns a template used to resolve the memory instance key at runtime.
func (b *ReferenceBuilder) WithKey(keyTemplate string) *ReferenceBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(keyTemplate)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("memory key template cannot be empty"))
		return b
	}
	b.config.Key = trimmed
	return b
}

// Build validates the accumulated configuration and produces a memory reference.
func (b *ReferenceBuilder) Build(ctx context.Context) (*ReferenceConfig, error) {
	if b == nil {
		return nil, fmt.Errorf("memory reference builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	b.config.ID = strings.TrimSpace(b.config.ID)
	log.Debug("building memory reference", "memory", b.config.ID, "mode", b.config.Mode)
	collected := make([]error, 0, len(b.errors)+2)
	collected = append(collected, b.errors...)
	collected = append(collected, validate.ValidateID(ctx, b.config.ID))
	collected = append(collected, b.validateMode())
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	result := *b.config
	return &result, nil
}

func (b *ReferenceBuilder) validateMode() error {
	if b == nil || b.config == nil {
		return fmt.Errorf("memory reference builder is required")
	}
	mode := strings.TrimSpace(b.config.Mode)
	switch mode {
	case core.MemoryModeReadWrite, core.MemoryModeReadOnly:
		b.config.Mode = mode
		return nil
	default:
		return fmt.Errorf(
			"memory access mode must be '%s' or '%s': got '%s'",
			core.MemoryModeReadWrite,
			core.MemoryModeReadOnly,
			mode,
		)
	}
}
