package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/configutil"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/ingest"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/logger"
)

type IngestInput struct {
	Project  string
	ID       string
	Strategy ingest.Strategy
	CWD      *core.PathCWD
}

type IngestOutput struct {
	Result *ingest.Result
}

type Ingest struct {
	store resources.ResourceStore
}

func NewIngest(store resources.ResourceStore) *Ingest {
	return &Ingest{store: store}
}

func (uc *Ingest) Execute(ctx context.Context, in *IngestInput) (*IngestOutput, error) {
	projectID, kbID, err := validateIngestInput(in)
	if err != nil {
		return nil, err
	}
	kb, emb, vec, err := uc.loadNormalizedTriple(ctx, projectID, kbID)
	if err != nil {
		return nil, err
	}
	binding := newResolvedBinding(kb, emb, vec)
	embAdapter, vecStore, closeStore, err := initIngestAdapters(ctx, projectID, emb, vec)
	if err != nil {
		return nil, err
	}
	defer closeStore(ctx, binding.ID)
	result, err := runIngestPipeline(ctx, binding, embAdapter, vecStore, ingest.Options{
		CWD:      in.CWD,
		Strategy: in.Strategy,
	})
	if err != nil {
		return nil, err
	}
	logIngestResult(ctx, binding.ID, result)
	return &IngestOutput{Result: result}, nil
}

func validateIngestInput(in *IngestInput) (string, string, error) {
	if in == nil {
		return "", "", ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return "", "", ErrProjectMissing
	}
	kbID := strings.TrimSpace(in.ID)
	if kbID == "" {
		return "", "", ErrIDMissing
	}
	return projectID, kbID, nil
}

func (uc *Ingest) loadNormalizedTriple(
	ctx context.Context,
	projectID string,
	kbID string,
) (*knowledge.BaseConfig, *knowledge.EmbedderConfig, *knowledge.VectorDBConfig, error) {
	triple, err := loadKnowledgeTriple(ctx, uc.store, projectID, kbID)
	if err != nil {
		return nil, nil, nil, err
	}
	kb, emb, vec, err := normalizeKnowledgeTriple(ctx, triple)
	if err != nil {
		return nil, nil, nil, err
	}
	return kb, emb, vec, nil
}

func newResolvedBinding(
	kb *knowledge.BaseConfig,
	emb *knowledge.EmbedderConfig,
	vec *knowledge.VectorDBConfig,
) *knowledge.ResolvedBinding {
	return &knowledge.ResolvedBinding{
		ID:            kb.ID,
		KnowledgeBase: *kb,
		Embedder:      *emb,
		Vector:        *vec,
		Retrieval:     kb.Retrieval,
	}
}

func initIngestAdapters(
	ctx context.Context,
	projectID string,
	emb *knowledge.EmbedderConfig,
	vec *knowledge.VectorDBConfig,
) (*embedder.Adapter, vectordb.Store, func(context.Context, string), error) {
	embCfg, err := configutil.ToEmbedderAdapterConfig(emb)
	if err != nil {
		return nil, nil, nil, err
	}
	embAdapter, err := embedder.New(ctx, embCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("init embedder: %w", err)
	}
	vecCfg, err := configutil.ToVectorStoreConfig(ctx, projectID, vec)
	if err != nil {
		return nil, nil, nil, err
	}
	vecStore, release, err := vectordb.AcquireShared(ctx, vecCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("init vector store: %w", err)
	}
	closeFn := func(closeCtx context.Context, bindingID string) {
		log := logger.FromContext(closeCtx)
		if cerr := release(closeCtx); cerr != nil {
			log.Warn(
				"failed to close vector store",
				"kb_id",
				bindingID,
				"error",
				cerr,
			)
		}
	}
	return embAdapter, vecStore, closeFn, nil
}

func runIngestPipeline(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	embAdapter *embedder.Adapter,
	vecStore vectordb.Store,
	opts ingest.Options,
) (*ingest.Result, error) {
	pipeline, err := ingest.NewPipeline(binding, embAdapter, vecStore, opts)
	if err != nil {
		return nil, err
	}
	return pipeline.Run(ctx)
}

func logIngestResult(ctx context.Context, kbID string, result *ingest.Result) {
	logger.FromContext(ctx).Info(
		"knowledge ingestion completed",
		"kb_id", kbID,
		"documents", result.Documents,
		"chunks", result.Chunks,
		"persisted", result.Persisted,
	)
}
