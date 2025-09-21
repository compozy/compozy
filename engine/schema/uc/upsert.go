package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type UpsertInput struct {
	Project string
	ID      string
	Body    map[string]any
	IfMatch string
}

type UpsertOutput struct {
	Schema  map[string]any
	ETag    resources.ETag
	Created bool
}

type Upsert struct {
	store resources.ResourceStore
}

func NewUpsert(store resources.ResourceStore) *Upsert {
	return &Upsert{store: store}
}

func (uc *Upsert) Execute(ctx context.Context, in *UpsertInput) (*UpsertOutput, error) {
	_ = config.FromContext(ctx)
	log := logger.FromContext(ctx)
	if in == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return nil, ErrProjectMissing
	}
	schemaID := strings.TrimSpace(in.ID)
	if schemaID == "" {
		return nil, ErrIDMissing
	}
	sc, err := decodeSchemaBody(in.Body, schemaID)
	if err != nil {
		return nil, err
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceSchema, ID: schemaID}
	etag, created, err := uc.storeSchema(ctx, key, sc, in.IfMatch)
	if err != nil {
		return nil, err
	}
	updatedBy := "api"
	if usr, ok := userctx.UserFromContext(ctx); ok && usr != nil {
		updatedBy = usr.ID.String()
	}
	if err := resources.WriteMeta(
		ctx,
		uc.store,
		projectID,
		resources.ResourceSchema,
		schemaID,
		"api",
		updatedBy,
	); err != nil {
		log.Error("failed to write schema meta", "error", err, "schema", schemaID)
		return nil, fmt.Errorf("write schema meta: %w", err)
	}
	entry := make(map[string]any)
	for k, v := range *sc {
		entry[k] = v
	}
	return &UpsertOutput{Schema: entry, ETag: etag, Created: created}, nil
}

func (uc *Upsert) storeSchema(
	ctx context.Context,
	key resources.ResourceKey,
	sc *schema.Schema,
	ifMatch string,
) (resources.ETag, bool, error) {
	trimmed := strings.TrimSpace(ifMatch)
	if trimmed != "" {
		etag, err := uc.store.PutIfMatch(ctx, key, sc, resources.ETag(trimmed))
		switch {
		case errors.Is(err, resources.ErrETagMismatch):
			return "", false, ErrETagMismatch
		case errors.Is(err, resources.ErrNotFound):
			return "", false, ErrStaleIfMatch
		case err != nil:
			return "", false, fmt.Errorf("upsert schema: %w", err)
		}
		return etag, false, nil
	}
	_, _, err := uc.store.Get(ctx, key)
	created := errors.Is(err, resources.ErrNotFound)
	if err != nil && !created {
		return "", false, fmt.Errorf("inspect schema: %w", err)
	}
	etag, putErr := uc.store.Put(ctx, key, sc)
	if putErr != nil {
		return "", false, fmt.Errorf("put schema: %w", putErr)
	}
	return etag, created, nil
}
