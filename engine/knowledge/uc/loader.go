package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
)

type knowledgeTriple struct {
	base     *knowledge.BaseConfig
	embedder *knowledge.EmbedderConfig
	vector   *knowledge.VectorDBConfig
	etag     resources.ETag
}

func loadKnowledgeTriple(
	ctx context.Context,
	store resources.ResourceStore,
	projectID string,
	kbID string,
) (*knowledgeTriple, error) {
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceKnowledgeBase, ID: kbID}
	val, etag, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load knowledge base %q: %w", kbID, err)
	}
	kb, err := decodeStoredKnowledgeBase(val, kbID)
	if err != nil {
		return nil, err
	}
	embKey := resources.ResourceKey{
		Project: projectID,
		Type:    resources.ResourceEmbedder,
		ID:      strings.TrimSpace(kb.Embedder),
	}
	embVal, _, err := store.Get(ctx, embKey)
	if err != nil {
		return nil, fmt.Errorf("load embedder %q: %w", kb.Embedder, err)
	}
	emb, err := decodeStoredEmbedder(embVal, kb.Embedder)
	if err != nil {
		return nil, err
	}
	vecKey := resources.ResourceKey{
		Project: projectID,
		Type:    resources.ResourceVectorDB,
		ID:      strings.TrimSpace(kb.VectorDB),
	}
	vecVal, _, err := store.Get(ctx, vecKey)
	if err != nil {
		return nil, fmt.Errorf("load vector_db %q: %w", kb.VectorDB, err)
	}
	vector, err := decodeStoredVectorDB(vecVal, kb.VectorDB)
	if err != nil {
		return nil, err
	}
	return &knowledgeTriple{base: kb, embedder: emb, vector: vector, etag: etag}, nil
}

func normalizeKnowledgeTriple(
	ctx context.Context,
	triple *knowledgeTriple,
) (*knowledge.BaseConfig, *knowledge.EmbedderConfig, *knowledge.VectorDBConfig, error) {
	if triple == nil {
		return nil, nil, nil, fmt.Errorf("knowledge components are required")
	}
	defs := knowledge.Definitions{
		Embedders:      []knowledge.EmbedderConfig{*triple.embedder},
		VectorDBs:      []knowledge.VectorDBConfig{*triple.vector},
		KnowledgeBases: []knowledge.BaseConfig{*triple.base},
	}
	defaults := knowledge.DefaultsFromContext(ctx)
	defs.NormalizeWithDefaults(defaults)
	if err := defs.Validate(); err != nil {
		return nil, nil, nil, err
	}
	return &defs.KnowledgeBases[0], &defs.Embedders[0], &defs.VectorDBs[0], nil
}
