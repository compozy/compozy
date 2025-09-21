package uc

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

func decodeModelBody(body map[string]any) (*core.ProviderConfig, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	cfg, err := core.FromMapDefault[*core.ProviderConfig](body)
	if err != nil {
		return nil, fmt.Errorf("decode model config: %w", err)
	}
	return cfg, nil
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
