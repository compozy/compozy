package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
)

type GetInput struct {
	Project string
	ID      string
}

type GetOutput struct {
	KnowledgeBase *knowledge.BaseConfig
	ETag          resources.ETag
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
	kbID := strings.TrimSpace(in.ID)
	if kbID == "" {
		return nil, ErrIDMissing
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceKnowledgeBase, ID: kbID}
	val, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get knowledge base %q: %w", kbID, err)
	}
	cfg, decErr := decodeStoredKnowledgeBase(val, kbID)
	if decErr != nil {
		return nil, decErr
	}
	return &GetOutput{KnowledgeBase: cfg, ETag: etag}, nil
}
