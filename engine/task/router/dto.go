package tkrouter

import (
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
)

const expandKeySubtasks = "subtasks"

// TaskResponse is a DTO for task responses that avoids circular references
type TaskResponse struct {
	ID            string                  `json:"id"`
	Type          task.Type               `json:"type"`
	Resource      string                  `json:"resource,omitempty"`
	Config        *core.GlobalOpts        `json:"config,omitempty"`
	Agent         *agent.Config           `json:"agent,omitempty"`
	Tool          *tool.Config            `json:"tool,omitempty"`
	Input         *schema.Schema          `json:"input,omitempty"`
	Output        *schema.Schema          `json:"output,omitempty"`
	With          *core.Input             `json:"with,omitempty"`
	Outputs       *core.Input             `json:"outputs,omitempty"`
	Env           *core.EnvMap            `json:"env,omitempty"`
	OnSuccess     *core.SuccessTransition `json:"on_success,omitempty"`
	OnError       *core.ErrorTransition   `json:"on_error,omitempty"`
	Sleep         string                  `json:"sleep,omitempty"`
	Final         bool                    `json:"final,omitempty"`
	Timeout       string                  `json:"timeout,omitempty"`
	Retries       int                     `json:"retries,omitempty"`
	Condition     string                  `json:"condition,omitempty"`
	Attachments   attachment.Attachments  `json:"attachments,omitempty"`
	Action        string                  `json:"action,omitempty"`
	Prompt        string                  `json:"prompt,omitempty"`
	ModelConfig   *core.ProviderConfig    `json:"model_config,omitempty"`
	Tools         []tool.Config           `json:"tools,omitempty"`
	MCPs          []mcp.Config            `json:"mcps,omitempty"`
	MaxIterations int                     `json:"max_iterations,omitempty"`
	Memory        []core.MemoryReference  `json:"memory,omitempty"`
	Routes        map[string]any          `json:"routes,omitempty"`
	Items         any                     `json:"items,omitempty"`
	Filter        string                  `json:"filter,omitempty"`
	ItemVar       string                  `json:"item_var,omitempty"`
	IndexVar      string                  `json:"index_var,omitempty"`
	Mode          string                  `json:"mode,omitempty"`
	Batch         int                     `json:"batch,omitempty"`
	Strategy      string                  `json:"strategy,omitempty"`
	MaxWorkers    int                     `json:"max_workers,omitempty"`
	SignalName    string                  `json:"signal_name,omitempty"`
	Signal        *task.SignalConfig      `json:"signal,omitempty"`
	WaitFor       string                  `json:"wait_for,omitempty"`
	OnTimeout     string                  `json:"on_timeout,omitempty"`
	Operation     task.MemoryOpType       `json:"operation,omitempty"`
	MemoryRef     string                  `json:"memory_ref,omitempty"`
	KeyTemplate   string                  `json:"key_template,omitempty"`
	Payload       any                     `json:"payload,omitempty"`
	BatchSize     int                     `json:"batch_size,omitempty"`
	MaxKeys       int                     `json:"max_keys,omitempty"`
	FlushConfig   *task.FlushConfig       `json:"flush_config,omitempty"`
	HealthConfig  *task.HealthConfig      `json:"health_config,omitempty"`
	StatsConfig   *task.StatsConfig       `json:"stats_config,omitempty"`
	ClearConfig   *task.ClearConfig       `json:"clear_config,omitempty"`
	HasSubtasks   bool                    `json:"has_subtasks"`
	SubtaskIDs    []string                `json:"subtask_ids,omitempty"`
}

// TaskDTO is the single-item typed transport shape (alias via embedding for flexibility)
type TaskDTO struct {
	TaskResponse
	Tasks        []*TaskDTO `json:"tasks,omitempty"`
	TemplateTask *TaskDTO   `json:"task,omitempty"`
}

// TaskListItem is the list item wrapper including optional strong ETag
type TaskListItem struct {
	TaskDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// TasksListResponse is the typed list payload returned from GET /tasks.
type TasksListResponse struct {
	Tasks []TaskListItem      `json:"tasks"`
	Page  httpdto.PageInfoDTO `json:"page"`
}

// TaskExecSyncResponse is returned from POST /tasks/{task_id}/executions/sync.
type TaskExecSyncResponse struct {
	Output *core.Output `json:"output,omitempty"`
	ExecID string       `json:"exec_id"          example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
}

// TaskExecAsyncResponse is returned from POST /tasks/{task_id}/executions.
type TaskExecAsyncResponse struct {
	ExecID  string `json:"exec_id"  example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
	ExecURL string `json:"exec_url" example:"https://api.compozy.dev/api/v0/executions/tasks/2Z4PVTL6K27XVT4A3NPKMDD5BG"`
}

// ToTaskDTOForWorkflow is an exported helper for workflow DTO expansion mapping.
func ToTaskDTOForWorkflow(src map[string]any) (TaskDTO, error) {
	return toTaskDTO(src, map[string]bool{expandKeySubtasks: true})
}

// ConvertTaskConfigToResponse converts a task.Config to TaskResponse
func ConvertTaskConfigToResponse(cfg *task.Config) (TaskResponse, error) {
	if cfg == nil {
		return TaskResponse{}, fmt.Errorf("task config is nil")
	}
	clone, err := core.DeepCopy[*task.Config](cfg)
	if err != nil {
		return TaskResponse{}, fmt.Errorf("deep copy task config: %w", err)
	}
	resp := TaskResponse{
		ID:            clone.ID,
		Type:          clone.Type,
		Resource:      clone.Resource,
		Agent:         clone.Agent,
		Tool:          clone.Tool,
		Input:         clone.InputSchema,
		Output:        clone.OutputSchema,
		With:          clone.With,
		Outputs:       clone.Outputs,
		Env:           clone.Env,
		OnSuccess:     clone.OnSuccess,
		OnError:       clone.OnError,
		Sleep:         clone.Sleep,
		Final:         clone.Final,
		Timeout:       clone.Timeout,
		Retries:       clone.Retries,
		Condition:     clone.Condition,
		Attachments:   clone.Attachments,
		Action:        clone.Action,
		Prompt:        clone.Prompt,
		Tools:         clone.Tools,
		MCPs:          clone.MCPs,
		MaxIterations: clone.MaxIterations,
		Memory:        clone.Memory,
		Routes:        clone.Routes,
		Items:         clone.Items,
		Filter:        clone.Filter,
		ItemVar:       clone.ItemVar,
		IndexVar:      clone.IndexVar,
		Batch:         clone.Batch,
		MaxWorkers:    clone.MaxWorkers,
		Signal:        clone.Signal,
		WaitFor:       clone.WaitFor,
		OnTimeout:     clone.OnTimeout,
		Operation:     clone.Operation,
		MemoryRef:     clone.MemoryRef,
		KeyTemplate:   clone.KeyTemplate,
		Payload:       clone.Payload,
		BatchSize:     clone.BatchSize,
		MaxKeys:       clone.MaxKeys,
		FlushConfig:   clone.FlushConfig,
		HealthConfig:  clone.HealthConfig,
		StatsConfig:   clone.StatsConfig,
		ClearConfig:   clone.ClearConfig,
	}
	if clone.Config != (core.GlobalOpts{}) {
		resp.Config = &clone.Config
	}
	if hasProviderConfig(&clone.ModelConfig) {
		resp.ModelConfig = &clone.ModelConfig
	}
	if clone.Mode != "" {
		resp.Mode = string(clone.Mode)
	}
	if clone.Strategy != "" {
		resp.Strategy = string(clone.Strategy)
	}
	if clone.Signal != nil {
		resp.SignalName = clone.Signal.ID
	}
	if clone.Type == task.TaskTypeWait {
		resp.SignalName = clone.WaitFor
	}
	populateSubtaskMeta(&resp, clone)
	return resp, nil
}

// ConvertTaskConfigsToResponses converts multiple task.Config to TaskResponse
func ConvertTaskConfigsToResponses(configs []task.Config) ([]TaskResponse, error) {
	responses := make([]TaskResponse, 0, len(configs))
	for i := range configs {
		resp, err := ConvertTaskConfigToResponse(&configs[i])
		if err != nil {
			return nil, err
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

// Helpers for mapping from UC map payloads (list/get) to typed DTOs
func toTaskDTO(src map[string]any, expand map[string]bool) (TaskDTO, error) {
	cfg := &task.Config{}
	if err := cfg.FromMap(src); err != nil {
		return TaskDTO{}, fmt.Errorf("map to task config: %w", err)
	}
	return taskConfigToDTO(cfg, expand)
}

func toTaskListItem(src map[string]any, expand map[string]bool) (TaskListItem, error) {
	dto, err := toTaskDTO(src, expand)
	if err != nil {
		return TaskListItem{}, err
	}
	return TaskListItem{TaskDTO: dto, ETag: httpdto.AsString(src["_etag"])}, nil
}

func taskConfigToDTO(cfg *task.Config, expand map[string]bool) (TaskDTO, error) {
	resp, err := ConvertTaskConfigToResponse(cfg)
	if err != nil {
		return TaskDTO{}, err
	}
	dto := TaskDTO{TaskResponse: resp}
	if expand != nil && expand[expandKeySubtasks] {
		if len(cfg.Tasks) > 0 {
			dto.Tasks = make([]*TaskDTO, 0, len(cfg.Tasks))
			for i := range cfg.Tasks {
				child, childErr := taskConfigToDTO(&cfg.Tasks[i], expand)
				if childErr != nil {
					return TaskDTO{}, childErr
				}
				childCopy := child
				dto.Tasks = append(dto.Tasks, &childCopy)
			}
		}
		if cfg.Task != nil {
			child, childErr := taskConfigToDTO(cfg.Task, expand)
			if childErr != nil {
				return TaskDTO{}, childErr
			}
			childCopy := child
			dto.TemplateTask = &childCopy
		}
	}
	return dto, nil
}

func populateSubtaskMeta(resp *TaskResponse, cfg *task.Config) {
	hs := len(cfg.Tasks) > 0 || cfg.Task != nil
	resp.HasSubtasks = hs
	if len(cfg.Tasks) > 0 {
		resp.SubtaskIDs = make([]string, len(cfg.Tasks))
		for i := range cfg.Tasks {
			resp.SubtaskIDs[i] = cfg.Tasks[i].ID
		}
	}
	if resp.SubtaskIDs == nil && cfg.Task != nil && cfg.Task.ID != "" {
		resp.SubtaskIDs = []string{cfg.Task.ID}
	}
}

func hasProviderConfig(cfg *core.ProviderConfig) bool {
	if cfg == nil {
		return false
	}
	for _, val := range []string{string(cfg.Provider), cfg.Model, cfg.APIKey, cfg.APIURL, cfg.Organization} {
		if val != "" {
			return true
		}
	}
	if cfg.Default || cfg.MaxToolIterations != 0 {
		return true
	}
	return providerParamsHaveValues(&cfg.Params)
}

func providerParamsHaveValues(params *core.PromptParams) bool {
	if params == nil {
		return false
	}
	if params.IsSetMaxTokens() || params.IsSetTopK() || params.IsSetSeed() || params.IsSetMinLength() ||
		params.MaxLength != 0 {
		return true
	}
	if params.IsSetTemperature() || params.IsSetTopP() || params.IsSetRepetitionPenalty() {
		return true
	}
	if params.IsSetStopWords() && len(params.StopWords) > 0 {
		return true
	}
	return false
}
