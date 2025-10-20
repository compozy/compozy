package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
)

type GetInput struct {
	Project string
	ID      string
}

type GetOutput struct {
	MCP  map[string]any
	ETag resources.ETag
}

type Get struct {
	store resources.ResourceStore
}

func NewGet(store resources.ResourceStore) *Get {
	return &Get{store: store}
}

func (uc *Get) Execute(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(in.Project)
	if projectID == "" {
		return nil, ErrProjectMissing
	}
	mcpID := strings.TrimSpace(in.ID)
	if mcpID == "" {
		return nil, ErrIDMissing
	}
	key := resources.ResourceKey{Project: projectID, Type: resources.ResourceMCP, ID: mcpID}
	value, etag, err := uc.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get mcp %q in project %q: %w", mcpID, projectID, err)
	}
	cfg, err := decodeStoredMCP(ctx, value, mcpID)
	if err != nil {
		return nil, fmt.Errorf("decode stored mcp %q: %w", mcpID, err)
	}
	payload, err := core.AsMapDefault(cfg)
	if err != nil {
		return nil, fmt.Errorf("map mcp %q: %w", mcpID, err)
	}
	return &GetOutput{MCP: payload, ETag: etag}, nil
}
