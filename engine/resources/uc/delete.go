package uc

import (
	"context"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type DeleteInput struct {
	Project string
	Type    resources.ResourceType
	ID      string
}

type DeleteResource struct {
	store resources.ResourceStore
}

func NewDeleteResource(store resources.ResourceStore) *DeleteResource {
	return &DeleteResource{store: store}
}

func (uc *DeleteResource) Execute(ctx context.Context, in *DeleteInput) error {
	_ = config.FromContext(ctx)
	_ = logger.FromContext(ctx)
	key := resources.ResourceKey{Project: in.Project, Type: in.Type, ID: in.ID}
	return uc.store.Delete(ctx, key)
}
