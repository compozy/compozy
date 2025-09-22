package uc

import (
	"context"
	"errors"
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
	mcpID := strings.TrimSpace(in.ID)
	if mcpID == "" {
		return ErrIDMissing
	}
	references := make([]resourceutil.ReferenceDetail, 0, 2)
	wfRefs, err := resourceutil.WorkflowsReferencingMCP(ctx, uc.store, projectID, mcpID)
	if err != nil {
		return fmt.Errorf("list workflows referencing mcp %q: %w", mcpID, err)
	}
	if len(wfRefs) > 0 {
		references = append(references, resourceutil.ReferenceDetail{Resource: "workflows", IDs: wfRefs})
	}
	agentRefs, err := resourceutil.AgentsReferencingMCP(ctx, uc.store, projectID, mcpID)
	if err != nil {
		return fmt.Errorf("list agents referencing mcp %q: %w", mcpID, err)
	}
	if len(agentRefs) > 0 {
		references = append(references, resourceutil.ReferenceDetail{Resource: "agents", IDs: agentRefs})
	}
	if len(references) > 0 {
		return resourceutil.ConflictError{Details: references}
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceMCP, ID: mcpID}
	if err := uc.store.Delete(ctx, key); err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete mcp %q: %w", mcpID, err)
	}
	return nil
}
