package knowledge

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const (
	defaultRetrievalMaxTokens = 1200
	maxRetrievalTopK          = 50
)

var supportedChunkStrategies = map[engineknowledge.ChunkStrategy]struct{}{
	engineknowledge.ChunkStrategyRecursiveTextSplitter: {},
}

var supportedIngestModes = map[engineknowledge.IngestMode]struct{}{
	engineknowledge.IngestManual:  {},
	engineknowledge.IngestOnStart: {},
}

// BaseBuilder constructs knowledge base configurations using a fluent API while accumulating validation errors.
type BaseBuilder struct {
	config               *engineknowledge.BaseConfig
	errors               []error
	chunkingConfigured   bool
	preprocessConfigured bool
	retrievalConfigured  bool
	ingestConfigured     bool
}

// NewBase creates a knowledge base builder for the supplied identifier.
// Example: knowledge.NewBase("docs").WithEmbedder("embedder-id").WithVectorDB("vector-db").AddSource(src).Build(ctx).
func NewBase(id string) *BaseBuilder {
	trimmedID := strings.TrimSpace(id)
	return &BaseBuilder{
		config: &engineknowledge.BaseConfig{
			ID:      trimmedID,
			Sources: make([]engineknowledge.SourceConfig, 0),
		},
		errors: make([]error, 0),
	}
}

// WithDescription assigns a human-readable description to the knowledge base.
func (b *BaseBuilder) WithDescription(desc string) *BaseBuilder {
	if b == nil {
		return nil
	}
	b.config.Description = strings.TrimSpace(desc)
	return b
}

// WithEmbedder sets the embedder identifier used during ingestion.
func (b *BaseBuilder) WithEmbedder(embedderID string) *BaseBuilder {
	if b == nil {
		return nil
	}
	b.config.Embedder = strings.TrimSpace(embedderID)
	return b
}

// WithVectorDB sets the vector database identifier used for storage.
func (b *BaseBuilder) WithVectorDB(vectorDBID string) *BaseBuilder {
	if b == nil {
		return nil
	}
	b.config.VectorDB = strings.TrimSpace(vectorDBID)
	return b
}

// AddSource registers an ingestion source that feeds the knowledge base.
func (b *BaseBuilder) AddSource(source *engineknowledge.SourceConfig) *BaseBuilder {
	if b == nil {
		return nil
	}
	if source == nil {
		b.errors = append(b.errors, fmt.Errorf("source cannot be nil"))
		return b
	}
	cloned, err := core.DeepCopy(source)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("failed to clone source: %w", err))
		return b
	}
	if cloned == nil {
		b.errors = append(b.errors, fmt.Errorf("source cannot be nil"))
		return b
	}
	b.config.Sources = append(b.config.Sources, *cloned)
	return b
}

// WithChunking configures chunking strategy, size, and overlap used during ingestion.
func (b *BaseBuilder) WithChunking(strategy ChunkStrategy, size, overlap int) *BaseBuilder {
	if b == nil {
		return nil
	}
	normalized := engineknowledge.ChunkStrategy(strings.ToLower(strings.TrimSpace(string(strategy))))
	if normalized == "" {
		normalized = engineknowledge.ChunkStrategyRecursiveTextSplitter
	}
	b.config.Chunking.Strategy = normalized
	b.config.Chunking.Size = size
	overlapCopy := overlap
	b.config.Chunking.Overlap = &overlapCopy
	b.chunkingConfigured = true
	return b
}

// WithPreprocess enables preprocessing flags before ingestion runs.
func (b *BaseBuilder) WithPreprocess(dedupe, removeHTML bool) *BaseBuilder {
	if b == nil {
		return nil
	}
	dedupeCopy := dedupe
	b.config.Preprocess.Deduplicate = &dedupeCopy
	b.config.Preprocess.RemoveHTML = removeHTML
	b.preprocessConfigured = true
	return b
}

// WithIngestMode configures when ingestion should execute.
func (b *BaseBuilder) WithIngestMode(mode IngestMode) *BaseBuilder {
	if b == nil {
		return nil
	}
	normalized := engineknowledge.IngestMode(strings.ToLower(strings.TrimSpace(string(mode))))
	b.config.Ingest = normalized
	b.ingestConfigured = true
	return b
}

// WithRetrieval sets retrieval parameters for downstream queries.
func (b *BaseBuilder) WithRetrieval(topK int, minScore float64, maxTokens int) *BaseBuilder {
	if b == nil {
		return nil
	}
	b.config.Retrieval.TopK = topK
	minScoreCopy := minScore
	b.config.Retrieval.MinScore = &minScoreCopy
	b.config.Retrieval.MaxTokens = maxTokens
	b.retrievalConfigured = true
	return b
}

// Build validates the accumulated configuration and returns a cloned knowledge base config.
func (b *BaseBuilder) Build(ctx context.Context) (*engineknowledge.BaseConfig, error) {
	if b == nil {
		return nil, fmt.Errorf("knowledge base builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building knowledge base configuration", "knowledge_base", b.config.ID, "sources", len(b.config.Sources))
	cfg := appconfig.FromContext(ctx)
	defaults := engineknowledge.DefaultsFromConfig(cfg)
	b.applyDefaults(defaults)
	collected := append(make([]error, 0, len(b.errors)+8), b.errors...)
	collected = append(
		collected,
		b.validateID(ctx),
		b.validateEmbedder(ctx),
		b.validateVectorDB(ctx),
		b.validateIngestMode(),
		b.validateSources(),
	)
	collected = append(collected, b.validateChunking(ctx)...)
	collected = append(collected, b.validateRetrieval()...)
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone knowledge base config: %w", err)
	}
	return cloned, nil
}

func (b *BaseBuilder) applyDefaults(defaults engineknowledge.Defaults) {
	b.config.ID = strings.TrimSpace(b.config.ID)
	b.config.Description = strings.TrimSpace(b.config.Description)
	b.config.Embedder = strings.TrimSpace(b.config.Embedder)
	b.config.VectorDB = strings.TrimSpace(b.config.VectorDB)
	b.normalizeIngestMode()
	b.normalizeChunking(defaults)
	b.normalizePreprocess()
	b.normalizeRetrieval(defaults)
}

func (b *BaseBuilder) normalizeIngestMode() {
	if !b.ingestConfigured && b.config.Ingest == "" {
		b.config.Ingest = engineknowledge.IngestManual
		return
	}
	b.config.Ingest = engineknowledge.IngestMode(strings.ToLower(strings.TrimSpace(string(b.config.Ingest))))
}

func (b *BaseBuilder) normalizeChunking(defaults engineknowledge.Defaults) {
	chunk := &b.config.Chunking
	if !b.chunkingConfigured || chunk.Strategy == "" {
		chunk.Strategy = engineknowledge.ChunkStrategyRecursiveTextSplitter
	} else {
		chunk.Strategy = engineknowledge.ChunkStrategy(strings.ToLower(strings.TrimSpace(string(chunk.Strategy))))
	}
	if !b.chunkingConfigured {
		if chunk.Size <= 0 {
			chunk.Size = defaults.ChunkSize
		}
		overlap := defaults.ChunkOverlap
		chunk.Overlap = &overlap
	} else if chunk.Overlap == nil {
		overlap := 0
		chunk.Overlap = &overlap
	}
}

func (b *BaseBuilder) normalizePreprocess() {
	if !b.preprocessConfigured || b.config.Preprocess.Deduplicate == nil {
		value := true
		b.config.Preprocess.Deduplicate = &value
	}
}

func (b *BaseBuilder) normalizeRetrieval(defaults engineknowledge.Defaults) {
	retrieval := &b.config.Retrieval
	if !b.retrievalConfigured && retrieval.TopK <= 0 {
		retrieval.TopK = defaults.RetrievalTopK
	}
	if retrieval.MinScore == nil {
		score := defaults.RetrievalMinScore
		retrieval.MinScore = &score
	}
	if !b.retrievalConfigured && retrieval.MaxTokens <= 0 {
		retrieval.MaxTokens = defaultRetrievalMaxTokens
	}
}

func (b *BaseBuilder) validateID(ctx context.Context) error {
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("knowledge_base id is invalid: %w", err)
	}
	return nil
}

func (b *BaseBuilder) validateEmbedder(ctx context.Context) error {
	if err := validate.NonEmpty(ctx, "embedder", b.config.Embedder); err != nil {
		return fmt.Errorf("embedder id is required: %w", err)
	}
	if err := validate.ID(ctx, b.config.Embedder); err != nil {
		return fmt.Errorf("embedder id is invalid: %w", err)
	}
	return nil
}

func (b *BaseBuilder) validateVectorDB(ctx context.Context) error {
	if err := validate.NonEmpty(ctx, "vector_db", b.config.VectorDB); err != nil {
		return fmt.Errorf("vector_db id is required: %w", err)
	}
	if err := validate.ID(ctx, b.config.VectorDB); err != nil {
		return fmt.Errorf("vector_db id is invalid: %w", err)
	}
	return nil
}

func (b *BaseBuilder) validateIngestMode() error {
	if b.config.Ingest == "" {
		return fmt.Errorf("ingest mode is required")
	}
	if _, ok := supportedIngestModes[b.config.Ingest]; !ok {
		return fmt.Errorf("ingest mode %q is not supported", b.config.Ingest)
	}
	return nil
}

func (b *BaseBuilder) validateSources() error {
	if len(b.config.Sources) == 0 {
		return fmt.Errorf("at least one source must be added")
	}
	return nil
}

func (b *BaseBuilder) validateChunking(ctx context.Context) []error {
	chunk := b.config.Chunking
	errs := make([]error, 0, 3)
	if chunk.Strategy == "" {
		errs = append(errs, fmt.Errorf("chunking.strategy is required"))
	} else if _, ok := supportedChunkStrategies[chunk.Strategy]; !ok {
		errs = append(errs, fmt.Errorf("chunking.strategy %q is not supported", chunk.Strategy))
	}
	if err := validate.Range(
		ctx,
		"chunking.size",
		chunk.Size,
		engineknowledge.MinChunkSize,
		engineknowledge.MaxChunkSize,
	); err != nil {
		errs = append(errs, err)
	}
	overlap := chunk.OverlapValue()
	maxOverlap := chunk.Size - 1
	if maxOverlap < 0 {
		maxOverlap = 0
	}
	if err := validate.Range(ctx, "chunking.overlap", overlap, 0, maxOverlap); err != nil {
		errs = append(errs, err)
	}
	if chunk.Size <= overlap {
		errs = append(
			errs,
			fmt.Errorf("chunking.overlap must be less than chunking.size: overlap %d, size %d", overlap, chunk.Size),
		)
	}
	return errs
}

func (b *BaseBuilder) validateRetrieval() []error {
	retrieval := b.config.Retrieval
	errs := make([]error, 0, 3)
	if retrieval.TopK <= 0 {
		errs = append(errs, fmt.Errorf("retrieval.top_k must be greater than zero: got %d", retrieval.TopK))
	} else if retrieval.TopK > maxRetrievalTopK {
		errs = append(
			errs,
			fmt.Errorf(
				"retrieval.top_k must be less than or equal to %d: got %d",
				maxRetrievalTopK,
				retrieval.TopK,
			),
		)
	}
	minScore := retrieval.MinScoreValue()
	if math.IsNaN(minScore) || minScore < engineknowledge.MinScoreFloor || minScore > engineknowledge.MaxScoreCeiling {
		errs = append(
			errs,
			fmt.Errorf(
				"retrieval.min_score must be between %.2f and %.2f: got %.4f",
				engineknowledge.MinScoreFloor,
				engineknowledge.MaxScoreCeiling,
				minScore,
			),
		)
	}
	if retrieval.MaxTokens <= 0 {
		errs = append(errs, fmt.Errorf("retrieval.max_tokens must be greater than zero: got %d", retrieval.MaxTokens))
	}
	return errs
}
