package ingest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/chunk"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/pkg/logger"
)

type retrySettings struct {
	attempts int
	backoff  time.Duration
	max      time.Duration
}

type Pipeline struct {
	binding   *knowledge.ResolvedBinding
	embedder  embedder.Embedder
	store     vectordb.Store
	options   Options
	chunker   *chunk.Processor
	batchSize int
	retry     retrySettings
}

type Result struct {
	KnowledgeBaseID string
	BindingID       string
	Documents       int
	Chunks          int
	Persisted       int
}

func NewPipeline(
	binding *knowledge.ResolvedBinding,
	emb embedder.Embedder,
	store vectordb.Store,
	opts Options,
) (*Pipeline, error) {
	if binding == nil {
		return nil, errors.New("knowledge: resolved binding is required")
	}
	if emb == nil {
		return nil, errors.New("knowledge: embedder implementation is required")
	}
	if store == nil {
		return nil, errors.New("knowledge: vector store is required")
	}
	settings := chunk.Settings{
		Strategy:          string(binding.KnowledgeBase.Chunking.Strategy),
		Size:              binding.KnowledgeBase.Chunking.Size,
		Overlap:           binding.KnowledgeBase.Chunking.Overlap,
		RemoveHTML:        binding.KnowledgeBase.Preprocess.RemoveHTML,
		Deduplicate:       derefBool(binding.KnowledgeBase.Preprocess.Deduplicate, true),
		NormalizeNewlines: true,
	}
	chunker, err := chunk.NewProcessor(settings)
	if err != nil {
		return nil, err
	}
	batchSize := binding.Embedder.Config.BatchSize
	if batchSize <= 0 {
		defaults := knowledge.DefaultDefaults()
		if defaults.EmbedderBatchSize > 0 {
			batchSize = defaults.EmbedderBatchSize
		} else {
			batchSize = 1
		}
	}
	retryCfg := parseRetry(binding.Embedder.Config.Retry)
	return &Pipeline{
		binding:   binding,
		embedder:  emb,
		store:     store,
		options:   opts,
		chunker:   chunker,
		batchSize: batchSize,
		retry:     retryCfg,
	}, nil
}

func (p *Pipeline) Run(ctx context.Context) (*Result, error) {
	log := logger.FromContext(ctx)
	strategy := p.options.normalizedStrategy()
	if strategy != StrategyUpsert && strategy != StrategyReplace {
		return nil, fmt.Errorf("knowledge: ingestion strategy %q not supported", strategy)
	}
	docs, err := enumerateSources(ctx, &p.binding.KnowledgeBase, &p.options)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return &Result{KnowledgeBaseID: p.binding.KnowledgeBase.ID, BindingID: p.binding.ID}, nil
	}
	chunks, err := p.chunker.Process(p.binding.KnowledgeBase.ID, docs)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return &Result{KnowledgeBaseID: p.binding.KnowledgeBase.ID, BindingID: p.binding.ID, Documents: len(docs)}, nil
	}
	if strategy == StrategyReplace {
		if err := p.deleteExistingRecords(ctx); err != nil {
			return nil, err
		}
	}
	totalPersisted, err := p.persistChunks(ctx, chunks)
	if err != nil {
		return nil, err
	}
	log.Debug(
		"Knowledge ingestion completed",
		"kb_id",
		p.binding.KnowledgeBase.ID,
		"binding_id",
		p.binding.ID,
		"documents",
		len(docs),
		"chunks",
		len(chunks),
		"persisted",
		totalPersisted,
	)
	result := &Result{
		KnowledgeBaseID: p.binding.KnowledgeBase.ID,
		BindingID:       p.binding.ID,
		Documents:       len(docs),
		Chunks:          len(chunks),
		Persisted:       totalPersisted,
	}
	return result, nil
}

func (p *Pipeline) embedBatch(ctx context.Context, batch []chunk.Chunk) ([][]float32, error) {
	texts := make([]string, len(batch))
	for i := range batch {
		texts[i] = batch[i].Text
	}
	var out [][]float32
	var err error
	for attempt := 0; attempt < p.retry.attempts; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			time.Sleep(p.backoffDuration(attempt))
		}
		out, err = p.embedder.EmbedDocuments(ctx, texts)
		if err == nil {
			return out, nil
		}
	}
	return nil, fmt.Errorf("knowledge: embed documents failed: %w", err)
}

func (p *Pipeline) upsertBatch(ctx context.Context, records []vectordb.Record) error {
	var err error
	for attempt := 0; attempt < p.retry.attempts; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			time.Sleep(p.backoffDuration(attempt))
		}
		err = p.store.Upsert(ctx, records)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("knowledge: persist vectors failed: %w", err)
}

func (p *Pipeline) backoffDuration(attempt int) time.Duration {
	if p.retry.backoff <= 0 {
		return 0
	}
	if attempt <= 0 {
		if p.retry.max > 0 && p.retry.backoff > p.retry.max {
			return p.retry.max
		}
		return p.retry.backoff
	}
	delay := p.retry.backoff
	maxDelay := p.retry.max
	if maxDelay > 0 && delay > maxDelay {
		return maxDelay
	}
	for i := 0; i < attempt; i++ {
		if maxDelay > 0 && delay >= maxDelay {
			return maxDelay
		}
		if delay > time.Duration(math.MaxInt64/2) {
			if maxDelay > 0 {
				return maxDelay
			}
			return time.Duration(math.MaxInt64)
		}
		delay *= 2
	}
	if maxDelay > 0 && delay > maxDelay {
		return maxDelay
	}
	return delay
}

func (p *Pipeline) persistChunks(ctx context.Context, chunks []chunk.Chunk) (int, error) {
	total := 0
	for start := 0; start < len(chunks); start += p.batchSize {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		end := start + p.batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[start:end]
		vectors, err := p.embedBatch(ctx, batch)
		if err != nil {
			return 0, err
		}
		if len(vectors) != len(batch) {
			return 0, fmt.Errorf("knowledge: embedder returned %d vectors for %d chunks", len(vectors), len(batch))
		}
		records := make([]vectordb.Record, len(batch))
		for i := range batch {
			meta := cloneMetadata(batch[i].Metadata)
			meta["knowledge_binding_id"] = p.binding.ID
			meta["knowledge_base_id"] = p.binding.KnowledgeBase.ID
			if len(p.binding.KnowledgeBase.Metadata.Tags) > 0 {
				meta["tags"] = p.binding.KnowledgeBase.Metadata.Tags
			}
			if len(p.binding.KnowledgeBase.Metadata.Owners) > 0 {
				meta["owners"] = p.binding.KnowledgeBase.Metadata.Owners
			}
			meta["chunk_hash"] = batch[i].Hash
			records[i] = vectordb.Record{
				ID:        batch[i].ID,
				Text:      batch[i].Text,
				Embedding: vectors[i],
				Metadata:  meta,
			}
		}
		if err := p.upsertBatch(ctx, records); err != nil {
			return 0, err
		}
		total += len(records)
	}
	return total, nil
}

func (p *Pipeline) deleteExistingRecords(ctx context.Context) error {
	filter := vectordb.Filter{Metadata: make(map[string]string, 2)}
	if p.binding.ID != "" {
		filter.Metadata["knowledge_binding_id"] = p.binding.ID
	}
	if kbID := p.binding.KnowledgeBase.ID; kbID != "" {
		filter.Metadata["knowledge_base_id"] = kbID
	}
	if len(filter.Metadata) == 0 && len(filter.IDs) == 0 {
		return nil
	}
	return p.store.Delete(ctx, filter)
}

func parseRetry(cfg map[string]any) retrySettings {
	settings := retrySettings{attempts: 3, backoff: 200 * time.Millisecond, max: 2 * time.Second}
	if len(cfg) == 0 {
		return settings
	}
	if v, ok := lookupInt(cfg, "attempts"); ok && v > 0 {
		settings.attempts = v
	}
	if v, ok := lookupInt(cfg, "max_attempts"); ok && v > 0 {
		settings.attempts = v
	}
	if v, ok := lookupInt(cfg, "backoff_ms"); ok && v > 0 {
		settings.backoff = time.Duration(v) * time.Millisecond
	}
	if v, ok := lookupInt(cfg, "max_backoff_ms"); ok && v > 0 {
		settings.max = time.Duration(v) * time.Millisecond
	}
	if settings.attempts <= 0 {
		settings.attempts = 1
	}
	if settings.backoff <= 0 {
		settings.backoff = 100 * time.Millisecond
	}
	if settings.max < settings.backoff {
		settings.max = settings.backoff
	}
	return settings
}

func lookupInt(m map[string]any, key string) (int, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}

func cloneMetadata(src map[string]any) map[string]any {
	if len(src) == 0 {
		return make(map[string]any, 8)
	}
	dst := make(map[string]any, len(src)+4)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func derefBool(ptr *bool, fallback bool) bool {
	if ptr == nil {
		return fallback
	}
	return *ptr
}
