package uc

import (
	"context"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resourceutil"
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
	if wfRefs, err := resourceutil.WorkflowsReferencingTask(ctx, uc.store, projectID, taskID); err != nil {
		return err
	} else if len(wfRefs) > 0 {
		return resourceutil.ConflictError{Details: []resourceutil.ReferenceDetail{{Resource: "workflows", IDs: wfRefs}}}
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceTask, ID: taskID}
	return uc.store.Delete(ctx, key)
}
