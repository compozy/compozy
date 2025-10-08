package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
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
	if in == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return nil, ErrProjectMissing
	}
	kbID := strings.TrimSpace(in.ID)
	if kbID == "" {
		return nil, ErrIDMissing
	}
	triple, err := loadKnowledgeTriple(ctx, uc.store, projectID, kbID)
	if err != nil {
		return nil, err
	}
	kb, emb, vec, err := normalizeKnowledgeTriple(ctx, triple)
	if err != nil {
		return nil, err
	}
	embCfg, err := toEmbedderAdapterConfig(emb)
	if err != nil {
		return nil, err
	}
	embAdapter, err := embedder.New(ctx, embCfg)
	if err != nil {
		return nil, fmt.Errorf("init embedder: %w", err)
	}
	vecCfg, err := toVectorStoreConfig(projectID, vec)
	if err != nil {
		return nil, err
	}
	vecStore, err := vectordb.New(ctx, vecCfg)
	if err != nil {
		return nil, fmt.Errorf("init vector store: %w", err)
	}
	defer func() {
		if cerr := vecStore.Close(ctx); cerr != nil {
			logger.FromContext(ctx).Warn(
				"failed to close vector store",
				"kb_id",
				kb.ID,
				"error",
				cerr,
			)
		}
	}()
	binding := &knowledge.ResolvedBinding{
		ID:            kb.ID,
		KnowledgeBase: *kb,
		Embedder:      *emb,
		Vector:        *vec,
		Retrieval:     kb.Retrieval,
	}
	options := ingest.Options{CWD: in.CWD, Strategy: in.Strategy}
	pipeline, err := ingest.NewPipeline(binding, embAdapter, vecStore, options)
	if err != nil {
		return nil, err
	}
	result, err := pipeline.Run(ctx)
	if err != nil {
		return nil, err
	}
	log := logger.FromContext(ctx)
	log.Info(
		"knowledge ingestion completed",
		"kb_id", kb.ID,
		"documents", result.Documents,
		"chunks", result.Chunks,
		"persisted", result.Persisted,
	)
	return &IngestOutput{Result: result}, nil
}
