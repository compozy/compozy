package uc

import (
	"context"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type UpsertInput struct {
	Project string
	Type    resources.ResourceType
	ID      string
	Body    map[string]any
	IfMatch string
}

type UpsertOutput struct {
	Value map[string]any
	ETag  string
}

type UpsertResource struct {
	store resources.ResourceStore
}

func NewUpsertResource(store resources.ResourceStore) *UpsertResource {
	return &UpsertResource{store: store}
}

func (uc *UpsertResource) Execute(ctx context.Context, in *UpsertInput) (*UpsertOutput, error) {
	_ = config.FromContext(ctx)
	_ = logger.FromContext(ctx)
	if _, err := validateBody(in.Type, in.Body, in.ID, false); err != nil {
		return nil, err
	}
	if err := validateTypedResource(in.Type, in.Body); err != nil {
		return nil, err
	}
	key := resources.ResourceKey{Project: in.Project, Type: in.Type, ID: in.ID}
	if in.IfMatch != "" {
		_, cur, err := uc.store.Get(ctx, key)
		if err != nil {
			return nil, ErrIfMatchStaleOrMissing
		}
		if cur != in.IfMatch {
			return nil, ErrETagMismatch
		}
	}
	etag, err := uc.store.Put(ctx, key, in.Body)
	if err != nil {
		return nil, err
	}
	return &UpsertOutput{Value: in.Body, ETag: etag}, nil
}
