package wfrouter

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

// WorkflowResponse is a DTO for workflow responses that avoids circular references
type WorkflowResponse struct {
	// Basic workflow fields
	ID          string       `json:"id"`
	Version     string       `json:"version,omitempty"`
	Description string       `json:"description,omitempty"`
	Author      *core.Author `json:"author,omitempty"`

	// Configuration
	Config   workflow.Opts      `json:"config"`
	Triggers []workflow.Trigger `json:"triggers,omitempty"`
	Schedule *workflow.Schedule `json:"schedule,omitempty"`

	// Task information (without circular references)
	TaskCount int      `json:"task_count"`
	TaskIDs   []string `json:"task_ids,omitempty"`

	// Other counts
	ToolCount  int `json:"tool_count"`
	AgentCount int `json:"agent_count"`
	MCPCount   int `json:"mcp_count"`
}

// ConvertWorkflowConfigToResponse converts a workflow.Config to WorkflowResponse
func ConvertWorkflowConfigToResponse(cfg *workflow.Config) WorkflowResponse {
	resp := WorkflowResponse{
		ID:          cfg.ID,
		Version:     cfg.Version,
		Description: cfg.Description,
		Author:      cfg.Author,
		Config:      cfg.Opts,
		Triggers:    cfg.Triggers,
		Schedule:    cfg.Schedule,
		TaskCount:   len(cfg.Tasks),
		ToolCount:   len(cfg.Tools),
		AgentCount:  len(cfg.Agents),
		MCPCount:    len(cfg.MCPs),
	}

	// Add task IDs if present
	if len(cfg.Tasks) > 0 {
		resp.TaskIDs = make([]string, len(cfg.Tasks))
		for i := range cfg.Tasks {
			resp.TaskIDs[i] = cfg.Tasks[i].ID
		}
	}

	return resp
}

// ConvertWorkflowConfigsToResponses converts multiple workflow.Config to WorkflowResponse
func ConvertWorkflowConfigsToResponses(configs []*workflow.Config) []WorkflowResponse {
	responses := make([]WorkflowResponse, len(configs))
	for i := range configs {
		responses[i] = ConvertWorkflowConfigToResponse(configs[i])
	}
	return responses
}
