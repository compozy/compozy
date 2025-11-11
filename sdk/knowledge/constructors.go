package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

//nolint:gocyclo,funlen // Complex validation logic for multiple config types
func NewBase(ctx context.Context, id string, opts ...BaseOption) (*engineknowledge.BaseConfig, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	cfg := appconfig.FromContext(ctx)
	defaults := engineknowledge.DefaultsFromConfig(cfg)

	config := &engineknowledge.BaseConfig{
		ID:      strings.TrimSpace(id),
		Sources: make([]engineknowledge.SourceConfig, 0),
	}

	for _, opt := range opts {
		opt(config)
	}

	log.Debug("creating knowledge base configuration", "knowledge_base", config.ID, "sources", len(config.Sources))

	collected := make([]error, 0, 12)

	// Validate ID
	config.ID = strings.TrimSpace(config.ID)
	if err := validate.ID(ctx, config.ID); err != nil {
		collected = append(collected, fmt.Errorf("knowledge_base id is invalid: %w", err))
	}

	// Validate embedder
	config.Embedder = strings.TrimSpace(config.Embedder)
	if err := validate.NonEmpty(ctx, "embedder", config.Embedder); err != nil {
		collected = append(collected, fmt.Errorf("embedder id is required: %w", err))
	} else if err := validate.ID(ctx, config.Embedder); err != nil {
		collected = append(collected, fmt.Errorf("embedder id is invalid: %w", err))
	}

	// Validate vector DB
	config.VectorDB = strings.TrimSpace(config.VectorDB)
	if err := validate.NonEmpty(ctx, "vector_db", config.VectorDB); err != nil {
		collected = append(collected, fmt.Errorf("vector_db id is required: %w", err))
	} else if err := validate.ID(ctx, config.VectorDB); err != nil {
		collected = append(collected, fmt.Errorf("vector_db id is invalid: %w", err))
	}

	// Apply defaults and normalize
	if config.Ingest == "" {
		config.Ingest = engineknowledge.IngestManual
	}
	config.Ingest = engineknowledge.IngestMode(strings.ToLower(strings.TrimSpace(string(config.Ingest))))

	// Validate ingest mode
	if config.Ingest != engineknowledge.IngestManual && config.Ingest != engineknowledge.IngestOnStart {
		collected = append(collected, fmt.Errorf("ingest mode %q is not supported", config.Ingest))
	}

	// Validate sources
	if len(config.Sources) == 0 {
		collected = append(collected, fmt.Errorf("at least one source must be added"))
	}

	// Normalize and validate chunking
	if config.Chunking.Strategy == "" {
		config.Chunking.Strategy = engineknowledge.ChunkStrategyRecursiveTextSplitter
	}
	if config.Chunking.Size == 0 {
		config.Chunking.Size = defaults.ChunkSize
	}
	if config.Chunking.Overlap == nil {
		overlap := defaults.ChunkOverlap
		config.Chunking.Overlap = &overlap
	}

	if err := validate.Range(
		ctx, "chunking.size", config.Chunking.Size,
		engineknowledge.MinChunkSize, engineknowledge.MaxChunkSize,
	); err != nil {
		collected = append(collected, err)
	}
	overlap := config.Chunking.OverlapValue()
	maxOverlap := config.Chunking.Size - 1
	if maxOverlap < 0 {
		maxOverlap = 0
	}
	if err := validate.Range(ctx, "chunking.overlap", overlap, 0, maxOverlap); err != nil {
		collected = append(collected, err)
	}
	if config.Chunking.Size <= overlap {
		collected = append(
			collected,
			fmt.Errorf(
				"chunking.overlap must be less than chunking.size: overlap %d, size %d",
				overlap,
				config.Chunking.Size,
			),
		)
	}

	// Normalize preprocess
	if config.Preprocess.Deduplicate == nil {
		val := true
		config.Preprocess.Deduplicate = &val
	}

	// Normalize and validate retrieval
	if config.Retrieval.TopK <= 0 {
		config.Retrieval.TopK = defaults.RetrievalTopK
	}
	if config.Retrieval.MinScore == nil {
		score := defaults.RetrievalMinScore
		config.Retrieval.MinScore = &score
	}
	if config.Retrieval.MaxTokens <= 0 {
		config.Retrieval.MaxTokens = 1200 // default from old builder
	}

	if config.Retrieval.TopK <= 0 || config.Retrieval.TopK > 50 {
		collected = append(
			collected,
			fmt.Errorf("retrieval.top_k must be between 1 and 50: got %d", config.Retrieval.TopK),
		)
	}
	minScore := config.Retrieval.MinScoreValue()
	if minScore < engineknowledge.MinScoreFloor || minScore > engineknowledge.MaxScoreCeiling {
		collected = append(
			collected,
			fmt.Errorf(
				"retrieval.min_score must be between %.2f and %.2f: got %.4f",
				engineknowledge.MinScoreFloor,
				engineknowledge.MaxScoreCeiling,
				minScore,
			),
		)
	}
	if config.Retrieval.MaxTokens <= 0 {
		collected = append(
			collected,
			fmt.Errorf("retrieval.max_tokens must be greater than zero: got %d", config.Retrieval.MaxTokens),
		)
	}

	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}

	cloned, err := core.DeepCopy(config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone knowledge base config: %w", err)
	}
	return cloned, nil
}

// NewBinding creates a knowledge binding configuration using functional options
func NewBinding(ctx context.Context, id string, opts ...BindingOption) (*core.KnowledgeBinding, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)

	config := &core.KnowledgeBinding{
		ID: strings.TrimSpace(id),
	}

	for _, opt := range opts {
		opt(config)
	}

	log.Debug("creating knowledge binding", "knowledge_base", config.ID)

	collected := make([]error, 0, 5)

	// Validate ID
	config.ID = strings.TrimSpace(config.ID)
	if err := validate.ID(ctx, config.ID); err != nil {
		collected = append(collected, fmt.Errorf("knowledge binding id is invalid: %w", err))
	}

	// Validate optional overrides
	if config.TopK != nil && *config.TopK <= 0 {
		collected = append(collected, fmt.Errorf("top_k override must be greater than zero: got %d", *config.TopK))
	}
	if config.MinScore != nil {
		score := *config.MinScore
		if score < 0.0 || score > 1.0 {
			collected = append(
				collected,
				fmt.Errorf("min_score override must be between 0.0 and 1.0 inclusive: got %.4f", score),
			)
		}
	}
	if config.MaxTokens != nil && *config.MaxTokens <= 0 {
		collected = append(
			collected,
			fmt.Errorf("max_tokens override must be greater than zero: got %d", *config.MaxTokens),
		)
	}

	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}

	// Clone using the built-in Clone method
	cloned := config.Clone()
	return &cloned, nil
}

//nolint:funlen // Comprehensive validation requires extended function
func NewEmbedder(
	ctx context.Context,
	id string,
	provider string,
	model string,
	opts ...EmbedderOption,
) (*engineknowledge.EmbedderConfig, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	cfg := appconfig.FromContext(ctx)
	defaults := engineknowledge.DefaultsFromConfig(cfg)

	config := &engineknowledge.EmbedderConfig{
		ID:       strings.TrimSpace(id),
		Provider: strings.ToLower(strings.TrimSpace(provider)),
		Model:    strings.TrimSpace(model),
		Config:   engineknowledge.EmbedderRuntimeConfig{},
	}

	for _, opt := range opts {
		opt(config)
	}

	log.Debug("creating embedder configuration", "embedder", config.ID, "provider", config.Provider)

	collected := make([]error, 0, 8)

	// Validate ID
	config.ID = strings.TrimSpace(config.ID)
	if err := validate.ID(ctx, config.ID); err != nil {
		collected = append(collected, fmt.Errorf("embedder id is invalid: %w", err))
	}

	// Validate provider
	config.Provider = strings.ToLower(strings.TrimSpace(config.Provider))
	if err := validate.NonEmpty(ctx, "provider", config.Provider); err != nil {
		collected = append(collected, err)
	} else {
		supportedProviders := map[string]struct{}{
			"openai": {},
			"google": {},
			"azure":  {},
			"cohere": {},
			"ollama": {},
		}
		if _, ok := supportedProviders[config.Provider]; !ok {
			collected = append(
				collected,
				fmt.Errorf(
					"provider %q is not supported; must be one of openai, google, azure, cohere, ollama",
					config.Provider,
				),
			)
		}
	}

	// Validate model
	config.Model = strings.TrimSpace(config.Model)
	if err := validate.NonEmpty(ctx, "model", config.Model); err != nil {
		collected = append(collected, err)
	}

	// Validate dimension
	if config.Config.Dimension <= 0 {
		collected = append(
			collected,
			fmt.Errorf("config.dimension must be greater than zero: got %d", config.Config.Dimension),
		)
	}

	// Apply defaults
	if config.Config.BatchSize <= 0 {
		config.Config.BatchSize = defaults.EmbedderBatchSize
	}
	if config.Config.MaxConcurrentWorkers <= 0 {
		config.Config.MaxConcurrentWorkers = 4 // default from old builder
	}

	// Validate after applying defaults
	if config.Config.BatchSize <= 0 {
		collected = append(
			collected,
			fmt.Errorf("config.batch_size must be greater than zero: got %d", config.Config.BatchSize),
		)
	}
	if config.Config.MaxConcurrentWorkers <= 0 {
		collected = append(
			collected,
			fmt.Errorf(
				"config.max_concurrent_workers must be greater than zero: got %d",
				config.Config.MaxConcurrentWorkers,
			),
		)
	}

	config.APIKey = strings.TrimSpace(config.APIKey)

	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}

	cloned, err := core.DeepCopy(config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone embedder config: %w", err)
	}
	return cloned, nil
}

//nolint:gocyclo,funlen // Complex validation logic for multiple vector DB types
func NewVectorDB(
	ctx context.Context,
	id string,
	dbType string,
	opts ...VectorDBOption,
) (*engineknowledge.VectorDBConfig, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)

	normalizedType := engineknowledge.VectorDBType(strings.ToLower(strings.TrimSpace(dbType)))

	config := &engineknowledge.VectorDBConfig{
		ID:   strings.TrimSpace(id),
		Type: normalizedType,
		Config: engineknowledge.VectorDBConnConfig{
			PGVector: nil,
		},
	}

	for _, opt := range opts {
		opt(config)
	}

	log.Debug("creating vector db configuration", "vector_db", config.ID, "type", config.Type)

	collected := make([]error, 0, 10)

	// Validate ID
	config.ID = strings.TrimSpace(config.ID)
	if err := validate.ID(ctx, config.ID); err != nil {
		collected = append(collected, fmt.Errorf("vector_db id is invalid: %w", err))
	}

	// Validate type
	config.Type = engineknowledge.VectorDBType(strings.ToLower(strings.TrimSpace(string(config.Type))))
	supportedTypes := map[engineknowledge.VectorDBType]struct{}{
		engineknowledge.VectorDBTypePGVector:   {},
		engineknowledge.VectorDBTypeQdrant:     {},
		engineknowledge.VectorDBTypeRedis:      {},
		engineknowledge.VectorDBTypeFilesystem: {},
	}
	if _, ok := supportedTypes[config.Type]; !ok {
		collected = append(
			collected,
			fmt.Errorf(
				"vector_db type %q is not supported; must be one of pgvector, qdrant, redis, filesystem",
				config.Type,
			),
		)
	}

	// Type-specific validation
	switch config.Type {
	case engineknowledge.VectorDBTypePGVector:
		config.Config.DSN = strings.TrimSpace(config.Config.DSN)
		if err := validate.NonEmpty(ctx, "dsn", config.Config.DSN); err != nil {
			collected = append(collected, fmt.Errorf("pgvector requires config.dsn: %w", err))
		}
		if config.Config.Dimension <= 0 {
			collected = append(
				collected,
				fmt.Errorf("config.dimension must be greater than zero: got %d", config.Config.Dimension),
			)
		}
		// Validate PGVector-specific config
		if pg := config.Config.PGVector; pg != nil {
			if idx := pg.Index; idx != nil {
				idx.Type = strings.ToLower(strings.TrimSpace(idx.Type))
				if idx.Type == "" {
					collected = append(collected, fmt.Errorf("pgvector.index.type cannot be empty"))
				}
				if idx.Lists <= 0 {
					collected = append(collected, fmt.Errorf("pgvector.index.lists must be greater than zero"))
				}
			}
			if pool := pg.Pool; pool != nil {
				if pool.MinConns < 0 {
					collected = append(collected, fmt.Errorf("pgvector.pool.min_conns must be >= 0"))
				}
				if pool.MaxConns < 0 {
					collected = append(collected, fmt.Errorf("pgvector.pool.max_conns must be >= 0"))
				}
				if pool.MaxConns > 0 && pool.MinConns > pool.MaxConns {
					collected = append(collected, fmt.Errorf("pgvector.pool.min_conns cannot exceed max_conns"))
				}
			}
		}
	case engineknowledge.VectorDBTypeQdrant:
		config.Config.DSN = strings.TrimSpace(config.Config.DSN)
		if err := validate.NonEmpty(ctx, "dsn", config.Config.DSN); err != nil {
			collected = append(collected, fmt.Errorf("qdrant requires config.dsn: %w", err))
		} else if err := validate.URL(ctx, config.Config.DSN); err != nil {
			collected = append(collected, fmt.Errorf("qdrant config.dsn must be a valid url: %w", err))
		}
		config.Config.Collection = strings.TrimSpace(config.Config.Collection)
		if err := validate.NonEmpty(ctx, "collection", config.Config.Collection); err != nil {
			collected = append(collected, fmt.Errorf("qdrant requires config.collection: %w", err))
		}
		if config.Config.Dimension <= 0 {
			collected = append(
				collected,
				fmt.Errorf("config.dimension must be greater than zero: got %d", config.Config.Dimension),
			)
		}
	case engineknowledge.VectorDBTypeRedis:
		config.Config.DSN = strings.TrimSpace(config.Config.DSN)
		// DSN is optional for Redis
		if config.Config.Dimension <= 0 {
			collected = append(
				collected,
				fmt.Errorf("config.dimension must be greater than zero: got %d", config.Config.Dimension),
			)
		}
	case engineknowledge.VectorDBTypeFilesystem:
		if config.Config.Dimension <= 0 {
			collected = append(
				collected,
				fmt.Errorf("config.dimension must be greater than zero: got %d", config.Config.Dimension),
			)
		}
	}

	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}

	cloned, err := core.DeepCopy(config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone vector db config: %w", err)
	}
	return cloned, nil
}
