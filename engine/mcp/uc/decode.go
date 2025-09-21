package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
)

func decodeMCPBody(body map[string]any, pathID string) (*mcp.Config, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	cfg, err := core.FromMapDefault[*mcp.Config](body)
	if err != nil {
		return nil, fmt.Errorf("decode mcp config: %w", err)
	}
	return normalizeMCPConfig(cfg, pathID)
}

func normalizeMCPConfig(cfg *mcp.Config, pathID string) (*mcp.Config, error) {
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
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate mcp config: %w", err)
	}
	return cfg, nil
}
