package ingest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/chunk"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
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
	tracer    trace.Tracer
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
		Overlap:           binding.KnowledgeBase.Chunking.OverlapValue(),
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
		tracer:    otel.Tracer("compozy.knowledge.ingest"),
	}, nil
}

func (p *Pipeline) Run(ctx context.Context) (result *Result, err error) {
	strategy := p.options.NormalizedStrategy()
	log := logger.FromContext(ctx).With(
		"kb_id", p.binding.KnowledgeBase.ID,
		"binding_id", p.binding.ID,
	)
	start := time.Now()
	ctx, span := p.tracer.Start(ctx, "compozy.knowledge.ingest.run", trace.WithAttributes(
		attribute.String("kb_id", p.binding.KnowledgeBase.ID),
		attribute.String("binding_id", p.binding.ID),
		attribute.String("strategy", string(strategy)),
	))
	defer p.finishRun(ctx, span, start, &result, &err)

	log.Info("Knowledge ingestion started", "strategy", string(strategy))
	if validateErr := p.validateStrategy(strategy); validateErr != nil {
		err = validateErr
		return nil, err
	}
	result, err = p.executeIngestion(ctx, strategy)
	return result, err
}

func (p *Pipeline) finishRun(
	ctx context.Context,
	span trace.Span,
	start time.Time,
	result **Result,
	runErr *error,
) {
	duration := time.Since(start)
	knowledge.RecordIngestDuration(ctx, p.binding.KnowledgeBase.ID, duration)

	log := logger.FromContext(ctx).With(
		"kb_id", p.binding.KnowledgeBase.ID,
		"binding_id", p.binding.ID,
	)
	seconds := duration.Seconds()
	if runErr != nil && *runErr != nil {
		err := *runErr
		log.Error("Knowledge ingestion failed", "error", err, "duration_seconds", seconds)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return
	}

	if result != nil && *result != nil {
		res := *result
		RecordDocuments(ctx, res.Documents, OutcomeSuccess)
		knowledge.RecordIngestChunks(ctx, p.binding.KnowledgeBase.ID, res.Chunks)
		log.Info(
			"Knowledge ingestion finished",
			"documents", res.Documents,
			"chunks", res.Chunks,
			"persisted", res.Persisted,
			"duration_seconds", seconds,
		)
		span.SetAttributes(
			attribute.Int("documents", res.Documents),
			attribute.Int("chunks", res.Chunks),
			attribute.Int("persisted", res.Persisted),
		)
	} else {
		log.Info("Knowledge ingestion finished", "duration_seconds", seconds)
	}
	span.End()
}

func (p *Pipeline) validateStrategy(strategy Strategy) error {
	if strategy != StrategyUpsert && strategy != StrategyReplace {
		return fmt.Errorf("knowledge: ingestion strategy %q not supported", strategy)
	}
	return nil
}

func (p *Pipeline) executeIngestion(ctx context.Context, strategy Strategy) (*Result, error) {
	docs, err := enumerateSources(ctx, &p.binding.KnowledgeBase, &p.options)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return &Result{KnowledgeBaseID: p.binding.KnowledgeBase.ID, BindingID: p.binding.ID}, nil
	}
	RecordBatchSize(ctx, len(docs))
	chunkStart := time.Now()
	chunks, err := p.chunker.Process(p.binding.KnowledgeBase.ID, docs)
	if err != nil {
		RecordError(ctx, StageChunking, categorizeIngestionError(err))
		RecordDocuments(ctx, len(docs), OutcomeError)
		return nil, err
	}
	RecordPipelineStage(ctx, StageChunking, time.Since(chunkStart))
	RecordChunks(ctx, len(chunks))
	if len(chunks) == 0 {
		return &Result{KnowledgeBaseID: p.binding.KnowledgeBase.ID, BindingID: p.binding.ID, Documents: len(docs)}, nil
	}
	if strategy == StrategyReplace {
		if err := p.deleteExistingRecords(ctx); err != nil {
			RecordDocuments(ctx, len(docs), OutcomeError)
			return nil, err
		}
	}
	totalPersisted, err := p.persistChunks(ctx, chunks)
	if err != nil {
		RecordDocuments(ctx, len(docs), OutcomeError)
		return nil, err
	}
	return &Result{
		KnowledgeBaseID: p.binding.KnowledgeBase.ID,
		BindingID:       p.binding.ID,
		Documents:       len(docs),
		Chunks:          len(chunks),
		Persisted:       totalPersisted,
	}, nil
}

func (p *Pipeline) embedBatch(ctx context.Context, batch []chunk.Chunk) ([][]float32, error) {
	ctx, span := p.tracer.Start(ctx, "compozy.knowledge.ingest.embed_batch", trace.WithAttributes(
		attribute.String("kb_id", p.binding.KnowledgeBase.ID),
		attribute.String("binding_id", p.binding.ID),
		attribute.String("embedder_id", p.binding.Embedder.ID),
		attribute.String("embedder_provider", p.binding.Embedder.Provider),
		attribute.String("embedder_model", p.binding.Embedder.Model),
		attribute.Int("batch_size", len(batch)),
	))
	defer span.End()
	start := time.Now()
	texts := make([]string, len(batch))
	for i := range batch {
		texts[i] = batch[i].Text
	}
	var out [][]float32
	var err error
	for attempt := 0; attempt < p.retry.attempts; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				err = ctx.Err()
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				RecordError(ctx, StageEmbedding, categorizeIngestionError(err))
				return nil, err
			}
			time.Sleep(p.backoffDuration(attempt))
		}
		out, err = p.embedder.EmbedDocuments(ctx, texts)
		if err == nil {
			span.SetAttributes(attribute.Int("vectors", len(out)))
			RecordPipelineStage(ctx, StageEmbedding, time.Since(start))
			return out, nil
		}
	}
	if err != nil {
		logger.FromContext(ctx).Error(
			"embed batch failed after retries",
			"attempts",
			p.retry.attempts,
			"error",
			err,
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		RecordError(ctx, StageEmbedding, categorizeIngestionError(err))
	}
	return nil, fmt.Errorf("knowledge: embed documents failed: %w", err)
}

func (p *Pipeline) upsertBatch(ctx context.Context, records []vectordb.Record) error {
	ctx, span := p.tracer.Start(ctx, "compozy.knowledge.ingest.upsert_batch", trace.WithAttributes(
		attribute.String("kb_id", p.binding.KnowledgeBase.ID),
		attribute.String("binding_id", p.binding.ID),
		attribute.String("vector_id", p.binding.Vector.ID),
		attribute.String("vector_type", string(p.binding.Vector.Type)),
		attribute.Int("records", len(records)),
	))
	defer span.End()
	start := time.Now()
	var err error
	for attempt := 0; attempt < p.retry.attempts; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				err = ctx.Err()
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				RecordError(ctx, StageStorage, categorizeIngestionError(err))
				return err
			}
			time.Sleep(p.backoffDuration(attempt))
		}
		err = p.store.Upsert(ctx, records)
		if err == nil {
			RecordPipelineStage(ctx, StageStorage, time.Since(start))
			return nil
		}
	}
	if err != nil {
		logger.FromContext(ctx).Error(
			"upsert batch failed after retries",
			"attempts",
			p.retry.attempts,
			"error",
			err,
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		RecordError(ctx, StageStorage, categorizeIngestionError(err))
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
	batches := p.partitionChunks(chunks)
	if len(batches) == 0 {
		return 0, nil
	}
	workers := p.embeddingWorkerCount(len(batches))
	g, groupCtx := errgroup.WithContext(ctx)
	results := make(chan []vectordb.Record, workers)
	var total int

	g.Go(func() error {
		defer close(results)
		return p.runEmbeddingStage(groupCtx, batches, workers, results)
	})

	g.Go(func() error {
		for records := range results {
			if err := p.upsertBatch(groupCtx, records); err != nil {
				return err
			}
			total += len(records)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return 0, err
	}
	return total, nil
}

func (p *Pipeline) runEmbeddingStage(
	ctx context.Context,
	batches [][]chunk.Chunk,
	workers int,
	out chan<- []vectordb.Record,
) error {
	if workers <= 0 {
		workers = 1
	}
	group, stageCtx := errgroup.WithContext(ctx)
	group.SetLimit(workers)
	for i := range batches {
		batch := batches[i]
		group.Go(func() error {
			vectors, err := p.embedBatch(stageCtx, batch)
			if err != nil {
				return err
			}
			records, buildErr := p.buildRecords(batch, vectors)
			if buildErr != nil {
				RecordError(stageCtx, StageEmbedding, categorizeIngestionError(buildErr))
				return buildErr
			}
			select {
			case <-stageCtx.Done():
				return stageCtx.Err()
			case out <- records:
				return nil
			}
		})
	}
	return group.Wait()
}

func (p *Pipeline) partitionChunks(chunks []chunk.Chunk) [][]chunk.Chunk {
	if len(chunks) == 0 || p.batchSize <= 0 {
		return nil
	}
	total := (len(chunks) + p.batchSize - 1) / p.batchSize
	batches := make([][]chunk.Chunk, 0, total)
	for start := 0; start < len(chunks); start += p.batchSize {
		end := min(start+p.batchSize, len(chunks))
		batches = append(batches, chunks[start:end])
	}
	return batches
}

func (p *Pipeline) embeddingWorkerCount(batchCount int) int {
	maxWorkers := p.binding.Embedder.Config.MaxConcurrentWorkers
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	if batchCount <= 0 {
		return 0
	}
	if batchCount < maxWorkers {
		return batchCount
	}
	return maxWorkers
}

func (p *Pipeline) buildRecords(batch []chunk.Chunk, vectors [][]float32) ([]vectordb.Record, error) {
	if len(vectors) != len(batch) {
		return nil, fmt.Errorf("knowledge: embedder returned %d vectors for %d chunks", len(vectors), len(batch))
	}
	records := make([]vectordb.Record, len(batch))
	for i := range batch {
		metadata := core.CloneMap(batch[i].Metadata)
		metadata["knowledge_binding_id"] = p.binding.ID
		metadata["knowledge_base_id"] = p.binding.KnowledgeBase.ID
		if tags := p.binding.KnowledgeBase.Metadata.Tags; len(tags) > 0 {
			metadata["tags"] = tags
		}
		if owners := p.binding.KnowledgeBase.Metadata.Owners; len(owners) > 0 {
			metadata["owners"] = owners
		}
		metadata["chunk_hash"] = batch[i].Hash
		records[i] = vectordb.Record{
			ID:        batch[i].ID,
			Text:      batch[i].Text,
			Embedding: vectors[i],
			Metadata:  metadata,
		}
	}
	return records, nil
}

func (p *Pipeline) deleteExistingRecords(ctx context.Context) error {
	ctx, span := p.tracer.Start(ctx, "compozy.knowledge.ingest.delete_vectors", trace.WithAttributes(
		attribute.String("kb_id", p.binding.KnowledgeBase.ID),
		attribute.String("binding_id", p.binding.ID),
		attribute.String("vector_id", p.binding.Vector.ID),
		attribute.String("vector_type", string(p.binding.Vector.Type)),
	))
	defer span.End()
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
	start := time.Now()
	err := p.store.Delete(ctx, filter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		RecordError(ctx, StageStorage, categorizeIngestionError(err))
		return err
	}
	RecordPipelineStage(ctx, StageStorage, time.Since(start))
	return err
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

func derefBool(ptr *bool, fallback bool) bool {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

func categorizeIngestionError(err error) string {
	if err == nil {
		return errorTypeInternal
	}
	if errors.Is(err, context.Canceled) {
		return errorTypeCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errorTypeTimeout
	}
	var coreErr *core.Error
	if errors.As(err, &coreErr) && coreErr != nil && coreErr.Code != "" {
		return strings.ToLower(coreErr.Code)
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "rate limit"), strings.Contains(lower, "429"):
		return errorTypeRateLimit
	case strings.Contains(lower, "unauthorized"), strings.Contains(lower, "forbidden"), strings.Contains(lower, "auth"):
		return errorTypeAuth
	case strings.Contains(lower, "invalid"),
		strings.Contains(lower, "bad request"),
		strings.Contains(lower, "422"),
		strings.Contains(lower, "400"):
		return errorTypeInvalid
	case strings.Contains(lower, "timeout"):
		return errorTypeTimeout
	default:
		return errorTypeInternal
	}
}
