package toolrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
)

// ToolDTO represents the typed API shape for a tool configuration.
// It mirrors public fields from tool.Config while keeping transport concerns
// (headers, envelopes) out of DTOs.
type ToolDTO struct{ ToolCoreDTO }

// ToolListItem is the list representation. Includes an optional strong ETag
// for clients that want to optimistically update items fetched from a list.
type ToolListItem struct {
	ToolCoreDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// ToolsListResponse is the typed list payload returned from GET /tools.
type ToolsListResponse struct {
	Tools []ToolListItem     `json:"tools"`
	Page  router.PageInfoDTO `json:"page"`
}

// ToToolDTOForWorkflow is an exported helper for workflow DTO expansion mapping.
func ToToolDTOForWorkflow(src map[string]any) ToolDTO { return toToolDTO(src) }

// ToolCoreDTO defines fields shared between single and list tool representations.
type ToolCoreDTO struct {
	Resource     string         `json:"resource,omitempty"    example:"tool"`
	ID           string         `json:"id"                    example:"http"`
	Description  string         `json:"description,omitempty" example:"HTTP client tool"`
	Timeout      string         `json:"timeout,omitempty"     example:"30s"`
	InputSchema  map[string]any `json:"input,omitempty"`
	OutputSchema map[string]any `json:"output,omitempty"`
	With         map[string]any `json:"with,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	Env          map[string]any `json:"env,omitempty"`
}

// toToolDTO maps a generic map payload (from UC.AsMap) to ToolDTO.
// Mapper is intentionally pure: do not import gin or depend on HTTP context.
func toToolDTO(src map[string]any) ToolDTO {
	return ToolDTO{ToolCoreDTO: ToolCoreDTO{
		Resource:     router.AsString(src["resource"]),
		ID:           router.AsString(src["id"]),
		Description:  router.AsString(src["description"]),
		Timeout:      router.AsString(src["timeout"]),
		InputSchema:  router.AsMap(src["input"]),
		OutputSchema: router.AsMap(src["output"]),
		With:         router.AsMap(src["with"]),
		Config:       router.AsMap(src["config"]),
		Env:          router.AsMap(src["env"]),
	}}
}

// toToolListItem maps a generic map payload to ToolListItem. If a UC provided
// an internal "_etag" field in the map, it is normalized to the public
// "etag" field on the DTO.
func toToolListItem(src map[string]any) ToolListItem {
	dto := ToolListItem{ToolCoreDTO: ToolCoreDTO{
		Resource:     router.AsString(src["resource"]),
		ID:           router.AsString(src["id"]),
		Description:  router.AsString(src["description"]),
		Timeout:      router.AsString(src["timeout"]),
		InputSchema:  router.AsMap(src["input"]),
		OutputSchema: router.AsMap(src["output"]),
		With:         router.AsMap(src["with"]),
		Config:       router.AsMap(src["config"]),
		Env:          router.AsMap(src["env"]),
	}, ETag: router.AsString(src["_etag"])}
	return dto
}

// helper functions centralized in router (AsString/AsMap)
