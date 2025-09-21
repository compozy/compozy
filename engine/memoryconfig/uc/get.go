package uc

import (
	"context"
	"strings"

	"github.com/compozy/compozy/engine/resources"
)

type GetInput struct {
	Project string
	ID      string
}

type GetOutput struct {
	Memory map[string]any
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
	memoryID := strings.TrimSpace(in.ID)
	if memoryID == "" {
		return nil, ErrIDMissing
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceMemory, ID: memoryID}
	value, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if err == resources.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cfg, err := decodeStoredMemory(value, memoryID)
	if err != nil {
		return nil, err
	}
	entry, err := cfg.AsMap()
	if err != nil {
		return nil, err
	}
	return &GetOutput{Memory: entry, ETag: etag}, nil
}
