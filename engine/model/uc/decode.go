package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
)

func decodeModelBody(body map[string]any, pathID string) (*core.ProviderConfig, string, error) {
	if body == nil {
		return nil, "", ErrInvalidInput
	}
	cfg, err := core.FromMapDefault[*core.ProviderConfig](body)
	if err != nil {
		return nil, "", fmt.Errorf("decode model config: %w", err)
	}
	trimmedPath := strings.TrimSpace(pathID)
	provider := strings.TrimSpace(string(cfg.Provider))
	model := strings.TrimSpace(cfg.Model)
	if provider == "" || model == "" {
		if trimmedPath == "" {
			return nil, "", ErrIDMissing
		}
		return cfg, trimmedPath, nil
	}
	colonID := provider + ":" + model
	hyphenID := provider + "-" + model
	if trimmedPath != "" {
		if trimmedPath != colonID && trimmedPath != hyphenID {
			return nil, "", ErrIDMismatch
		}
		return cfg, trimmedPath, nil
	}
	return cfg, colonID, nil
}

func decodeStoredModel(value any, _ string) (*core.ProviderConfig, error) {
	switch v := value.(type) {
	case *core.ProviderConfig:
		return v, nil
	case core.ProviderConfig:
		clone := v
		return &clone, nil
	case map[string]any:
		cfg, err := core.FromMapDefault[*core.ProviderConfig](v)
		if err != nil {
			return nil, fmt.Errorf("decode model config: %w", err)
		}
		return cfg, nil
	default:
		return nil, ErrInvalidInput
	}
}
