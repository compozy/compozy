package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const defaultMaxConcurrentWorkers = 4

var supportedEmbedderProviders = map[string]struct{}{
	"openai": {},
	"google": {},
	"azure":  {},
	"cohere": {},
	"ollama": {},
}

var embedderProviderList = []string{"openai", "google", "azure", "cohere", "ollama"}

// EmbedderBuilder constructs knowledge embedder configurations using a fluent API.
type EmbedderBuilder struct {
	config *engineknowledge.EmbedderConfig
	errors []error
}

// NewEmbedder creates an embedder builder with the supplied identifier, provider, and model.
func NewEmbedder(id, provider, model string) *EmbedderBuilder {
	trimmedID := strings.TrimSpace(id)
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	trimmedModel := strings.TrimSpace(model)
	return &EmbedderBuilder{
		config: &engineknowledge.EmbedderConfig{
			ID:       trimmedID,
			Provider: normalizedProvider,
			Model:    trimmedModel,
			Config:   engineknowledge.EmbedderRuntimeConfig{},
		},
		errors: make([]error, 0),
	}
}

// WithAPIKey assigns the API key used to authenticate against the provider.
func (b *EmbedderBuilder) WithAPIKey(key string) *EmbedderBuilder {
	if b == nil {
		return nil
	}
	b.config.APIKey = strings.TrimSpace(key)
	return b
}

// WithDimension configures the embedding dimension required by the provider.
func (b *EmbedderBuilder) WithDimension(dim int) *EmbedderBuilder {
	if b == nil {
		return nil
	}
	b.config.Config.Dimension = dim
	return b
}

// WithBatchSize overrides the batch size used during ingestion.
func (b *EmbedderBuilder) WithBatchSize(size int) *EmbedderBuilder {
	if b == nil {
		return nil
	}
	b.config.Config.BatchSize = size
	return b
}

// WithMaxConcurrentWorkers sets the maximum concurrent embedding workers.
func (b *EmbedderBuilder) WithMaxConcurrentWorkers(max int) *EmbedderBuilder {
	if b == nil {
		return nil
	}
	b.config.Config.MaxConcurrentWorkers = max
	return b
}

// Build validates the accumulated configuration and returns an embedder config.
func (b *EmbedderBuilder) Build(ctx context.Context) (*engineknowledge.EmbedderConfig, error) {
	if b == nil {
		return nil, fmt.Errorf("embedder builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	log := logger.FromContext(ctx)
	log.Debug("building embedder configuration", "embedder", b.config.ID, "provider", b.config.Provider)

	cfg := appconfig.FromContext(ctx)
	defaults := engineknowledge.DefaultsFromConfig(cfg)

	collected := make([]error, 0, len(b.errors)+6)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateID(ctx))
	collected = append(collected, b.validateProvider(ctx))
	collected = append(collected, b.validateModel(ctx))
	collected = append(collected, b.validateDimension())
	collected = append(collected, b.validateBatchSize(defaults))
	collected = append(collected, b.validateMaxConcurrentWorkers())

	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	if b.config.Config.BatchSize <= 0 {
		b.config.Config.BatchSize = defaults.EmbedderBatchSize
	}
	if b.config.Config.MaxConcurrentWorkers <= 0 {
		b.config.Config.MaxConcurrentWorkers = defaultMaxConcurrentWorkers
	}
	b.config.APIKey = strings.TrimSpace(b.config.APIKey)

	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone embedder config: %w", err)
	}
	return cloned, nil
}

func (b *EmbedderBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ValidateID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("embedder id is invalid: %w", err)
	}
	return nil
}

func (b *EmbedderBuilder) validateProvider(ctx context.Context) error {
	b.config.Provider = strings.ToLower(strings.TrimSpace(b.config.Provider))
	if err := validate.ValidateNonEmpty(ctx, "provider", b.config.Provider); err != nil {
		return err
	}
	if _, ok := supportedEmbedderProviders[b.config.Provider]; !ok {
		return fmt.Errorf(
			"provider %q is not supported; must be one of %s",
			b.config.Provider,
			strings.Join(embedderProviderList, ", "),
		)
	}
	return nil
}

func (b *EmbedderBuilder) validateModel(ctx context.Context) error {
	b.config.Model = strings.TrimSpace(b.config.Model)
	if err := validate.ValidateNonEmpty(ctx, "model", b.config.Model); err != nil {
		return err
	}
	return nil
}

func (b *EmbedderBuilder) validateDimension() error {
	if b.config.Config.Dimension <= 0 {
		return fmt.Errorf("config.dimension must be greater than zero: got %d", b.config.Config.Dimension)
	}
	return nil
}

func (b *EmbedderBuilder) validateBatchSize(defaults engineknowledge.Defaults) error {
	if b.config.Config.BatchSize < 0 {
		return fmt.Errorf("config.batch_size must be >= 0: got %d", b.config.Config.BatchSize)
	}
	if b.config.Config.BatchSize == 0 {
		b.config.Config.BatchSize = defaults.EmbedderBatchSize
	}
	return nil
}

func (b *EmbedderBuilder) validateMaxConcurrentWorkers() error {
	if b.config.Config.MaxConcurrentWorkers < 0 {
		return fmt.Errorf("config.max_concurrent_workers must be >= 0: got %d", b.config.Config.MaxConcurrentWorkers)
	}
	if b.config.Config.MaxConcurrentWorkers == 0 {
		b.config.Config.MaxConcurrentWorkers = defaultMaxConcurrentWorkers
	}
	return nil
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
