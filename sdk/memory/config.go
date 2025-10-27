package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	enginememory "github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// FlushStrategyKind enumerates supported flush strategy types.
type FlushStrategyKind string

const (
	// FlushStrategyNone leaves flushing to engine defaults.
	FlushStrategyNone FlushStrategyKind = "none"
	// FlushStrategyFIFO configures simple FIFO flushing.
	FlushStrategyFIFO FlushStrategyKind = "fifo"
)

// FlushStrategy captures the flush behavior requested by the builder.
type FlushStrategy struct {
	Kind        FlushStrategyKind
	MaxMessages int
}

// ConfigBuilder constructs engine memory configurations using a fluent API.
type ConfigBuilder struct {
	config        *enginememory.Config
	errors        []error
	provider      string
	model         string
	flushStrategy *FlushStrategy
}

// New creates a memory configuration builder for the supplied identifier.
func New(id string) *ConfigBuilder {
	trimmedID := strings.TrimSpace(id)
	return &ConfigBuilder{
		config: &enginememory.Config{
			Resource:    string(core.ConfigMemory),
			ID:          trimmedID,
			Type:        memcore.TokenBasedMemory,
			Persistence: memcore.PersistenceConfig{Type: memcore.InMemoryPersistence},
		},
		errors: make([]error, 0),
	}
}

// WithProvider records the provider used for memory token operations.
func (b *ConfigBuilder) WithProvider(provider string) *ConfigBuilder {
	if b == nil {
		return nil
	}
	b.provider = provider
	return b
}

// WithModel sets the model leveraged for memory token operations.
func (b *ConfigBuilder) WithModel(model string) *ConfigBuilder {
	if b == nil {
		return nil
	}
	b.model = model
	return b
}

// WithMaxTokens defines the maximum token budget retained in memory.
func (b *ConfigBuilder) WithMaxTokens(max int) *ConfigBuilder {
	if b == nil {
		return nil
	}
	b.config.MaxTokens = max
	return b
}

// WithFlushStrategy assigns a flush strategy for managing stored messages.
func (b *ConfigBuilder) WithFlushStrategy(strategy FlushStrategy) *ConfigBuilder {
	if b == nil {
		return nil
	}
	copy := strategy
	b.flushStrategy = &copy
	return b
}

// WithFIFOFlush configures a FIFO flush strategy with a message cap.
func (b *ConfigBuilder) WithFIFOFlush(maxMessages int) *ConfigBuilder {
	if b == nil {
		return nil
	}
	return b.WithFlushStrategy(FlushStrategy{Kind: FlushStrategyFIFO, MaxMessages: maxMessages})
}

// Build validates inputs, aggregates errors, and returns a memory config.
func (b *ConfigBuilder) Build(ctx context.Context) (*enginememory.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("memory config builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	log := logger.FromContext(ctx)
	log.Debug("building memory configuration", "memory", b.config.ID)

	flushConfig, flushMessages, flushErr := b.buildFlushStrategy()

	collected := make([]error, 0, len(b.errors)+5)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateID(ctx))
	collected = append(collected, b.validateProvider(ctx))
	collected = append(collected, b.validateModel(ctx))
	collected = append(collected, b.validateMaxTokens())
	collected = append(collected, flushErr)

	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	if flushConfig != nil {
		b.config.Flushing = flushConfig
	}
	if flushMessages > 0 {
		b.config.MaxMessages = flushMessages
	}
	if b.provider != "" || b.model != "" {
		normalizedProvider := strings.ToLower(b.provider)
		if b.config.TokenProvider == nil {
			b.config.TokenProvider = &memcore.TokenProviderConfig{}
		}
		b.config.TokenProvider.Provider = normalizedProvider
		b.config.TokenProvider.Model = b.model
	}

	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone memory config: %w", err)
	}
	return cloned, nil
}

func (b *ConfigBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ValidateID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("memory id is invalid: %w", err)
	}
	return nil
}

func (b *ConfigBuilder) validateProvider(ctx context.Context) error {
	normalized := strings.ToLower(strings.TrimSpace(b.provider))
	if err := validate.ValidateNonEmpty(ctx, "provider", normalized); err != nil {
		return err
	}
	b.provider = normalized
	return nil
}

func (b *ConfigBuilder) validateModel(ctx context.Context) error {
	trimmed := strings.TrimSpace(b.model)
	if err := validate.ValidateNonEmpty(ctx, "model", trimmed); err != nil {
		return err
	}
	b.model = trimmed
	return nil
}

func (b *ConfigBuilder) validateMaxTokens() error {
	if b.config.MaxTokens <= 0 {
		return fmt.Errorf("max tokens must be greater than zero: got %d", b.config.MaxTokens)
	}
	return nil
}

func (b *ConfigBuilder) buildFlushStrategy() (*memcore.FlushingStrategyConfig, int, error) {
	if b.flushStrategy == nil || b.flushStrategy.Kind == FlushStrategyNone {
		return nil, 0, nil
	}
	switch b.flushStrategy.Kind {
	case FlushStrategyFIFO:
		if b.flushStrategy.MaxMessages <= 0 {
			return nil, 0, fmt.Errorf(
				"fifo flush requires max messages greater than zero: got %d",
				b.flushStrategy.MaxMessages,
			)
		}
		cfg := &memcore.FlushingStrategyConfig{Type: memcore.SimpleFIFOFlushing}
		return cfg, b.flushStrategy.MaxMessages, nil
	default:
		return nil, 0, fmt.Errorf("unsupported flush strategy %q", b.flushStrategy.Kind)
	}
}

func filterErrors(errs []error) []error {
	if len(errs) == 0 {
		return nil
	}
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}
