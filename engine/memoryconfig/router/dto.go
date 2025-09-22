package memoryrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
)

// MemoryDTO represents the typed API shape for a memory configuration.
type MemoryDTO struct{ MemoryCoreDTO }

// MemoryListItem is the list representation with optional strong ETag.
type MemoryListItem struct {
	MemoryCoreDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// MemoriesListResponse is the typed list payload returned from GET /memories.
type MemoriesListResponse struct {
	Memories []MemoryListItem   `json:"memories"`
	Page     router.PageInfoDTO `json:"page"`
}

// MemoryCoreDTO defines fields shared between single and list representations.
type MemoryCoreDTO struct {
	Resource           string         `json:"resource,omitempty"`
	ID                 string         `json:"id"`
	Description        string         `json:"description,omitempty"`
	Type               string         `json:"type"`
	MaxTokens          *int           `json:"max_tokens,omitempty"`
	MaxMessages        *int           `json:"max_messages,omitempty"`
	MaxContextRatio    *float64       `json:"max_context_ratio,omitempty"`
	TokenAllocation    map[string]any `json:"token_allocation,omitempty"`
	Flushing           map[string]any `json:"flushing,omitempty"`
	Persistence        map[string]any `json:"persistence"`
	PrivacyPolicy      map[string]any `json:"privacy_policy,omitempty"`
	Locking            map[string]any `json:"locking,omitempty"`
	TokenProvider      map[string]any `json:"token_provider,omitempty"`
	DefaultKeyTemplate string         `json:"default_key_template,omitempty"`
}

// toMemoryDTO maps a generic UC map payload to MemoryDTO.
func toMemoryDTO(src map[string]any) MemoryDTO {
	return MemoryDTO{MemoryCoreDTO: MemoryCoreDTO{
		Resource:           router.AsString(src["resource"]),
		ID:                 router.AsString(src["id"]),
		Description:        router.AsString(src["description"]),
		Type:               router.AsString(src["type"]),
		MaxTokens:          intPtrFromAny(src["max_tokens"]),
		MaxMessages:        intPtrFromAny(src["max_messages"]),
		MaxContextRatio:    floatPtrFromAny(src["max_context_ratio"]),
		TokenAllocation:    router.AsMap(src["token_allocation"]),
		Flushing:           router.AsMap(src["flushing"]),
		Persistence:        router.AsMap(src["persistence"]),
		PrivacyPolicy:      router.AsMap(src["privacy_policy"]),
		Locking:            router.AsMap(src["locking"]),
		TokenProvider:      router.AsMap(src["token_provider"]),
		DefaultKeyTemplate: router.AsString(src["default_key_template"]),
	}}
}

// toMemoryListItem maps a UC map payload to MemoryListItem, normalizing _etag → etag.
func toMemoryListItem(src map[string]any) MemoryListItem {
	return MemoryListItem{MemoryCoreDTO: MemoryCoreDTO{
		Resource:           router.AsString(src["resource"]),
		ID:                 router.AsString(src["id"]),
		Description:        router.AsString(src["description"]),
		Type:               router.AsString(src["type"]),
		MaxTokens:          intPtrFromAny(src["max_tokens"]),
		MaxMessages:        intPtrFromAny(src["max_messages"]),
		MaxContextRatio:    floatPtrFromAny(src["max_context_ratio"]),
		TokenAllocation:    router.AsMap(src["token_allocation"]),
		Flushing:           router.AsMap(src["flushing"]),
		Persistence:        router.AsMap(src["persistence"]),
		PrivacyPolicy:      router.AsMap(src["privacy_policy"]),
		Locking:            router.AsMap(src["locking"]),
		TokenProvider:      router.AsMap(src["token_provider"]),
		DefaultKeyTemplate: router.AsString(src["default_key_template"]),
	}, ETag: router.AsString(src["_etag"])}
}

// helpers for numeric conversions from interface{}
func intPtrFromAny(v any) *int {
	switch t := v.(type) {
	case int:
		x := t
		return &x
	case int64:
		x := int(t)
		return &x
	case float64:
		x := int(t)
		return &x
	default:
		return nil
	}
}

func floatPtrFromAny(v any) *float64 {
	switch t := v.(type) {
	case float64:
		x := t
		return &x
	case int:
		x := float64(t)
		return &x
	case int64:
		x := float64(t)
		return &x
	default:
		return nil
	}
}
