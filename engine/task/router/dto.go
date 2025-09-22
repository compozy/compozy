package tkrouter

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/task"
)

// TaskResponse is a DTO for task responses that avoids circular references
type TaskResponse struct {
	ID          string         `json:"id"`
	Type        task.Type      `json:"type"`
	Action      string         `json:"action,omitempty"`
	With        *core.Input    `json:"with,omitempty"`
	Env         *core.EnvMap   `json:"env,omitempty"`
	Outputs     *core.Input    `json:"outputs,omitempty"`
	Condition   string         `json:"condition,omitempty"`
	Routes      map[string]any `json:"routes,omitempty"`
	Items       any            `json:"items,omitempty"`
	Mode        string         `json:"mode,omitempty"`
	Strategy    string         `json:"strategy,omitempty"`
	SignalName  string         `json:"signal_name,omitempty"`
	Timeout     string         `json:"timeout,omitempty"`
	HasSubtasks bool           `json:"has_subtasks"`
	SubtaskIDs  []string       `json:"subtask_ids,omitempty"`
}

// TaskDTO is the single-item typed transport shape (alias via embedding for flexibility)
type TaskDTO struct{ TaskResponse }

// TaskListItem is the list item wrapper including optional strong ETag
type TaskListItem struct {
	TaskResponse
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// TasksListResponse is the typed list payload returned from GET /tasks.
type TasksListResponse struct {
	Tasks []TaskListItem     `json:"tasks"`
	Page  router.PageInfoDTO `json:"page"`
}

// ToTaskDTOForWorkflow is an exported helper for workflow DTO expansion mapping.
func ToTaskDTOForWorkflow(src map[string]any) TaskDTO { return toTaskDTO(src) }

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
	if len(cfg.Tasks) > 0 {
		resp.SubtaskIDs = make([]string, len(cfg.Tasks))
		for i := range cfg.Tasks {
			resp.SubtaskIDs[i] = cfg.Tasks[i].ID
		}
	}
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

// Helpers for mapping from UC map payloads (list/get) to typed DTOs
func toTaskDTO(src map[string]any) TaskDTO {
	return TaskDTO{TaskResponse: TaskResponse{
		ID:         router.AsString(src["id"]),
		Type:       task.Type(router.AsString(src["type"])),
		Action:     router.AsString(src["action"]),
		With:       toInput(src["with"]),
		Env:        toEnv(src["env"]),
		Outputs:    toInput(src["outputs"]),
		Condition:  router.AsString(src["condition"]),
		Routes:     router.AsMap(src["routes"]),
		Items:      src["items"],
		Mode:       router.AsString(src["mode"]),
		Strategy:   router.AsString(src["strategy"]),
		SignalName: router.AsString(src["signal_name"]),
		Timeout:    router.AsString(src["timeout"]),
	}}
}

func toTaskListItem(src map[string]any) TaskListItem {
	return TaskListItem{TaskResponse: TaskResponse{
		ID:         router.AsString(src["id"]),
		Type:       task.Type(router.AsString(src["type"])),
		Action:     router.AsString(src["action"]),
		With:       toInput(src["with"]),
		Env:        toEnv(src["env"]),
		Outputs:    toInput(src["outputs"]),
		Condition:  router.AsString(src["condition"]),
		Routes:     router.AsMap(src["routes"]),
		Items:      src["items"],
		Mode:       router.AsString(src["mode"]),
		Strategy:   router.AsString(src["strategy"]),
		SignalName: router.AsString(src["signal_name"]),
		Timeout:    router.AsString(src["timeout"]),
	}, ETag: router.AsString(src["_etag"])}
}

func toInput(v any) *core.Input {
	if m, ok := v.(map[string]any); ok {
		in := core.Input(m)
		return &in
	}
	return nil
}

func toEnv(v any) *core.EnvMap {
	mv, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	res := core.EnvMap{}
	for k, val := range mv {
		if s, ok2 := val.(string); ok2 {
			res[k] = s
		}
	}
	return &res
}
