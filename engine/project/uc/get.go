package uc

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
)

// GetInput carries parameters required to retrieve the project configuration.
type GetInput struct {
	Project string
}

// GetOutput contains the decoded project configuration and its current ETag.
type GetOutput struct {
	Config *project.Config
	ETag   resources.ETag
}

// Get loads the singleton project configuration from the resource store.
type Get struct {
	store resources.ResourceStore
}

// NewGet constructs a Get use case bound to the provided resource store.
func NewGet(store resources.ResourceStore) *Get {
	return &Get{store: store}
}

// Execute fetches the project configuration referenced by the input project ID.
func (uc *Get) Execute(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return nil, ErrProjectMissing
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceProject, ID: projectID}
	value, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cfg, decodeErr := decodeStoredProject(value, projectID)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return &GetOutput{Config: cfg, ETag: etag}, nil
}
