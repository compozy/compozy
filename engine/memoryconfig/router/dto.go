package memoryrouter

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	memory "github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// MemoryDTO represents the typed API shape for a memory configuration.
type MemoryDTO struct {
	MemoryCoreDTO
}

// MemoryListItem is the list representation with optional strong ETag.
type MemoryListItem struct {
	MemoryCoreDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// MemoriesListResponse is the typed list payload returned from GET /memories.
type MemoriesListResponse struct {
	Memories []MemoryListItem    `json:"memories"`
	Page     httpdto.PageInfoDTO `json:"page"`
}

// MemoryCoreDTO defines fields shared between single and list representations.
type MemoryCoreDTO struct {
	Resource           string                          `json:"resource,omitempty"`
	ID                 string                          `json:"id"`
	Description        string                          `json:"description,omitempty"`
	Version            string                          `json:"version,omitempty"`
	Type               memcore.Type                    `json:"type"`
	MaxTokens          int                             `json:"max_tokens,omitempty"`
	MaxMessages        int                             `json:"max_messages,omitempty"`
	MaxContextRatio    float64                         `json:"max_context_ratio,omitempty"`
	TokenAllocation    *memcore.TokenAllocation        `json:"token_allocation,omitempty"`
	Flushing           *memcore.FlushingStrategyConfig `json:"flushing,omitempty"`
	Persistence        memcore.PersistenceConfig       `json:"persistence"`
	PrivacyPolicy      *memcore.PrivacyPolicyConfig    `json:"privacy_policy,omitempty"`
	Locking            *memcore.LockConfig             `json:"locking,omitempty"`
	TokenProvider      *memcore.TokenProviderConfig    `json:"token_provider,omitempty"`
	DefaultKeyTemplate string                          `json:"default_key_template,omitempty"`
}

// toMemoryDTO maps a generic UC map payload to MemoryDTO.
func toMemoryDTO(src map[string]any) (MemoryDTO, error) {
	cfg, err := mapToMemoryConfig(src)
	if err != nil {
		return MemoryDTO{}, err
	}
	coreDTO, err := convertMemoryConfigToDTO(cfg)
	if err != nil {
		return MemoryDTO{}, err
	}
	return MemoryDTO{MemoryCoreDTO: coreDTO}, nil
}

// toMemoryListItem maps a UC map payload to MemoryListItem, normalizing _etag → etag.
func toMemoryListItem(src map[string]any) (MemoryListItem, error) {
	dto, err := toMemoryDTO(src)
	if err != nil {
		return MemoryListItem{}, err
	}
	return MemoryListItem{MemoryCoreDTO: dto.MemoryCoreDTO, ETag: httpdto.AsString(src["_etag"])}, nil
}

func mapToMemoryConfig(src map[string]any) (*memory.Config, error) {
	if src == nil {
		return nil, fmt.Errorf("memory payload is nil")
	}
	cfg, err := core.FromMapDefault[*memory.Config](src)
	if err != nil {
		return nil, fmt.Errorf("map to memory config: %w", err)
	}
	return cfg, nil
}

func convertMemoryConfigToDTO(cfg *memory.Config) (MemoryCoreDTO, error) {
	if cfg == nil {
		return MemoryCoreDTO{}, fmt.Errorf("memory config is nil")
	}
	return MemoryCoreDTO{
		Resource:           cfg.Resource,
		ID:                 cfg.ID,
		Description:        cfg.Description,
		Version:            cfg.Version,
		Type:               cfg.Type,
		MaxTokens:          cfg.MaxTokens,
		MaxMessages:        cfg.MaxMessages,
		MaxContextRatio:    cfg.MaxContextRatio,
		TokenAllocation:    cfg.TokenAllocation,
		Flushing:           cfg.Flushing,
		Persistence:        cfg.Persistence,
		PrivacyPolicy:      cfg.PrivacyPolicy,
		Locking:            cfg.Locking,
		TokenProvider:      cfg.TokenProvider,
		DefaultKeyTemplate: cfg.DefaultKeyTemplate,
	}, nil
}
