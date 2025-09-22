package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
)

func decodeMemoryBody(body map[string]any, pathID string) (*memory.Config, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	cfg := &memory.Config{}
	if err := cfg.FromMap(body); err != nil {
		return nil, fmt.Errorf("decode memory config: %w", err)
	}
	cfg.Resource = string(core.ConfigMemory)
	cfg.ID = pathID
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate memory config: %w", err)
	}
	return cfg, nil
}

func decodeStoredMemory(value any, pathID string) (*memory.Config, error) {
	switch v := value.(type) {
	case *memory.Config:
		return normalizeMemory(v, pathID)
	case memory.Config:
		asMap, err := v.AsMap()
		if err != nil {
			return nil, fmt.Errorf("convert memory config: %w", err)
		}
		cfg := &memory.Config{}
		if err := cfg.FromMap(asMap); err != nil {
			return nil, fmt.Errorf("decode memory config: %w", err)
		}
		return normalizeMemory(cfg, pathID)
	case map[string]any:
		cfg := &memory.Config{}
		if err := cfg.FromMap(v); err != nil {
			return nil, fmt.Errorf("decode memory config: %w", err)
		}
		return normalizeMemory(cfg, pathID)
	default:
		return nil, ErrInvalidInput
	}
}

func normalizeMemory(cfg *memory.Config, pathID string) (*memory.Config, error) {
	if cfg == nil {
		return nil, ErrInvalidInput
	}
	id := strings.TrimSpace(pathID)
	if id == "" {
		return nil, ErrIDMissing
	}
	cfg.ID = id
	cfg.Resource = string(core.ConfigMemory)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate memory config: %w", err)
	}
	return cfg, nil
}
