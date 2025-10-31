package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/tool"
)

func decodeToolBody(body map[string]any, pathID string) (*tool.Config, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	cfg := &tool.Config{}
	if err := cfg.FromMap(body); err != nil {
		return nil, fmt.Errorf("decode tool config: %w", err)
	}
	impl, err := cfg.EffectiveImplementation()
	if err != nil {
		return nil, err
	}
	if impl == tool.ImplementationNative {
		return nil, ErrNativeImplementation
	}
	cfg.SetImplementation(impl)
	return normalizeToolID(cfg, pathID)
}

func decodeStoredTool(value any, pathID string) (*tool.Config, error) {
	switch v := value.(type) {
	case *tool.Config:
		return normalizeToolID(v, pathID)
	case tool.Config:
		clone := v
		return normalizeToolID(&clone, pathID)
	case map[string]any:
		cfg := &tool.Config{}
		if err := cfg.FromMap(v); err != nil {
			return nil, fmt.Errorf("decode tool config: %w", err)
		}
		return normalizeToolID(cfg, pathID)
	default:
		return nil, ErrInvalidInput
	}
}

func normalizeToolID(cfg *tool.Config, pathID string) (*tool.Config, error) {
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
	impl, err := cfg.EffectiveImplementation()
	if err != nil {
		return nil, err
	}
	cfg.SetImplementation(impl)
	return cfg, nil
}
