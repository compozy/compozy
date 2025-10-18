package uc

import (
	"context"
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
	if in == nil {
		return ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return ErrProjectMissing
	}
	taskID := strings.TrimSpace(in.ID)
	if taskID == "" {
		return ErrIDMissing
	}
	references := make([]resourceutil.ReferenceDetail, 0)
	if wfRefs, err := resourceutil.WorkflowsReferencingTask(ctx, uc.store, projectID, taskID); err != nil {
		return err
	} else if len(wfRefs) > 0 {
		references = append(references, resourceutil.ReferenceDetail{Resource: "workflows", IDs: wfRefs})
	}
	if taskRefs, err := resourceutil.TasksReferencingTaskResources(ctx, uc.store, projectID, taskID); err != nil {
		return err
	} else if len(taskRefs) > 0 {
		references = append(references, resourceutil.ReferenceDetail{Resource: "tasks", IDs: taskRefs})
	}
	if len(references) > 0 {
		return resourceutil.ConflictError{Details: references}
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceTask, ID: taskID}
	return uc.store.Delete(ctx, key)
}
