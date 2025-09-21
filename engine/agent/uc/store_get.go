package uc

import (
	"context"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
)

type GetInput struct {
	Project string
	ID      string
}

type GetOutput struct {
	Agent map[string]any
	ETag  resources.ETag
}

type Get struct {
	store resources.ResourceStore
}

func NewGet(store resources.ResourceStore) *Get {
	return &Get{store: store}
}

func (uc *Get) Execute(ctx context.Context, in *GetInput) (*GetOutput, error) {
	_ = config.FromContext(ctx)
	if in == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return nil, ErrProjectMissing
	}
	agentID := strings.TrimSpace(in.ID)
	if agentID == "" {
		return nil, ErrIDMissing
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceAgent, ID: agentID}
	value, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if err == resources.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cfg, err := decodeStoredAgent(value, agentID)
	if err != nil {
		return nil, err
	}
	entry, err := cfg.AsMap()
	if err != nil {
		return nil, err
	}
	return &GetOutput{Agent: entry, ETag: etag}, nil
}
