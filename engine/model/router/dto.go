package modelrouter

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
)

const resourceModel = "model"

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
	Resource          string            `json:"resource,omitempty"`
	ID                string            `json:"id"`
	Provider          string            `json:"provider"`
	Model             string            `json:"model"`
	APIKey            string            `json:"api_key,omitempty"`
	APIURL            string            `json:"api_url,omitempty"`
	Params            core.PromptParams `json:"params,omitempty"`
	Organization      string            `json:"organization,omitempty"`
	Default           bool              `json:"default,omitempty"`
	MaxToolIterations int               `json:"max_tool_iterations,omitempty"`
}

// toModelDTO maps a generic payload (from UC) to ModelDTO. Mapper remains pure.
func toModelDTO(src map[string]any) (ModelDTO, error) {
	cfg, err := mapToProviderConfig(src)
	if err != nil {
		return ModelDTO{}, err
	}
	fallbackID := router.AsString(src["id"])
	coreDTO, err := convertProviderConfigToDTO(cfg, fallbackID)
	if err != nil {
		return ModelDTO{}, err
	}
	return ModelDTO{ModelCoreDTO: coreDTO}, nil
}

// toModelListItem maps a generic payload to ModelListItem, normalizing _etag → etag.
func toModelListItem(src map[string]any) (ModelListItem, error) {
	dto, err := toModelDTO(src)
	if err != nil {
		return ModelListItem{}, err
	}
	return ModelListItem{ModelCoreDTO: dto.ModelCoreDTO, ETag: router.AsString(src["_etag"])}, nil
}

func buildModelID(provider, model, fallback string) string {
	if provider != "" && model != "" {
		return provider + ":" + model
	}
	return fallback
}

func mapToProviderConfig(src map[string]any) (*core.ProviderConfig, error) {
	if src == nil {
		return nil, fmt.Errorf("model payload is nil")
	}
	cfg, err := core.FromMapDefault[*core.ProviderConfig](src)
	if err != nil {
		return nil, fmt.Errorf("map to provider config: %w", err)
	}
	return cfg, nil
}

func convertProviderConfigToDTO(cfg *core.ProviderConfig, fallbackID string) (ModelCoreDTO, error) {
	if cfg == nil {
		return ModelCoreDTO{}, fmt.Errorf("provider config is nil")
	}
	clone, err := core.DeepCopy[*core.ProviderConfig](cfg)
	if err != nil {
		return ModelCoreDTO{}, fmt.Errorf("deep copy provider config: %w", err)
	}
	provider := string(clone.Provider)
	model := clone.Model
	id := buildModelID(provider, model, fallbackID)
	return ModelCoreDTO{
		Resource:          resourceModel,
		ID:                id,
		Provider:          provider,
		Model:             model,
		APIKey:            clone.APIKey,
		APIURL:            clone.APIURL,
		Params:            clone.Params,
		Organization:      clone.Organization,
		Default:           clone.Default,
		MaxToolIterations: clone.MaxToolIterations,
	}, nil
}
