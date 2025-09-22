package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
)

func decodeAgentBody(body map[string]any, pathID string) (*agent.Config, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	cfg := &agent.Config{}
	if err := cfg.FromMap(body); err != nil {
		return nil, fmt.Errorf("decode agent config: %w", err)
	}
	if err := normalizeAgentID(cfg, pathID); err != nil {
		return nil, err
	}
	return cfg, nil
}

func decodeStoredAgent(value any, pathID string) (*agent.Config, error) {
	switch v := value.(type) {
	case *agent.Config:
		if err := normalizeAgentID(v, pathID); err != nil {
			return nil, err
		}
		return v, nil
	case agent.Config:
		clone := v
		if err := normalizeAgentID(&clone, pathID); err != nil {
			return nil, err
		}
		return &clone, nil
	case map[string]any:
		cfg := &agent.Config{}
		if err := cfg.FromMap(v); err != nil {
			return nil, fmt.Errorf("decode agent config: %w", err)
		}
		if err := normalizeAgentID(cfg, pathID); err != nil {
			return nil, err
		}
		return cfg, nil
	default:
		return nil, ErrInvalidInput
	}
}

func normalizeAgentID(cfg *agent.Config, pathID string) error {
	if cfg == nil {
		return ErrInvalidInput
	}
	id := strings.TrimSpace(pathID)
	if id == "" {
		return ErrIDMissing
	}
	bodyID := strings.TrimSpace(cfg.ID)
	if bodyID != "" && bodyID != id {
		return fmt.Errorf("id mismatch: body=%s path=%s", bodyID, id)
	}
	cfg.ID = id
	return nil
}
