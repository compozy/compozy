package uc

import (
	"context"
	"fmt"

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
	if in == nil {
		return fmt.Errorf("delete resource: nil input")
	}
	if in.Project == "" || in.ID == "" {
		return fmt.Errorf("delete resource: project and id are required")
	}
	key := resources.ResourceKey{Project: in.Project, Type: in.Type, ID: in.ID}
	if err := uc.store.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete %s/%s: %w", string(in.Type), in.ID, err)
	}
	return nil
}
