package uc

import (
	"context"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type CreateInput struct {
	Project string
	Type    resources.ResourceType
	Body    map[string]any
}

type CreateOutput struct {
	Value map[string]any
	ETag  string
	ID    string
}

type CreateResource struct {
	store resources.ResourceStore
}

func NewCreateResource(store resources.ResourceStore) *CreateResource {
	return &CreateResource{store: store}
}

func (uc *CreateResource) Execute(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
	_ = config.FromContext(ctx)
	_ = logger.FromContext(ctx)
	id, err := validateBody(in.Type, in.Body, "", true)
	if err != nil {
		return nil, err
	}
	if err := validateTypedResource(in.Type, in.Body); err != nil {
		return nil, err
	}
	key := resources.ResourceKey{Project: in.Project, Type: in.Type, ID: id}
	etag, err := uc.store.Put(ctx, key, in.Body)
	if err != nil {
		return nil, err
	}
	return &CreateOutput{Value: in.Body, ETag: etag, ID: id}, nil
}
