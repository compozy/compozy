package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/task"
)

func decodeTaskBody(body map[string]any, pathID string) (*task.Config, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	cfg := &task.Config{}
	if err := cfg.FromMap(body); err != nil {
		return nil, fmt.Errorf("decode task config: %w", err)
	}
	return normalizeTaskID(cfg, pathID)
}

func decodeStoredTask(value any, pathID string) (*task.Config, error) {
	switch v := value.(type) {
	case *task.Config:
		return normalizeTaskID(v, pathID)
	case task.Config:
		clone := v
		return normalizeTaskID(&clone, pathID)
	case map[string]any:
		cfg := &task.Config{}
		if err := cfg.FromMap(v); err != nil {
			return nil, fmt.Errorf("decode task config: %w", err)
		}
		return normalizeTaskID(cfg, pathID)
	default:
		return nil, ErrInvalidInput
	}
}

func normalizeTaskID(cfg *task.Config, pathID string) (*task.Config, error) {
	if cfg == nil {
		return nil, ErrInvalidInput
	}
	id := strings.TrimSpace(pathID)
	if id == "" {
		return nil, ErrIDMissing
	}
	bodyID := strings.TrimSpace(cfg.ID)
	if bodyID != "" && bodyID != id {
		return nil, fmt.Errorf("%w: body=%s path=%s", ErrIDMismatch, bodyID, id)
	}
	cfg.ID = id
	return cfg, nil
}
