package modelrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
)

// ModelDTO represents the typed API shape for a model configuration.
// Mirrors public fields from core.ProviderConfig while keeping transport concerns
// out of DTOs.
type ModelDTO struct{ ModelCoreDTO }

// ModelListItem is the list representation with optional strong ETag for
// optimistic concurrency on items fetched from the list.
type ModelListItem struct {
	ModelCoreDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// ModelsListResponse is the typed list payload returned from GET /models.
type ModelsListResponse struct {
	Models []ModelListItem    `json:"models"`
	Page   router.PageInfoDTO `json:"page"`
}

// ModelCoreDTO defines fields shared between single and list model representations.
type ModelCoreDTO struct {
	Resource     string         `json:"resource,omitempty"`
	ID           string         `json:"id"`
	Provider     string         `json:"provider"`
	Model        string         `json:"model"`
	APIURL       string         `json:"api_url,omitempty"`
	Params       map[string]any `json:"params,omitempty"`
	Organization string         `json:"organization,omitempty"`
}

// toModelDTO maps a generic payload (from UC) to ModelDTO. Mapper remains pure.
func toModelDTO(src map[string]any) ModelDTO {
	provider := router.AsString(src["provider"])
	model := router.AsString(src["model"])
	id := buildModelID(provider, model, router.AsString(src["id"]))
	return ModelDTO{ModelCoreDTO: ModelCoreDTO{
		Resource:     "model",
		ID:           id,
		Provider:     provider,
		Model:        model,
		APIURL:       router.AsString(src["api_url"]),
		Params:       router.AsMap(src["params"]),
		Organization: router.AsString(src["organization"]),
	}}
}

// toModelListItem maps a generic payload to ModelListItem, normalizing _etag → etag.
func toModelListItem(src map[string]any) ModelListItem {
	provider := router.AsString(src["provider"])
	model := router.AsString(src["model"])
	id := buildModelID(provider, model, router.AsString(src["id"]))
	return ModelListItem{ModelCoreDTO: ModelCoreDTO{
		Resource:     "model",
		ID:           id,
		Provider:     provider,
		Model:        model,
		APIURL:       router.AsString(src["api_url"]),
		Params:       router.AsMap(src["params"]),
		Organization: router.AsString(src["organization"]),
	}, ETag: router.AsString(src["_etag"])}
}

func buildModelID(provider, model, fallback string) string {
	if provider != "" && model != "" {
		return provider + ":" + model
	}
	return fallback
}
