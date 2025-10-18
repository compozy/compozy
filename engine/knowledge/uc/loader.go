package uc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/tplengine"
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
		if errors.Is(err, resources.ErrNotFound) {
			return nil, errors.Join(ErrNotFound, fmt.Errorf("load embedder %q: %w", kb.Embedder, err))
		}
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
		if errors.Is(err, resources.ErrNotFound) {
			return nil, errors.Join(ErrNotFound, fmt.Errorf("load vector_db %q: %w", kb.VectorDB, err))
		}
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
	if err := defs.Validate(ctx); err != nil {
		return nil, nil, nil, err
	}
	if err := renderKnowledgeDefinitions(ctx, &defs); err != nil {
		return nil, nil, nil, err
	}
	return &defs.KnowledgeBases[0], &defs.Embedders[0], &defs.VectorDBs[0], nil
}

func renderKnowledgeDefinitions(
	_ context.Context,
	defs *knowledge.Definitions,
) error {
	if defs == nil {
		return nil
	}
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	templateCtx := map[string]any{
		"env": captureEnvironment(),
	}
	for i := range defs.Embedders {
		current := defs.Embedders[i]
		resolved, err := renderKnowledgeValue(engine, templateCtx, current)
		if err != nil {
			return fmt.Errorf("knowledge: embedder %q template render failed: %w", current.ID, err)
		}
		defs.Embedders[i] = resolved
	}
	for i := range defs.VectorDBs {
		current := defs.VectorDBs[i]
		resolved, err := renderKnowledgeValue(engine, templateCtx, current)
		if err != nil {
			return fmt.Errorf("knowledge: vector_db %q template render failed: %w", current.ID, err)
		}
		defs.VectorDBs[i] = resolved
	}
	for i := range defs.KnowledgeBases {
		current := defs.KnowledgeBases[i]
		resolved, err := renderKnowledgeValue(engine, templateCtx, current)
		if err != nil {
			return fmt.Errorf("knowledge: knowledge_base %q template render failed: %w", current.ID, err)
		}
		defs.KnowledgeBases[i] = resolved
	}
	return nil
}

func renderKnowledgeValue[T any](
	engine *tplengine.TemplateEngine,
	templateCtx map[string]any,
	value T,
) (T, error) {
	if engine == nil {
		return value, nil
	}
	asMap, err := core.AsMapDefault(value)
	if err != nil {
		return value, err
	}
	parsed, err := engine.ParseAny(asMap, templateCtx)
	if err != nil {
		return value, err
	}
	return core.FromMapDefault[T](parsed)
}

func captureEnvironment() map[string]any {
	env := make(map[string]any)
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		env[parts[0]] = parts[1]
	}
	return env
}
