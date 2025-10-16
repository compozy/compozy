package wfrouter

import (
	"encoding/json"
	"fmt"

	agentrouter "github.com/compozy/compozy/engine/agent/router"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	toolrouter "github.com/compozy/compozy/engine/tool/router"
	"github.com/compozy/compozy/engine/workflow"
)

// WorkflowDTO is the canonical typed representation for workflows
type WorkflowDTO struct {
	ID          string             `json:"id"`
	Version     string             `json:"version,omitempty"`
	Description string             `json:"description,omitempty"`
	Author      *core.Author       `json:"author,omitempty"`
	Config      workflow.Opts      `json:"config"`
	Resource    string             `json:"resource,omitempty"`
	Schemas     []schema.Schema    `json:"schemas,omitempty"`
	Outputs     *core.Output       `json:"outputs,omitempty"`
	Triggers    []workflow.Trigger `json:"triggers,omitempty"`
	Schedule    *workflow.Schedule `json:"schedule,omitempty"`
	TaskCount   int                `json:"task_count"`
	TaskIDs     []string           `json:"task_ids,omitempty"`
	ToolCount   int                `json:"tool_count"`
	AgentCount  int                `json:"agent_count"`
	MCPCount    int                `json:"mcp_count"`
	MCPs        []mcp.Config       `json:"mcps,omitempty"`
	// Expandable collections: marshaled as either []string or []<DTO>
	Tasks  TasksOrDTOs  `json:"tasks"`
	Agents AgentsOrDTOs `json:"agents"`
	Tools  ToolsOrDTOs  `json:"tools"`
}

// WorkflowListItem is the list item wrapper including optional strong ETag
type WorkflowListItem struct {
	WorkflowDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// WorkflowsListResponse is the typed list payload returned from GET /workflows.
type WorkflowsListResponse struct {
	Workflows []WorkflowListItem  `json:"workflows"`
	Page      httpdto.PageInfoDTO `json:"page"`
}

// WorkflowExecutionDTO wraps workflow.State with an optional aggregated usage summary.
type WorkflowExecutionDTO struct {
	*workflow.State
	Usage *router.UsageSummary `json:"usage,omitempty"`
}

func newWorkflowExecutionDTO(state *workflow.State, usageSummary *router.UsageSummary) *WorkflowExecutionDTO {
	if state == nil {
		return nil
	}
	return &WorkflowExecutionDTO{State: state, Usage: usageSummary}
}

// ConvertWorkflowConfigToDTO converts a workflow.Config to WorkflowDTO
func ConvertWorkflowConfigToDTO(cfg *workflow.Config) WorkflowDTO {
	resp := WorkflowDTO{
		ID:          cfg.ID,
		Version:     cfg.Version,
		Description: cfg.Description,
		Author:      cfg.Author,
		Config:      cfg.Opts,
		Resource:    cfg.Resource,
		Schemas:     cfg.Schemas,
		Outputs:     cfg.Outputs,
		Triggers:    cfg.Triggers,
		Schedule:    cfg.Schedule,
		TaskCount:   len(cfg.Tasks),
		ToolCount:   len(cfg.Tools),
		AgentCount:  len(cfg.Agents),
		MCPCount:    len(cfg.MCPs),
		MCPs:        cfg.MCPs,
	}
	if len(cfg.Tasks) > 0 {
		resp.TaskIDs = make([]string, len(cfg.Tasks))
		for i := range cfg.Tasks {
			resp.TaskIDs[i] = cfg.Tasks[i].ID
		}
	}
	return resp
}

// ConvertWorkflowConfigsToDTOs converts multiple workflow.Config to WorkflowDTO
func ConvertWorkflowConfigsToDTOs(configs []*workflow.Config) []WorkflowDTO {
	responses := make([]WorkflowDTO, len(configs))
	for i := range configs {
		responses[i] = ConvertWorkflowConfigToDTO(configs[i])
	}
	return responses
}

// Union field types with custom JSON marshalers for expand behavior
type TasksOrDTOs struct {
	IDs      []string
	Expanded []tkrouter.TaskDTO
}

func (t TasksOrDTOs) MarshalJSON() ([]byte, error) {
	if len(t.IDs) > 0 && len(t.Expanded) > 0 {
		return nil, fmt.Errorf("tasks payload has both ids and expanded values set")
	}
	if len(t.Expanded) > 0 {
		return json.Marshal(t.Expanded)
	}
	return json.Marshal(t.IDs)
}

type AgentsOrDTOs struct {
	IDs      []string
	Expanded []agentrouter.AgentDTO
}

func (a AgentsOrDTOs) MarshalJSON() ([]byte, error) {
	if len(a.IDs) > 0 && len(a.Expanded) > 0 {
		return nil, fmt.Errorf("agents payload has both ids and expanded values set")
	}
	if len(a.Expanded) > 0 {
		return json.Marshal(a.Expanded)
	}
	return json.Marshal(a.IDs)
}

type ToolsOrDTOs struct {
	IDs      []string
	Expanded []toolrouter.ToolDTO
}

func (t ToolsOrDTOs) MarshalJSON() ([]byte, error) {
	if len(t.IDs) > 0 && len(t.Expanded) > 0 {
		return nil, fmt.Errorf("tools payload has both ids and expanded values set")
	}
	if len(t.Expanded) > 0 {
		return json.Marshal(t.Expanded)
	}
	return json.Marshal(t.IDs)
}
