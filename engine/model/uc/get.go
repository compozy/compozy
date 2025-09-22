package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
)

type GetInput struct {
	Project string
	ID      string
}

type GetOutput struct {
	Model map[string]any
	ETag  resources.ETag
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
	modelID := strings.TrimSpace(in.ID)
	if modelID == "" {
		return nil, ErrIDMissing
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceModel, ID: modelID}
	value, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if err == resources.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get model %q in project %q: %w", modelID, projectID, err)
	}
	cfg, err := decodeStoredModel(value, modelID)
	if err != nil {
		return nil, fmt.Errorf("decode stored model %q: %w", modelID, err)
	}
	payload, err := core.AsMapDefault(cfg)
	if err != nil {
		return nil, fmt.Errorf("map model %q: %w", modelID, err)
	}
	return &GetOutput{Model: payload, ETag: etag}, nil
}
