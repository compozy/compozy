package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
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
	sc, err := decodeSchemaBody(ctx, in.Body, schemaID)
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
	entry := core.CloneMap(*sc)
	return &UpsertOutput{Schema: entry, ETag: etag, Created: created}, nil
}

func (uc *Upsert) storeSchema(
	ctx context.Context,
	key resources.ResourceKey,
	sc *schema.Schema,
	ifMatch string,
) (resources.ETag, bool, error) {
	etag, created, err := resources.ConditionalUpsert(ctx, uc.store, key, sc, ifMatch)
	switch {
	case errors.Is(err, resources.ErrETagMismatch):
		return "", false, ErrETagMismatch
	case errors.Is(err, resources.ErrNotFound) && strings.TrimSpace(ifMatch) != "":
		return "", false, ErrStaleIfMatch
	case err != nil:
		return "", false, fmt.Errorf("upsert schema: %w", err)
	}
	return etag, created, nil
}
