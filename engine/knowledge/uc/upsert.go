package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/logger"
)

const sourceAPI = "api"

type UpsertInput struct {
	Project string
	ID      string
	Body    map[string]any
	IfMatch string
}

type UpsertOutput struct {
	KnowledgeBase map[string]any
	ETag          resources.ETag
	Created       bool
}

type Upsert struct {
	store resources.ResourceStore
}

func validateUpsertInput(in *UpsertInput) (string, string, error) {
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

func NewUpsert(store resources.ResourceStore) *Upsert {
	return &Upsert{store: store}
}

func (uc *Upsert) normalizeConfig(
	ctx context.Context,
	projectID string,
	kbID string,
	body map[string]any,
) (*knowledge.BaseConfig, error) {
	cfg, err := decodeKnowledgeBase(body, kbID)
	if err != nil {
		return nil, err
	}
	emb, err := uc.loadEmbedderConfig(ctx, projectID, cfg.Embedder)
	if err != nil {
		return nil, err
	}
	vector, err := uc.loadVectorDBConfig(ctx, projectID, cfg.VectorDB)
	if err != nil {
		return nil, err
	}
	return normalizeKnowledgeDefinitions(ctx, cfg, emb, vector)
}

func (uc *Upsert) Execute(ctx context.Context, in *UpsertInput) (*UpsertOutput, error) {
	projectID, kbID, err := validateUpsertInput(in)
	if err != nil {
		return nil, err
	}
	normalized, err := uc.normalizeConfig(ctx, projectID, kbID, in.Body)
	if err != nil {
		return nil, err
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceKnowledgeBase, ID: normalized.ID}
	etag, created, err := uc.storeKnowledgeBase(ctx, key, normalized, strings.TrimSpace(in.IfMatch))
	if err != nil {
		return nil, err
	}
	updatedBy := sourceAPI
	if usr, ok := userctx.UserFromContext(ctx); ok && usr != nil {
		updatedBy = usr.ID.String()
	}
	if err := resources.WriteMeta(
		ctx,
		uc.store,
		projectID,
		resources.ResourceKnowledgeBase,
		normalized.ID,
		sourceAPI,
		updatedBy,
	); err != nil {
		logger.FromContext(ctx).Error("failed to write knowledge base meta", "error", err, "kb_id", normalized.ID)
	}
	entry, err := core.AsMapDefault(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode knowledge base: %w", err)
	}
	return &UpsertOutput{KnowledgeBase: entry, ETag: etag, Created: created}, nil
}

func (uc *Upsert) loadEmbedderConfig(
	ctx context.Context,
	projectID string,
	embedderID string,
) (*knowledge.EmbedderConfig, error) {
	trimmed := strings.TrimSpace(embedderID)
	key := resources.ResourceKey{
		Project: projectID,
		Type:    resources.ResourceEmbedder,
		ID:      trimmed,
	}
	val, _, err := uc.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, fmt.Errorf("%w: unknown embedder %q", ErrValidationFail, embedderID)
		}
		return nil, fmt.Errorf("load embedder %q: %w", embedderID, err)
	}
	return decodeStoredEmbedder(val, trimmed)
}

func (uc *Upsert) loadVectorDBConfig(
	ctx context.Context,
	projectID string,
	vectorID string,
) (*knowledge.VectorDBConfig, error) {
	trimmed := strings.TrimSpace(vectorID)
	key := resources.ResourceKey{
		Project: projectID,
		Type:    resources.ResourceVectorDB,
		ID:      trimmed,
	}
	val, _, err := uc.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, fmt.Errorf("%w: unknown vector_db %q", ErrValidationFail, vectorID)
		}
		return nil, fmt.Errorf("load vector_db %q: %w", vectorID, err)
	}
	return decodeStoredVectorDB(val, trimmed)
}

func normalizeKnowledgeDefinitions(
	ctx context.Context,
	cfg *knowledge.BaseConfig,
	emb *knowledge.EmbedderConfig,
	vector *knowledge.VectorDBConfig,
) (*knowledge.BaseConfig, error) {
	defs := knowledge.Definitions{
		Embedders:      []knowledge.EmbedderConfig{*emb},
		VectorDBs:      []knowledge.VectorDBConfig{*vector},
		KnowledgeBases: []knowledge.BaseConfig{*cfg},
	}
	defs.NormalizeWithDefaults(knowledge.DefaultsFromContext(ctx))
	if err := defs.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidationFail, err)
	}
	normalized := defs.KnowledgeBases[0]
	return &normalized, nil
}

func (uc *Upsert) storeKnowledgeBase(
	ctx context.Context,
	key resources.ResourceKey,
	cfg *knowledge.BaseConfig,
	ifMatch string,
) (resources.ETag, bool, error) {
	trimmed := strings.TrimSpace(ifMatch)
	if trimmed != "" {
		etag, err := uc.store.PutIfMatch(ctx, key, cfg, resources.ETag(trimmed))
		switch {
		case errors.Is(err, resources.ErrETagMismatch):
			return "", false, ErrETagMismatch
		case errors.Is(err, resources.ErrNotFound):
			return "", false, ErrStaleIfMatch
		case err != nil:
			return "", false, fmt.Errorf("upsert knowledge base: %w", err)
		}
		return etag, false, nil
	}
	_, _, err := uc.store.Get(ctx, key)
	if errors.Is(err, resources.ErrNotFound) {
		etag, putErr := uc.store.PutIfMatch(ctx, key, cfg, "")
		switch {
		case putErr == nil:
			return etag, true, nil
		case errors.Is(putErr, resources.ErrETagMismatch):
			return "", false, ErrAlreadyExists
		case errors.Is(putErr, resources.ErrNotFound):
			fallbackETag, fallbackErr := uc.store.Put(ctx, key, cfg)
			if fallbackErr != nil {
				return "", false, fmt.Errorf("create knowledge base: %w", fallbackErr)
			}
			return fallbackETag, true, nil
		default:
			return "", false, fmt.Errorf("create knowledge base: %w", putErr)
		}
	}
	if err != nil {
		return "", false, fmt.Errorf("inspect knowledge base: %w", err)
	}
	etag, putErr := uc.store.Put(ctx, key, cfg)
	if putErr != nil {
		return "", false, fmt.Errorf("put knowledge base: %w", putErr)
	}
	return etag, false, nil
}
