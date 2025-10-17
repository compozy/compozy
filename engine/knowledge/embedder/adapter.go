package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/embeddings/cybertron"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/googleai/vertex"
	"github.com/tmc/langchaingo/llms/openai"

	appconfig "github.com/compozy/compozy/pkg/config"
)

// Adapter wraps a langchaingo embedder implementation and augments error reporting.
type Adapter struct {
	id        string
	provider  Provider
	dimension int
	batchSize int
	impl      embeddings.Embedder
	cacheMu   sync.Mutex
	cache     *lru.Cache[string, []float32]
}

var (
	errMissingID        = errors.New("embedder id is required")
	errMissingProvider  = errors.New("embedder provider is required")
	errMissingModel     = errors.New("embedder model is required")
	errInvalidDimension = errors.New("embedder dimension must be greater than zero")
	errInvalidBatchSize = errors.New("embedder batch size must be greater than zero")
)

// New constructs a provider-backed embedder adapter.
func New(ctx context.Context, cfg *Config) (*Adapter, error) {
	if cfg == nil {
		return nil, errors.New("embedder config is required")
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	options := []embeddings.Option{
		embeddings.WithBatchSize(cfg.BatchSize),
		embeddings.WithStripNewLines(cfg.StripNewLines),
	}
	builder, err := buildProviderEmbedder(ctx, cfg, options...)
	if err != nil {
		return nil, err
	}
	return &Adapter{
		id:        cfg.ID,
		provider:  cfg.Provider,
		dimension: cfg.Dimension,
		batchSize: cfg.BatchSize,
		impl:      builder,
	}, nil
}

// Wrap constructs an adapter around an existing langchaingo embedder.
func Wrap(cfg *Config, impl embeddings.Embedder) (*Adapter, error) {
	if cfg == nil {
		return nil, errors.New("embedder config is required")
	}
	if impl == nil {
		return nil, fmt.Errorf("embedder %q: implementation is required", cfg.ID)
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return &Adapter{
		id:        cfg.ID,
		provider:  cfg.Provider,
		dimension: cfg.Dimension,
		batchSize: cfg.BatchSize,
		impl:      impl,
	}, nil
}

// Dimension returns the configured vector dimension.
func (a *Adapter) Dimension() int {
	return a.dimension
}

// BatchSize returns the configured batch size.
func (a *Adapter) BatchSize() int {
	return a.batchSize
}

// EnableCache initializes an LRU cache for embeddings.
func (a *Adapter) EnableCache(size int) error {
	if size <= 0 {
		return fmt.Errorf("embedder %q: cache size must be greater than zero", a.id)
	}
	cache, err := lru.New[string, []float32](size)
	if err != nil {
		return fmt.Errorf("embedder %q: init cache: %w", a.id, err)
	}
	a.cacheMu.Lock()
	a.cache = cache
	a.cacheMu.Unlock()
	return nil
}

// EmbedDocuments delegates to the underlying implementation with contextual errors.
func (a *Adapter) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if cache := a.getCache(); cache != nil {
		return a.cachedEmbedDocuments(ctx, cache, texts)
	}
	vectors, err := a.impl.EmbedDocuments(ctx, texts)
	if err != nil {
		return nil, a.withContext(err)
	}
	return vectors, nil
}

// EmbedQuery delegates to the underlying implementation with contextual errors.
func (a *Adapter) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if cache := a.getCache(); cache != nil {
		if vector, ok := a.lookupCache(cache, text); ok {
			return vector, nil
		}
		result, err := a.impl.EmbedQuery(ctx, text)
		if err != nil {
			return nil, a.withContext(err)
		}
		cloned := cloneVector(result)
		a.storeCache(cache, text, cloned)
		return cloned, nil
	}
	vector, err := a.impl.EmbedQuery(ctx, text)
	if err != nil {
		return nil, a.withContext(err)
	}
	return vector, nil
}

func (a *Adapter) cachedEmbedDocuments(
	ctx context.Context,
	cache *lru.Cache[string, []float32],
	texts []string,
) ([][]float32, error) {
	results := make([][]float32, len(texts))
	missing := make([]string, 0)
	missingIdx := make([]int, 0)
	for i := range texts {
		text := texts[i]
		if vector, ok := a.lookupCache(cache, text); ok {
			results[i] = vector
			continue
		}
		missing = append(missing, text)
		missingIdx = append(missingIdx, i)
	}
	if len(missing) == 0 {
		return results, nil
	}
	embedded, err := a.impl.EmbedDocuments(ctx, missing)
	if err != nil {
		return nil, a.withContext(err)
	}
	if len(embedded) != len(missing) {
		return nil, a.withContext(fmt.Errorf("received %d embeddings for %d texts", len(embedded), len(missing)))
	}
	for i := range embedded {
		idx := missingIdx[i]
		cloned := cloneVector(embedded[i])
		results[idx] = cloned
		a.storeCache(cache, missing[i], cloned)
	}
	return results, nil
}

func (a *Adapter) getCache() *lru.Cache[string, []float32] {
	a.cacheMu.Lock()
	cache := a.cache
	a.cacheMu.Unlock()
	return cache
}

func (a *Adapter) lookupCache(cache *lru.Cache[string, []float32], text string) ([]float32, bool) {
	if cache == nil {
		return nil, false
	}
	key := cacheKey(text)
	a.cacheMu.Lock()
	current := a.cache
	if current == nil || current != cache {
		a.cacheMu.Unlock()
		return nil, false
	}
	value, ok := current.Get(key)
	a.cacheMu.Unlock()
	if !ok {
		return nil, false
	}
	return cloneVector(value), true
}

func (a *Adapter) storeCache(cache *lru.Cache[string, []float32], text string, vector []float32) {
	if cache == nil || len(vector) == 0 {
		return
	}
	key := cacheKey(text)
	cloned := cloneVector(vector)
	a.cacheMu.Lock()
	if a.cache == cache && a.cache != nil {
		a.cache.Add(key, cloned)
	}
	a.cacheMu.Unlock()
}

func (a *Adapter) withContext(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("embedder %q: %w", a.id, err)
}

func cacheKey(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func cloneVector(src []float32) []float32 {
	if len(src) == 0 {
		return nil
	}
	dst := make([]float32, len(src))
	copy(dst, src)
	return dst
}

func validateConfig(cfg *Config) error {
	if strings.TrimSpace(cfg.ID) == "" {
		return errMissingID
	}
	if strings.TrimSpace(string(cfg.Provider)) == "" {
		return fmt.Errorf("embedder %q: %w", cfg.ID, errMissingProvider)
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("embedder %q: %w", cfg.ID, errMissingModel)
	}
	if cfg.Dimension <= 0 {
		return fmt.Errorf("embedder %q: %w", cfg.ID, errInvalidDimension)
	}
	if cfg.BatchSize <= 0 {
		return fmt.Errorf("embedder %q: %w", cfg.ID, errInvalidBatchSize)
	}
	return nil
}

func buildProviderEmbedder(
	ctx context.Context,
	cfg *Config,
	options ...embeddings.Option,
) (embeddings.Embedder, error) {
	switch cfg.Provider {
	case ProviderOpenAI:
		return buildOpenAIEmbedder(cfg, options...)
	case ProviderVertex:
		return buildVertexEmbedder(ctx, cfg, options...)
	case ProviderLocal:
		return buildLocalEmbedder(cfg, options...)
	default:
		return nil, fmt.Errorf("embedder %q: provider %q is not supported", cfg.ID, cfg.Provider)
	}
}

func buildOpenAIEmbedder(cfg *Config, opts ...embeddings.Option) (embeddings.Embedder, error) {
	openaiOpts := []openai.Option{
		openai.WithEmbeddingModel(cfg.Model),
	}
	if cfg.APIKey != "" {
		openaiOpts = append(openaiOpts, openai.WithToken(cfg.APIKey))
	}
	client, err := openai.New(openaiOpts...)
	if err != nil {
		return nil, fmt.Errorf("embedder %q: failed to initialize openai client: %w", cfg.ID, err)
	}
	embedder, err := embeddings.NewEmbedder(client, opts...)
	if err != nil {
		return nil, fmt.Errorf("embedder %q: failed to construct openai embedder: %w", cfg.ID, err)
	}
	return embedder, nil
}

func buildVertexEmbedder(
	ctx context.Context,
	cfg *Config,
	opts ...embeddings.Option,
) (embeddings.Embedder, error) {
	appconfig.FromContext(ctx) // ensure configuration is loaded for downstream providers
	vertexOpts := []googleai.Option{
		googleai.WithDefaultEmbeddingModel(cfg.Model),
	}
	if cfg.APIKey != "" {
		vertexOpts = append(vertexOpts, googleai.WithAPIKey(cfg.APIKey))
	}
	project := lookupString(cfg.Options, "project_id")
	location := lookupString(cfg.Options, "location")
	if project != "" {
		vertexOpts = append(vertexOpts, googleai.WithCloudProject(project))
	}
	if location != "" {
		vertexOpts = append(vertexOpts, googleai.WithCloudLocation(location))
	}
	client, err := vertex.New(ctx, vertexOpts...)
	if err != nil {
		return nil, fmt.Errorf("embedder %q: failed to initialize vertex client: %w", cfg.ID, err)
	}
	embedder, err := embeddings.NewEmbedder(client, opts...)
	if err != nil {
		return nil, fmt.Errorf("embedder %q: failed to construct vertex embedder: %w", cfg.ID, err)
	}
	return embedder, nil
}

func buildLocalEmbedder(cfg *Config, opts ...embeddings.Option) (embeddings.Embedder, error) {
	localClient, err := newLocalEmbedderClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("embedder %q: failed to initialize local embedder: %w", cfg.ID, err)
	}
	embedder, err := embeddings.NewEmbedder(localClient, opts...)
	if err != nil {
		return nil, fmt.Errorf("embedder %q: failed to construct local embedder: %w", cfg.ID, err)
	}
	return embedder, nil
}

func newLocalEmbedderClient(cfg *Config) (embeddings.EmbedderClient, error) {
	model := strings.TrimSpace(cfg.Model)
	opts := make([]cybertron.Option, 0, 2)
	if model != "" {
		opts = append(opts, cybertron.WithModel(model))
	}
	if dir := lookupString(cfg.Options, "models_dir"); dir != "" {
		opts = append(opts, cybertron.WithModelsDir(dir))
	}
	client, err := cybertron.NewCybertron(opts...)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func lookupString(options map[string]any, key string) string {
	if len(options) == 0 {
		return ""
	}
	val, ok := options[key]
	if !ok {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	default:
		return ""
	}
}
