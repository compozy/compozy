package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/configutil"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/retriever"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/logger"
)

type QueryInput struct {
	Project  string
	ID       string
	Query    string
	TopK     int
	MinScore *float64
	Filters  map[string]string
}

type QueryOutput struct {
	Contexts []knowledge.RetrievedContext
}

type Query struct {
	store resources.ResourceStore
}

func NewQuery(store resources.ResourceStore) *Query {
	return &Query{store: store}
}

func (uc *Query) Execute(ctx context.Context, in *QueryInput) (*QueryOutput, error) {
	projectID, kbID, queryText, err := validateQueryInput(in)
	if err != nil {
		return nil, err
	}
	binding, embAdapter, vecStore, release, err := uc.prepareQuery(ctx, projectID, kbID, in)
	if err != nil {
		return nil, err
	}
	defer func() {
		if release == nil {
			return
		}
		releaseCtx := context.WithoutCancel(ctx)
		if cerr := release(releaseCtx); cerr != nil {
			logger.FromContext(ctx).Warn(
				"failed to close vector store",
				"kb_id",
				binding.ID,
				"error",
				cerr,
			)
		}
	}()
	service, err := retriever.NewService(embAdapter, vecStore, nil)
	if err != nil {
		return nil, fmt.Errorf("init retriever: %w", err)
	}
	contexts, err := service.Retrieve(ctx, binding, queryText)
	if err != nil {
		return nil, err
	}
	return &QueryOutput{Contexts: contexts}, nil
}

func validateQueryInput(in *QueryInput) (string, string, string, error) {
	if in == nil {
		return "", "", "", ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return "", "", "", ErrProjectMissing
	}
	kbID := strings.TrimSpace(in.ID)
	if kbID == "" {
		return "", "", "", ErrIDMissing
	}
	queryText := strings.TrimSpace(in.Query)
	if queryText == "" {
		return "", "", "", fmt.Errorf("%w: query text is required", ErrInvalidInput)
	}
	return projectID, kbID, queryText, nil
}

func mergeRetrieval(
	base *knowledge.RetrievalConfig,
	topK int,
	minScore *float64,
	filters map[string]string,
) knowledge.RetrievalConfig {
	var merged knowledge.RetrievalConfig
	if base != nil {
		merged = *base
	}
	if topK > 0 {
		merged.TopK = topK
	}
	if minScore != nil {
		value := *minScore
		merged.MinScore = &value
	}
	if filters != nil {
		merged.Filters = core.CloneMap(filters)
	}
	return merged
}

func (uc *Query) prepareQuery(
	ctx context.Context,
	projectID string,
	kbID string,
	in *QueryInput,
) (*knowledge.ResolvedBinding, *embedder.Adapter, vectordb.Store, func(context.Context) error, error) {
	triple, err := loadKnowledgeTriple(ctx, uc.store, projectID, kbID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	kb, emb, vec, err := normalizeKnowledgeTriple(ctx, triple)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	retrieval := mergeRetrieval(&kb.Retrieval, in.TopK, in.MinScore, in.Filters)
	embCfg, err := configutil.ToEmbedderAdapterConfig(emb)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	embAdapter, err := embedder.New(ctx, embCfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("init embedder: %w", err)
	}
	if cacheCfg := emb.Config.Cache; cacheCfg != nil && cacheCfg.Enabled {
		if err := embAdapter.EnableCache(cacheCfg.Size); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("init embedder cache: %w", err)
		}
	}
	vecCfg, err := configutil.ToVectorStoreConfig(ctx, projectID, vec)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	vecStore, release, err := vectordb.AcquireShared(ctx, vecCfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("init vector store: %w", err)
	}
	binding := &knowledge.ResolvedBinding{
		ID:            kb.ID,
		KnowledgeBase: *kb,
		Embedder:      *emb,
		Vector:        *vec,
		Retrieval:     retrieval,
	}
	return binding, embAdapter, vecStore, release, nil
}
