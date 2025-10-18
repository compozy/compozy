package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
)

func decodeMCPBody(ctx context.Context, body map[string]any, pathID string) (*mcp.Config, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	cfg, err := core.FromMapDefault[*mcp.Config](body)
	if err != nil {
		return nil, fmt.Errorf("decode mcp config: %w", err)
	}
	return normalizeMCPConfig(ctx, cfg, pathID)
}

func normalizeMCPConfig(ctx context.Context, cfg *mcp.Config, pathID string) (*mcp.Config, error) {
	if cfg == nil {
		return nil, ErrInvalidInput
	}
	id := strings.TrimSpace(pathID)
	if id == "" {
		return nil, ErrIDMissing
	}
	bodyID := strings.TrimSpace(cfg.ID)
	if bodyID != "" && bodyID != id {
		return nil, fmt.Errorf("id mismatch: body=%s path=%s", bodyID, id)
	}
	cfg.ID = id
	cfg.SetDefaults()
	if err := cfg.Validate(ctx); err != nil {
		return nil, fmt.Errorf("validate mcp config: %w", err)
	}
	return cfg, nil
}

func decodeStoredMCP(value any, id string) (*mcp.Config, error) {
	switch v := value.(type) {
	case *mcp.Config:
		if strings.TrimSpace(v.ID) == "" {
			v.ID = id
		}
		v.SetDefaults()
		return v, nil
	case mcp.Config:
		clone := v
		if strings.TrimSpace(clone.ID) == "" {
			clone.ID = id
		}
		clone.SetDefaults()
		return &clone, nil
	case map[string]any:
		cfg, err := core.FromMapDefault[*mcp.Config](v)
		if err != nil {
			return nil, fmt.Errorf("decode mcp config: %w", err)
		}
		if strings.TrimSpace(cfg.ID) == "" {
			cfg.ID = strings.TrimSpace(id)
		}
		cfg.SetDefaults()
		return cfg, nil
	default:
		return nil, ErrInvalidInput
	}
}
