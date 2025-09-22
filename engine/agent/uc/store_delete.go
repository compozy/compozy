package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
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
	agentID := strings.TrimSpace(in.ID)
	if agentID == "" {
		return ErrIDMissing
	}
	references := make([]resourceutil.ReferenceDetail, 0)
	if wfRefs, err := resourceutil.WorkflowsReferencingAgent(ctx, uc.store, projectID, agentID); err != nil {
		return err
	} else if len(wfRefs) > 0 {
		references = append(references, resourceutil.ReferenceDetail{Resource: "workflows", IDs: wfRefs})
	}
	if taskRefs, err := resourceutil.TasksReferencingAgentResources(ctx, uc.store, projectID, agentID); err != nil {
		return err
	} else if len(taskRefs) > 0 {
		references = append(references, resourceutil.ReferenceDetail{Resource: "tasks", IDs: taskRefs})
	}
	if len(references) > 0 {
		return resourceutil.ConflictError{Details: references}
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceAgent, ID: agentID}
	if err := uc.store.Delete(ctx, key); err != nil {
		if err == resources.ErrNotFound {
			return ErrNotFound
		}
		return fmt.Errorf("delete agent %q in project %q: %w", agentID, projectID, err)
	}
	return nil
}
