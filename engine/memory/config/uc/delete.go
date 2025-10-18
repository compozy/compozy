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
	memoryID := strings.TrimSpace(in.ID)
	if memoryID == "" {
		return ErrIDMissing
	}
	refs, err := resourceutil.AgentsReferencingMemory(ctx, uc.store, projectID, memoryID)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		return resourceutil.ConflictError{Details: []resourceutil.ReferenceDetail{{Resource: "agents", IDs: refs}}}
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceMemory, ID: memoryID}
	return uc.store.Delete(ctx, key)
}
