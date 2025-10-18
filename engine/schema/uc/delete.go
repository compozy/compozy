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
	if in == nil {
		return ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return ErrProjectMissing
	}
	schemaID := strings.TrimSpace(in.ID)
	if schemaID == "" {
		return ErrIDMissing
	}
	referenceDetails, err := uc.collectReferences(ctx, projectID, schemaID)
	if err != nil {
		return err
	}
	if len(referenceDetails) > 0 {
		return resourceutil.ConflictError{Details: referenceDetails}
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceSchema, ID: schemaID}
	if err := uc.store.Delete(ctx, key); err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (uc *Delete) collectReferences(
	ctx context.Context,
	project string,
	schemaID string,
) ([]resourceutil.ReferenceDetail, error) {
	refs := make([]resourceutil.ReferenceDetail, 0)
	if workflowRefs, err := resourceutil.WorkflowsReferencingSchema(ctx, uc.store, project, schemaID); err != nil {
		return nil, err
	} else if len(workflowRefs) > 0 {
		refs = append(refs, resourceutil.ReferenceDetail{Resource: "workflows", IDs: workflowRefs})
	}
	if agentRefs, err := resourceutil.AgentsReferencingSchema(ctx, uc.store, project, schemaID); err != nil {
		return nil, err
	} else if len(agentRefs) > 0 {
		refs = append(refs, resourceutil.ReferenceDetail{Resource: "agents", IDs: agentRefs})
	}
	if toolRefs, err := resourceutil.ToolsReferencingSchema(ctx, uc.store, project, schemaID); err != nil {
		return nil, err
	} else if len(toolRefs) > 0 {
		refs = append(refs, resourceutil.ReferenceDetail{Resource: "tools", IDs: toolRefs})
	}
	if taskRefs, err := resourceutil.TasksReferencingSchema(ctx, uc.store, project, schemaID); err != nil {
		return nil, err
	} else if len(taskRefs) > 0 {
		refs = append(refs, resourceutil.ReferenceDetail{Resource: "tasks", IDs: taskRefs})
	}
	return refs, nil
}
