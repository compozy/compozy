package agentrouter

import (
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
)

// AgentCoreDTO defines stable, transport-facing fields for an agent.
type AgentCoreDTO struct {
	Resource     string         `json:"resource,omitempty"`
	ID           string         `json:"id"`
	Instructions string         `json:"instructions,omitempty"`
	Model        map[string]any `json:"model,omitempty"`
	With         map[string]any `json:"with,omitempty"`
	Env          map[string]any `json:"env,omitempty"`
}

// AgentDTO is the single-item representation.
type AgentDTO struct {
	AgentCoreDTO
}

// AgentListItem is the collection item; includes public ETag for optimistic updates.
type AgentListItem struct {
	AgentCoreDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// AgentsListResponse is the list envelope.
type AgentsListResponse struct {
	Agents []AgentListItem    `json:"agents"`
	Page   router.PageInfoDTO `json:"page"`
}

// ToAgentDTOForWorkflow is an exported helper for workflow DTO expansion mapping.
func ToAgentDTOForWorkflow(src map[string]any) AgentDTO {
	return toAgentDTO(src)
}

func agentCoreDTOFromMap(src map[string]any) AgentCoreDTO {
	return AgentCoreDTO{
		Resource:     router.AsString(src["resource"]),
		ID:           router.AsString(src["id"]),
		Instructions: router.AsString(src["instructions"]),
		Model:        router.AsMap(src["model"]),
		With:         router.AsMap(src["with"]),
		Env:          maskSecrets(router.AsMap(src["env"])),
	}
}

// toAgentDTO maps a generic UC map payload to AgentDTO.
func toAgentDTO(src map[string]any) AgentDTO {
	return AgentDTO{AgentCoreDTO: agentCoreDTOFromMap(src)}
}

// toAgentListItem maps a UC map payload to AgentListItem, normalizing _etag → etag.
func toAgentListItem(src map[string]any) AgentListItem {
	etag := router.AsString(src["_etag"])
	if etag == "" {
		etag = router.AsString(src["etag"])
	}
	return AgentListItem{AgentCoreDTO: agentCoreDTOFromMap(src), ETag: etag}
}

func maskSecrets(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	masked := make(map[string]any, len(values))
	for k, v := range values {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "secret") ||
			strings.Contains(lower, "token") ||
			strings.Contains(lower, "key") ||
			strings.Contains(lower, "password") {
			masked[k] = "********"
			continue
		}
		masked[k] = v
	}
	return masked
}
