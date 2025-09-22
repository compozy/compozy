package uc

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
)

type GetInput struct {
	Project string
	ID      string
}

type GetOutput struct {
	Config *workflow.Config
	ETag   resources.ETag
}

type Get struct {
	store resources.ResourceStore
}

func NewGet(store resources.ResourceStore) *Get {
	return &Get{store: store}
}

func (uc *Get) Execute(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil {
		return nil, errors.Join(
			ErrInvalidInput,
			core.NewError(nil, "INVALID_INPUT", map[string]any{"reason": "input cannot be nil"}),
		)
	}
	workflowID := strings.TrimSpace(in.ID)
	if workflowID == "" {
		return nil, errors.Join(
			ErrInvalidInput,
			core.NewError(nil, "INVALID_INPUT", map[string]any{"reason": "workflow ID is required"}),
		)
	}
	project := strings.TrimSpace(in.Project)
	if project == "" {
		return nil, errors.Join(
			ErrProjectMissing,
			core.NewError(nil, "INVALID_INPUT", map[string]any{"reason": "project is required"}),
		)
	}
	key := resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: workflowID}
	value, etag, err := uc.store.Get(ctx, key)
	if errors.Is(err, resources.ErrNotFound) {
		return nil, errors.Join(ErrNotFound, core.NewError(nil, "NOT_FOUND", map[string]any{"workflow_id": workflowID}))
	}
	if err != nil {
		return nil, err
	}
	cfg, decodeErr := decodeStoredWorkflow(value, key.ID)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return &GetOutput{Config: cfg, ETag: etag}, nil
}
