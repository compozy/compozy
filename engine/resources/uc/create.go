package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const apiMetaSource = "api"

type CreateInput struct {
	Project string
	Type    resources.ResourceType
	Body    map[string]any
}

type CreateOutput struct {
	Value map[string]any
	ETag  resources.ETag
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
	if in == nil {
		return nil, fmt.Errorf("create resource: nil input")
	}
	if in.Project == "" {
		return nil, fmt.Errorf("create resource: project is required")
	}
	if in.Type == "" {
		return nil, fmt.Errorf("create resource: type is required")
	}
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
		return nil, fmt.Errorf("store put %s/%s: %w", string(in.Type), id, err)
	}
	updatedBy := apiMetaSource
	if u, ok := userctx.UserFromContext(ctx); ok && u != nil {
		updatedBy = u.ID.String()
	}
	if err := resources.WriteMeta(ctx, uc.store, in.Project, in.Type, id, apiMetaSource, updatedBy); err != nil {
		log.Error("failed to write resource meta", "error", err, "type", string(in.Type), "id", id)
		if delErr := uc.store.Delete(ctx, key); delErr != nil {
			log.Error("rollback failed after meta write error", "error", delErr, "type", string(in.Type), "id", id)
		}
		return nil, fmt.Errorf("failed to write resource metadata: %w", err)
	}
	return &CreateOutput{Value: in.Body, ETag: etag, ID: id}, nil
}
