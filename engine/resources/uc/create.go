package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/userctx"
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
	log := logger.FromContext(ctx)
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
	updatedBy := "api"
	if u, ok := userctx.UserFromContext(ctx); ok && u != nil {
		updatedBy = u.ID.String()
	}
	if err := resources.WriteMeta(ctx, uc.store, in.Project, in.Type, id, "api", updatedBy); err != nil {
		log.Error("failed to write resource meta", "error", err, "type", string(in.Type), "id", id)
		return nil, fmt.Errorf("failed to write resource metadata: %w", err)
	}
	return &CreateOutput{Value: in.Body, ETag: etag, ID: id}, nil
}
