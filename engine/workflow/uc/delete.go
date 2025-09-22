package uc

import (
	"context"
	"strings"

	"github.com/compozy/compozy/engine/resources"
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
	if in == nil {
		return ErrInvalidInput
	}
	project := strings.TrimSpace(in.Project)
	if project == "" {
		return ErrProjectMissing
	}
	workflowID := strings.TrimSpace(in.ID)
	if workflowID == "" {
		return ErrInvalidInput
	}
	key := resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: workflowID}
	return uc.store.Delete(ctx, key)
}
