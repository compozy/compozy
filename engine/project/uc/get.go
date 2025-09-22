package uc

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
)

type GetInput struct {
	Project string
}

type GetOutput struct {
	Config *project.Config
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
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceProject, ID: projectID}
	value, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cfg, decodeErr := decodeStoredProject(value, projectID)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return &GetOutput{Config: cfg, ETag: etag}, nil
}
