package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	// FlushStrategySummarization configures hybrid summarization flushing.
	FlushStrategySummarization FlushStrategyKind = "summarization"
)

const (
	defaultSummarizeThreshold = 0.8
	defaultPersistenceTTL     = "168h"
)

// FlushStrategy captures the flush behavior requested by the builder.
type FlushStrategy struct {
	Kind          FlushStrategyKind
	MaxMessages   int
	Provider      string
	Model         string
	SummaryTokens int
}

// ConfigBuilder constructs engine memory configurations using a fluent API.
type ConfigBuilder struct {
	config        *enginememory.Config
	errors        []error
	provider      string
	model         string
	flushStrategy *FlushStrategy
	expiration    *time.Duration
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

// WithTokenCounter configures provider and model for token counting.
func (b *ConfigBuilder) WithTokenCounter(provider, model string) *ConfigBuilder {
	if b == nil {
		return nil
	}
	b.provider = provider
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

// WithSummarizationFlush configures a hybrid summarization flush strategy.
func (b *ConfigBuilder) WithSummarizationFlush(provider, model string, maxTokens int) *ConfigBuilder {
	if b == nil {
		return nil
	}
	return b.WithFlushStrategy(FlushStrategy{
		Kind:          FlushStrategySummarization,
		Provider:      provider,
		Model:         model,
		SummaryTokens: maxTokens,
	})
}

// WithPrivacy configures how the memory resource is shared across tenants, users, or sessions.
func (b *ConfigBuilder) WithPrivacy(scope PrivacyScope) *ConfigBuilder {
	if b == nil {
		return nil
	}
	b.config.PrivacyScope = scope
	return b
}

// WithExpiration sets an automatic expiration window for stored memory entries.
func (b *ConfigBuilder) WithExpiration(duration time.Duration) *ConfigBuilder {
	if b == nil {
		return nil
	}
	copy := duration
	b.expiration = &copy
	return b
}

// WithPersistence selects the backend used to persist memory state.
func (b *ConfigBuilder) WithPersistence(backend PersistenceBackend) *ConfigBuilder {
	if b == nil {
		return nil
	}
	normalized := memcore.PersistenceType(strings.ToLower(strings.TrimSpace(string(backend))))
	b.config.Persistence.Type = normalized
	if normalized == memcore.InMemoryPersistence {
		b.config.Persistence.TTL = ""
		return b
	}
	b.config.Persistence.TTL = defaultPersistenceTTL
	return b
}

// WithDistributedLocking toggles distributed locking for memory operations.
func (b *ConfigBuilder) WithDistributedLocking(enabled bool) *ConfigBuilder {
	if b == nil {
		return nil
	}
	if !enabled {
		b.config.Locking = nil
		return b
	}
	b.config.Locking = &memcore.LockConfig{}
	return b
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

	flushConfig, flushMessages, flushErrs := b.buildFlushStrategy(ctx)

	collected := make([]error, 0, len(b.errors)+9)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateID(ctx))
	collected = append(collected, b.validateProvider(ctx))
	collected = append(collected, b.validateModel(ctx))
	collected = append(collected, b.validateMaxTokens())
	collected = append(collected, flushErrs...)
	collected = append(collected, b.validatePrivacyScope())
	collected = append(collected, b.validateExpiration())
	collected = append(collected, b.validatePersistence(ctx))
	collected = append(collected, b.validateDistributedLocking())

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

func (b *ConfigBuilder) validatePrivacyScope() error {
	if b.config.PrivacyScope.IsValid() {
		return nil
	}
	return fmt.Errorf("privacy scope '%s' is not supported", b.config.PrivacyScope)
}

func (b *ConfigBuilder) validateExpiration() error {
	if b.expiration == nil {
		return nil
	}
	duration := *b.expiration
	if duration < 0 {
		return fmt.Errorf("expiration duration must be non-negative: got %s", duration)
	}
	if duration == 0 {
		b.config.Expiration = ""
		return nil
	}
	b.config.Expiration = duration.String()
	return nil
}

func (b *ConfigBuilder) validatePersistence(ctx context.Context) error {
	backend := strings.ToLower(strings.TrimSpace(string(b.config.Persistence.Type)))
	if err := validate.ValidateNonEmpty(ctx, "persistence backend", backend); err != nil {
		return err
	}
	switch memcore.PersistenceType(backend) {
	case memcore.InMemoryPersistence:
		b.config.Persistence.Type = memcore.InMemoryPersistence
		ttl := strings.TrimSpace(b.config.Persistence.TTL)
		if ttl == "" {
			b.config.Persistence.TTL = ""
			return nil
		}
		duration, err := core.ParseHumanDuration(ttl)
		if err != nil {
			return fmt.Errorf("persistence ttl must be a valid duration: %w", err)
		}
		if duration < 0 {
			return fmt.Errorf("persistence ttl must be non-negative: got %s", ttl)
		}
		b.config.Persistence.TTL = ttl
		return nil
	case memcore.RedisPersistence:
		ttl := strings.TrimSpace(b.config.Persistence.TTL)
		if ttl == "" {
			ttl = defaultPersistenceTTL
		}
		duration, err := core.ParseHumanDuration(ttl)
		if err != nil {
			return fmt.Errorf("persistence ttl must be a valid duration: %w", err)
		}
		if duration <= 0 {
			return fmt.Errorf("persistence ttl must be greater than zero: got %s", ttl)
		}
		b.config.Persistence.Type = memcore.RedisPersistence
		b.config.Persistence.TTL = ttl
		return nil
	default:
		return fmt.Errorf("persistence backend '%s' is not supported", backend)
	}
}

func (b *ConfigBuilder) validateDistributedLocking() error {
	if b.config.Locking == nil {
		return nil
	}
	if b.config.Persistence.Type == memcore.InMemoryPersistence {
		return fmt.Errorf("distributed locking requires a persistent backend")
	}
	return nil
}

func (b *ConfigBuilder) buildFlushStrategy(ctx context.Context) (*memcore.FlushingStrategyConfig, int, []error) {
	if b.flushStrategy == nil || b.flushStrategy.Kind == FlushStrategyNone {
		return nil, 0, nil
	}
	switch b.flushStrategy.Kind {
	case FlushStrategyFIFO:
		if b.flushStrategy.MaxMessages <= 0 {
			msg := fmt.Errorf(
				"fifo flush requires max messages greater than zero: got %d",
				b.flushStrategy.MaxMessages,
			)
			return nil, 0, []error{msg}
		}
		cfg := &memcore.FlushingStrategyConfig{Type: memcore.SimpleFIFOFlushing}
		return cfg, b.flushStrategy.MaxMessages, nil
	case FlushStrategySummarization:
		errs := make([]error, 0, 3)
		provider := strings.ToLower(strings.TrimSpace(b.flushStrategy.Provider))
		model := strings.TrimSpace(b.flushStrategy.Model)
		if err := validate.ValidateNonEmpty(ctx, "summarization provider", provider); err != nil {
			errs = append(errs, err)
		}
		if err := validate.ValidateNonEmpty(ctx, "summarization model", model); err != nil {
			errs = append(errs, err)
		}
		if b.flushStrategy.SummaryTokens <= 0 {
			errs = append(errs, fmt.Errorf(
				"summarization flush requires summary tokens greater than zero: got %d",
				b.flushStrategy.SummaryTokens,
			))
		}
		if len(errs) > 0 {
			return nil, 0, errs
		}
		cfg := &memcore.FlushingStrategyConfig{
			Type:               memcore.HybridSummaryFlushing,
			SummarizeThreshold: defaultSummarizeThreshold,
			SummaryTokens:      b.flushStrategy.SummaryTokens,
		}
		return cfg, 0, nil
	default:
		return nil, 0, []error{fmt.Errorf("unsupported flush strategy %q", b.flushStrategy.Kind)}
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
