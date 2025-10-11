package uc

import (
	"context"
	"maps"
	"strings"

	"github.com/compozy/compozy/engine/resources"
)

type GetInput struct {
	Project string
	ID      string
}

type GetOutput struct {
	Schema map[string]any
	ETag   resources.ETag
}

type Get struct {
	store resources.ResourceStore
}

func NewGet(store resources.ResourceStore) *Get {
	return &Get{store: store}
}

func (uc *Get) Execute(ctx context.Context, in *GetInput) (*GetOutput, error) {
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
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceSchema, ID: schemaID}
	value, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if err == resources.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	sc, err := decodeStoredSchema(value, schemaID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any)
	maps.Copy(out, *sc)
	return &GetOutput{Schema: out, ETag: etag}, nil
}
