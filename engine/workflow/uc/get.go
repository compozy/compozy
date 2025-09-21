package uc

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
)

type GetInput struct {
	Project string
	ID      string
}

type GetOutput struct {
	Config *workflow.Config
	ETag   resources.ETag
}

type Get struct {
	store resources.ResourceStore
}

func NewGet(store resources.ResourceStore) *Get {
	return &Get{store: store}
}

func (uc *Get) Execute(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil || strings.TrimSpace(in.ID) == "" {
		return nil, ErrInvalidInput
	}
	project := strings.TrimSpace(in.Project)
	if project == "" {
		return nil, ErrProjectMissing
	}
	key := resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: strings.TrimSpace(in.ID)}
	value, etag, err := uc.store.Get(ctx, key)
	if errors.Is(err, resources.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	cfg, decodeErr := decodeStoredWorkflow(value, key.ID)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return &GetOutput{Config: cfg, ETag: etag}, nil
}
