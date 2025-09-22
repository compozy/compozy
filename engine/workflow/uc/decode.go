package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

func decodeWorkflowBody(body map[string]any, pathID string) (*workflow.Config, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	cfg, err := core.FromMapDefault[*workflow.Config](body)
	if err != nil {
		return nil, fmt.Errorf("decode workflow config: %w", err)
	}
	return normalizeWorkflowID(cfg, pathID)
}

func decodeStoredWorkflow(value any, pathID string) (*workflow.Config, error) {
	switch v := value.(type) {
	case *workflow.Config:
		return normalizeWorkflowID(v, pathID)
	case workflow.Config:
		cfg := v
		return normalizeWorkflowID(&cfg, pathID)
	case map[string]any:
		cfg, err := core.FromMapDefault[*workflow.Config](v)
		if err != nil {
			return nil, fmt.Errorf("decode workflow config: %w", err)
		}
		return normalizeWorkflowID(cfg, pathID)
	default:
		return nil, ErrInvalidInput
	}
}

func normalizeWorkflowID(cfg *workflow.Config, pathID string) (*workflow.Config, error) {
	if cfg == nil {
		return nil, ErrInvalidInput
	}
	bodyID := strings.TrimSpace(cfg.ID)
	routeID := strings.TrimSpace(pathID)
	if bodyID != "" && routeID != "" && bodyID != routeID {
		return nil, ErrIDMismatch
	}
	if bodyID == "" {
		cfg.ID = routeID
	}
	if strings.TrimSpace(cfg.ID) == "" {
		return nil, fmt.Errorf("workflow id is required: %w", ErrInvalidInput)
	}
	return cfg, nil
}
