package uc

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
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
	if uc == nil || uc.store == nil {
		return ErrInvalidInput
	}
	if in == nil {
		return ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return ErrProjectMissing
	}
	toolID := strings.TrimSpace(in.ID)
	if toolID == "" {
		return ErrIDMissing
	}
	references := make([]resourceutil.ReferenceDetail, 0)
	wfRefs, err := resourceutil.WorkflowsReferencingTool(ctx, uc.store, projectID, toolID)
	if err != nil {
		return err
	}
	if len(wfRefs) > 0 {
		references = append(references, resourceutil.ReferenceDetail{Resource: "workflows", IDs: wfRefs})
	}
	taskRefs, err := resourceutil.TasksReferencingToolResources(ctx, uc.store, projectID, toolID)
	if err != nil {
		return err
	}
	if len(taskRefs) > 0 {
		references = append(references, resourceutil.ReferenceDetail{Resource: "tasks", IDs: taskRefs})
	}
	if len(references) > 0 {
		return resourceutil.ConflictError{Details: references}
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceTool, ID: toolID}
	if err := uc.store.Delete(ctx, key); err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
