package tkrouter

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// TaskResponse is a DTO for task responses that avoids circular references
type TaskResponse struct {
	// BaseConfig fields
	ID      string       `json:"id"`
	Type    task.Type    `json:"type"`
	Action  string       `json:"action,omitempty"`
	With    *core.Input  `json:"with,omitempty"`
	Env     *core.EnvMap `json:"env,omitempty"`
	Outputs *core.Input  `json:"outputs,omitempty"`
	// Task-specific fields (without recursive Tasks/Task fields)
	Condition  string         `json:"condition,omitempty"`
	Routes     map[string]any `json:"routes,omitempty"`
	Items      any            `json:"items,omitempty"`
	Mode       string         `json:"mode,omitempty"`
	Strategy   string         `json:"strategy,omitempty"`
	SignalName string         `json:"signal_name,omitempty"`
	Timeout    string         `json:"timeout,omitempty"`
	// Metadata
	HasSubtasks bool     `json:"has_subtasks"`
	SubtaskIDs  []string `json:"subtask_ids,omitempty"`
}

// ConvertTaskConfigToResponse converts a task.Config to TaskResponse
func ConvertTaskConfigToResponse(cfg *task.Config) TaskResponse {
	resp := TaskResponse{
		ID:          cfg.ID,
		Type:        cfg.Type,
		Action:      cfg.Action,
		With:        cfg.With,
		Env:         cfg.Env,
		Outputs:     cfg.Outputs,
		Condition:   cfg.Condition,
		HasSubtasks: len(cfg.Tasks) > 0 || cfg.Task != nil,
	}

	// Add subtask IDs if present
	if len(cfg.Tasks) > 0 {
		resp.SubtaskIDs = make([]string, len(cfg.Tasks))
		for i := range cfg.Tasks {
			resp.SubtaskIDs[i] = cfg.Tasks[i].ID
		}
	}

	// Add type-specific fields
	switch cfg.Type {
	case task.TaskTypeRouter:
		resp.Routes = cfg.Routes
	case task.TaskTypeParallel:
		if cfg.Strategy != "" {
			resp.Strategy = string(cfg.Strategy)
		}
	case task.TaskTypeCollection:
		resp.Items = cfg.Items
		if cfg.Mode != "" {
			resp.Mode = string(cfg.Mode)
		}
	case task.TaskTypeSignal:
		if cfg.Signal != nil {
			resp.SignalName = cfg.Signal.ID
		}
	case task.TaskTypeWait:
		resp.SignalName = cfg.WaitFor
		if cfg.Timeout != "" {
			resp.Timeout = cfg.Timeout
		}
	}

	return resp
}

// ConvertTaskConfigsToResponses converts multiple task.Config to TaskResponse
func ConvertTaskConfigsToResponses(configs []task.Config) []TaskResponse {
	responses := make([]TaskResponse, len(configs))
	for i := range configs {
		responses[i] = ConvertTaskConfigToResponse(&configs[i])
	}
	return responses
}
