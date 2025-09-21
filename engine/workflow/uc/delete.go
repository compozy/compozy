package uc

import (
	"context"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
)

type DeleteInput struct {
	Project string
	ID      string
}

type Delete struct {
	store resources.ResourceStore
}

func NewDelete(store resources.ResourceStore) *Delete {
	return &Delete{store: store}
}

func (uc *Delete) Execute(ctx context.Context, in *DeleteInput) error {
	_ = config.FromContext(ctx)
	if in == nil || strings.TrimSpace(in.ID) == "" {
		return ErrInvalidInput
	}
	project := strings.TrimSpace(in.Project)
	if project == "" {
		return ErrProjectMissing
	}
	key := resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: strings.TrimSpace(in.ID)}
	return uc.store.Delete(ctx, key)
}
