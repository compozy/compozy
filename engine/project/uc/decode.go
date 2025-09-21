package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
)

func decodeStoredProject(value any, expectedName string) (*project.Config, error) {
	switch v := value.(type) {
	case *project.Config:
		return normalizeProjectName(v, expectedName)
	case project.Config:
		clone := v
		return normalizeProjectName(&clone, expectedName)
	case map[string]any:
		cfg, err := core.FromMapDefault[*project.Config](v)
		if err != nil {
			return nil, fmt.Errorf("decode project config: %w", err)
		}
		return normalizeProjectName(cfg, expectedName)
	default:
		return nil, ErrInvalidInput
	}
}

func normalizeProjectName(cfg *project.Config, expected string) (*project.Config, error) {
	if cfg == nil {
		return nil, ErrInvalidInput
	}
	name := strings.TrimSpace(cfg.Name)
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil, ErrProjectMissing
	}
	if name == "" {
		cfg.Name = expected
		return cfg, nil
	}
	if name != expected {
		return nil, ErrNameMismatch
	}
	return cfg, nil
}
