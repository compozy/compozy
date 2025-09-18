package uc

import (
	"context"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type GetInput struct {
	Project string
	Type    resources.ResourceType
	ID      string
}

type GetOutput struct {
	Value any
	ETag  string
}

type GetResource struct {
	store resources.ResourceStore
}

func NewGetResource(store resources.ResourceStore) *GetResource {
	return &GetResource{store: store}
}

func (uc *GetResource) Execute(ctx context.Context, in *GetInput) (*GetOutput, error) {
	_ = config.FromContext(ctx)
	_ = logger.FromContext(ctx)
	key := resources.ResourceKey{Project: in.Project, Type: in.Type, ID: in.ID}
	v, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if err == resources.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &GetOutput{Value: v, ETag: etag}, nil
}
