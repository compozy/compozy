package uc

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
)

func decodeKnowledgeBase(body map[string]any, expectedID string) (*knowledge.BaseConfig, error) {
	cfg, err := core.FromMapDefault[knowledge.BaseConfig](body)
	if err != nil {
		return nil, fmt.Errorf("decode knowledge base: %w", err)
	}
	id := strings.TrimSpace(cfg.ID)
	expected := strings.TrimSpace(expectedID)
	if expected != "" {
		if id == "" {
			cfg.ID = expected
		} else if id != expected {
			return nil, ErrIDMismatch
		}
	}
	if strings.TrimSpace(cfg.ID) == "" {
		return nil, ErrIDMissing
	}
	return &cfg, nil
}

func decodeStoredKnowledgeBase(val any, id string) (*knowledge.BaseConfig, error) {
	switch typed := val.(type) {
	case *knowledge.BaseConfig:
		if typed == nil {
			return nil, fmt.Errorf("knowledge base %q: nil value", id)
		}
		clone := *typed
		return &clone, nil
	case knowledge.BaseConfig:
		clone := typed
		return &clone, nil
	case map[string]any:
		cfg, err := core.FromMapDefault[knowledge.BaseConfig](typed)
		if err != nil {
			return nil, fmt.Errorf("knowledge base %q: decode map: %w", id, err)
		}
		return &cfg, nil
	default:
		return nil, fmt.Errorf("knowledge base %q: unsupported type %T", id, val)
	}
}

func decodeStoredEmbedder(val any, id string) (*knowledge.EmbedderConfig, error) {
	switch typed := val.(type) {
	case *knowledge.EmbedderConfig:
		if typed == nil {
			return nil, fmt.Errorf("embedder %q: nil value", id)
		}
		clone := *typed
		return &clone, nil
	case knowledge.EmbedderConfig:
		clone := typed
		return &clone, nil
	case map[string]any:
		cfg, err := core.FromMapDefault[knowledge.EmbedderConfig](typed)
		if err != nil {
			return nil, fmt.Errorf("embedder %q: decode map: %w", id, err)
		}
		return &cfg, nil
	default:
		return nil, fmt.Errorf("embedder %q: unsupported type %T", id, val)
	}
}

func decodeStoredVectorDB(val any, id string) (*knowledge.VectorDBConfig, error) {
	switch typed := val.(type) {
	case *knowledge.VectorDBConfig:
		if typed == nil {
			return nil, fmt.Errorf("vector_db %q: nil value", id)
		}
		clone := *typed
		return &clone, nil
	case knowledge.VectorDBConfig:
		clone := typed
		return &clone, nil
	case map[string]any:
		cfg, err := core.FromMapDefault[knowledge.VectorDBConfig](typed)
		if err != nil {
			return nil, fmt.Errorf("vector_db %q: decode map: %w", id, err)
		}
		return &cfg, nil
	default:
		return nil, fmt.Errorf("vector_db %q: unsupported type %T", id, val)
	}
}
