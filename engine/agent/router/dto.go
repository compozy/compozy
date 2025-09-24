package agentrouter

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	tool "github.com/compozy/compozy/engine/tool"
)

const redaction = "********"

var sensitiveEnvKeySubstrings = []string{
	"secret",
	"token",
	"password",
	"passwd",
	"pwd",
	"apikey",
	"api_key",
	"private",
	"credential",
	"bearer",
}

func isSensitiveEnvKey(k string) bool {
	lower := strings.ToLower(k)
	for i := range sensitiveEnvKeySubstrings {
		if strings.Contains(lower, sensitiveEnvKeySubstrings[i]) {
			return true
		}
	}
	return false
}

// AgentModelDTO exposes the configured provider reference without leaking inline configuration.
type AgentModelDTO struct {
	Ref    string               `json:"ref,omitempty"`
	Config *core.ProviderConfig `json:"config,omitempty"`
}

// AgentActionDTO mirrors agent.ActionConfig for transport.
type AgentActionDTO struct {
	ID           string                 `json:"id"`
	Prompt       string                 `json:"prompt"`
	InputSchema  *schema.Schema         `json:"input,omitempty"`
	OutputSchema *schema.Schema         `json:"output,omitempty"`
	With         *core.Input            `json:"with,omitempty"`
	JSONMode     bool                   `json:"json_mode"`
	Attachments  attachment.Attachments `json:"attachments,omitempty"`
}

// AgentDTO is the canonical typed representation for agents.
type AgentDTO struct {
	Resource      string                 `json:"resource,omitempty"`
	ID            string                 `json:"id"`
	Instructions  string                 `json:"instructions,omitempty"`
	Model         AgentModelDTO          `json:"model"`
	MaxIterations int                    `json:"max_iterations,omitempty"`
	JSONMode      bool                   `json:"json_mode"`
	Actions       []AgentActionDTO       `json:"actions,omitempty"`
	With          *core.Input            `json:"with,omitempty"`
	Env           map[string]string      `json:"env,omitempty"`
	Tools         []tool.Config          `json:"tools,omitempty"`
	MCPs          []mcp.Config           `json:"mcps,omitempty"`
	Memory        []core.MemoryReference `json:"memory,omitempty"`
	Attachments   attachment.Attachments `json:"attachments,omitempty"`
}

// AgentListItem is the list representation and carries the item ETag.
type AgentListItem struct {
	AgentDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// AgentsListResponse is the typed list payload returned from GET /agents.
type AgentsListResponse struct {
	Agents []AgentListItem     `json:"agents"`
	Page   httpdto.PageInfoDTO `json:"page"`
}

// ToAgentDTOForWorkflow converts UC map payloads into typed DTOs for workflow expansion.
func ToAgentDTOForWorkflow(src map[string]any) (AgentDTO, error) {
	return toAgentDTO(src)
}

// ConvertAgentConfigToDTO converts an agent.Config to AgentDTO using deep-copy semantics.
func ConvertAgentConfigToDTO(cfg *agent.Config) (AgentDTO, error) {
	if cfg == nil {
		return AgentDTO{}, fmt.Errorf("agent config is nil")
	}
	clone, err := core.DeepCopy[*agent.Config](cfg)
	if err != nil {
		return AgentDTO{}, fmt.Errorf("deep copy agent config: %w", err)
	}
	dto := AgentDTO{
		Resource:      clone.Resource,
		ID:            clone.ID,
		Instructions:  clone.Instructions,
		MaxIterations: clone.MaxIterations,
		JSONMode:      clone.JSONMode,
		With:          clone.With,
		Tools:         clone.Tools,
		MCPs:          clone.MCPs,
		Memory:        clone.Memory,
		Attachments:   clone.Attachments,
	}
	dto.Model = exportAgentModel(&clone.Model)
	dto.Actions = exportAgentActions(clone.Actions)
	dto.Env = maskEnv(clone.Env)
	return dto, nil
}

func exportAgentModel(model *agent.Model) AgentModelDTO {
	if model == nil {
		return AgentModelDTO{}
	}
	return AgentModelDTO{Ref: model.Ref}
}

func exportAgentActions(actions []*agent.ActionConfig) []AgentActionDTO {
	if len(actions) == 0 {
		return nil
	}
	out := make([]AgentActionDTO, 0, len(actions))
	for i := range actions {
		action := actions[i]
		if action == nil {
			continue
		}
		out = append(out, AgentActionDTO{
			ID:           action.ID,
			Prompt:       action.Prompt,
			InputSchema:  action.InputSchema,
			OutputSchema: action.OutputSchema,
			With:         action.With,
			JSONMode:     action.JSONMode,
			Attachments:  action.Attachments,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func maskEnv(env *core.EnvMap) map[string]string {
	if env == nil || len(*env) == 0 {
		return nil
	}
	masked := make(map[string]string, len(*env))
	for key, value := range *env {
		if isSensitiveEnvKey(key) {
			masked[key] = redaction
			continue
		}
		masked[key] = value
	}
	return masked
}

func toAgentDTO(src map[string]any) (AgentDTO, error) {
	cfg := &agent.Config{}
	if err := cfg.FromMap(src); err != nil {
		return AgentDTO{}, fmt.Errorf("map to agent config: %w", err)
	}
	return ConvertAgentConfigToDTO(cfg)
}

func toAgentListItem(src map[string]any) (AgentListItem, error) {
	dto, err := toAgentDTO(src)
	if err != nil {
		return AgentListItem{}, err
	}
	etag := httpdto.AsString(src["_etag"])
	if etag == "" {
		etag = httpdto.AsString(src["etag"])
	}
	return AgentListItem{AgentDTO: dto, ETag: etag}, nil
}
