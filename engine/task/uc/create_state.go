package uc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

// Constants for metadata keys
const (
	ParallelConfigKey   = "_parallel_config"
	CollectionConfigKey = "_collection_config"
	ChildConfigsKey     = "child_configs"
)

// TaskConfigDTO is a lightweight DTO for storing essential task configuration data
type TaskConfigDTO struct {
	ID     string            `json:"id"`
	Type   string            `json:"type,omitempty"`
	Action string            `json:"action,omitempty"`
	With   map[string]any    `json:"with,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
	Agent  map[string]any    `json:"agent,omitempty"`
	Tool   map[string]any    `json:"tool,omitempty"`
}

// -----------------------------------------------------------------------------
// CreateTaskState
// -----------------------------------------------------------------------------

type CreateStateInput struct {
	WorkflowState  *workflow.State  `json:"workflow_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	TaskConfig     *task.Config     `json:"task_config"`
}

type CreateState struct {
	taskRepo task.Repository
}

func NewCreateState(taskRepo task.Repository) *CreateState {
	return &CreateState{taskRepo: taskRepo}
}

func (uc *CreateState) Execute(ctx context.Context, input *CreateStateInput) (*task.State, error) {
	envMap := input.TaskConfig.Env
	result, err := uc.processComponent(input, envMap)
	if err != nil {
		return nil, err
	}
	taskExecID := core.MustNewID()
	stateInput := task.CreateStateInput{
		WorkflowID:     input.WorkflowConfig.ID,
		WorkflowExecID: input.WorkflowState.WorkflowExecID,
		TaskID:         input.TaskConfig.ID,
		TaskExecID:     taskExecID,
	}
	taskState, err := task.CreateAndPersistState(ctx, uc.taskRepo, &stateInput, result)
	if err != nil {
		return nil, err
	}
	if err := input.TaskConfig.ValidateInput(ctx, taskState.Input); err != nil {
		return nil, fmt.Errorf("failed to validate task params: %w", err)
	}
	return taskState, nil
}

func (uc *CreateState) processComponent(
	input *CreateStateInput,
	baseEnv *core.EnvMap,
) (*task.PartialState, error) {
	executionType := input.TaskConfig.GetExecType()
	agentConfig := input.TaskConfig.GetAgent()
	toolConfig := input.TaskConfig.GetTool()
	switch {
	case input.TaskConfig.Type == task.TaskTypeParallel:
		return uc.processParallelTask(input, baseEnv)
	case input.TaskConfig.Type == task.TaskTypeCollection:
		return uc.processCollectionTask(input, baseEnv)
	case agentConfig != nil:
		return uc.processAgent(agentConfig, executionType, input.TaskConfig.Action)
	case toolConfig != nil:
		return uc.processTool(toolConfig, executionType)
	default:
		var actionID *string
		if input.TaskConfig.Action != "" {
			actionID = &input.TaskConfig.Action
		}
		return &task.PartialState{
			Component:     core.ComponentTask,
			ExecutionType: executionType,
			Input:         input.TaskConfig.With,
			ActionID:      actionID,
			MergedEnv:     baseEnv,
		}, nil
	}
}

func (uc *CreateState) processAgent(
	agentConfig *agent.Config,
	executionType task.ExecutionType,
	actionID string,
) (*task.PartialState, error) {
	agentID := agentConfig.ID
	return &task.PartialState{
		Component:     core.ComponentAgent,
		ExecutionType: executionType,
		AgentID:       &agentID,
		ActionID:      &actionID,
		Input:         agentConfig.With,
		MergedEnv:     agentConfig.Env,
	}, nil
}

func (uc *CreateState) processTool(
	toolConfig *tool.Config,
	executionType task.ExecutionType,
) (*task.PartialState, error) {
	toolID := toolConfig.ID
	return &task.PartialState{
		Component:     core.ComponentTool,
		ExecutionType: executionType,
		ToolID:        &toolID,
		Input:         toolConfig.With,
		MergedEnv:     toolConfig.Env,
	}, nil
}

func (uc *CreateState) processParallelTask(
	input *CreateStateInput,
	baseEnv *core.EnvMap,
) (*task.PartialState, error) {
	// Store parallel configuration in the parent task's input for child task creation
	parallelConfig := input.TaskConfig.ParallelTask
	// Create enriched input that includes parallel metadata
	parentInput := input.TaskConfig.With
	if parentInput == nil {
		parentInput = &core.Input{}
	}
	// Convert task configs to lightweight DTOs
	childDTOs := make([]TaskConfigDTO, len(parallelConfig.Tasks))
	for i := range parallelConfig.Tasks {
		taskConfig := &parallelConfig.Tasks[i]
		dto := TaskConfigDTO{
			ID:     taskConfig.ID,
			Type:   string(taskConfig.Type),
			Action: taskConfig.Action,
		}
		if taskConfig.With != nil {
			dto.With = *taskConfig.With
		}
		if taskConfig.Env != nil {
			dto.Env = *taskConfig.Env
		}
		if agent := taskConfig.GetAgent(); agent != nil {
			dto.Agent = map[string]any{
				"id":   agent.ID,
				"with": agent.With,
				"env":  agent.Env,
			}
		}
		if tool := taskConfig.GetTool(); tool != nil {
			dto.Tool = map[string]any{
				"id":   tool.ID,
				"with": tool.With,
				"env":  tool.Env,
			}
		}
		childDTOs[i] = dto
	}

	// Store parallel configuration and child task configs as metadata
	(*parentInput)[ParallelConfigKey] = map[string]any{
		"strategy":      parallelConfig.GetStrategy(),
		"max_workers":   parallelConfig.GetMaxWorkers(),
		"timeout":       parallelConfig.Timeout,
		ChildConfigsKey: childDTOs, // Store lightweight DTOs instead of full configs
	}
	return task.CreateParentPartialState(
		parentInput,
		baseEnv,
	), nil
}

func (uc *CreateState) processCollectionTask(
	input *CreateStateInput,
	baseEnv *core.EnvMap,
) (*task.PartialState, error) {
	// Collection tasks are essentially parallel tasks with item expansion
	// Store collection-specific configuration alongside parallel configuration
	collectionConfig := input.TaskConfig.CollectionConfig

	// Create enriched input that includes collection metadata
	parentInput := input.TaskConfig.With
	if parentInput == nil {
		parentInput = &core.Input{}
	}

	// Store collection configuration metadata for the ExecuteCollection activity
	(*parentInput)[CollectionConfigKey] = map[string]any{
		"items":     collectionConfig.Items,
		"filter":    collectionConfig.Filter,
		"item_var":  collectionConfig.GetItemVar(),
		"index_var": collectionConfig.GetIndexVar(),
		"mode":      collectionConfig.GetMode(),
		"batch":     collectionConfig.Batch,
	}

	return task.CreateParentPartialStateWithExecType(
		parentInput,
		baseEnv,
		task.ExecutionCollection,
	), nil
}

// CreateChildTasksInput follows Temporal best practices by passing minimal data
type CreateChildTasksInput struct {
	ParentStateID  core.ID `json:"parent_state_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	WorkflowID     string  `json:"workflow_id"`
}

// CreateChildTasks creates child tasks for a parallel parent using lightweight input
func (uc *CreateState) CreateChildTasks(ctx context.Context, input *CreateChildTasksInput) error {
	parentState, err := uc.taskRepo.GetState(ctx, input.ParentStateID)
	if err != nil {
		return fmt.Errorf("failed to retrieve parent state: %w", err)
	}

	if err := uc.validateParentState(parentState, input.ParentStateID); err != nil {
		return err
	}

	parallelMeta, err := uc.extractParallelConfig(parentState)
	if err != nil {
		return err
	}

	childDTOs, err := uc.extractChildDTOs(parallelMeta)
	if err != nil {
		return err
	}

	childConfigs, err := uc.convertDTOsToConfigs(childDTOs)
	if err != nil {
		return err
	}

	if err := uc.validateChildConfigs(childConfigs); err != nil {
		return err
	}

	return uc.createChildTasksInTransaction(ctx, parentState, childConfigs)
}

// validateParentState validates that the parent state is a parallel task
func (uc *CreateState) validateParentState(parentState *task.State, parentStateID core.ID) error {
	if !parentState.IsParallelExecution() {
		return fmt.Errorf("state %s is not a parent task", parentStateID)
	}
	return nil
}

// extractParallelConfig extracts and validates parallel configuration from parent state
func (uc *CreateState) extractParallelConfig(parentState *task.State) (map[string]any, error) {
	parallelMetaRaw, exists := (*parentState.Input)[ParallelConfigKey]
	if !exists {
		return nil, fmt.Errorf("parent state missing parallel configuration metadata")
	}

	parallelMeta, ok := parallelMetaRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid parallel configuration metadata format")
	}

	return parallelMeta, nil
}

// extractChildDTOs extracts and converts child configurations to DTOs
func (uc *CreateState) extractChildDTOs(parallelMeta map[string]any) ([]TaskConfigDTO, error) {
	childConfigsRaw, ok := parallelMeta[ChildConfigsKey]
	if !ok {
		return nil, fmt.Errorf("parent state missing child configurations")
	}

	rawSlice, ok := childConfigsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid child configurations format: expected array, got %T", childConfigsRaw)
	}

	childDTOs := make([]TaskConfigDTO, len(rawSlice))
	for i, v := range rawSlice {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal child DTO at index %d: %w", i, err)
		}
		if err := json.Unmarshal(b, &childDTOs[i]); err != nil {
			return nil, fmt.Errorf("invalid child DTO at index %d: %w", i, err)
		}
	}

	return childDTOs, nil
}

// convertDTOsToConfigs converts DTOs back to full task.Config structures
func (uc *CreateState) convertDTOsToConfigs(childDTOs []TaskConfigDTO) ([]task.Config, error) {
	childConfigs := make([]task.Config, len(childDTOs))
	for i, dto := range childDTOs {
		config, err := uc.convertDTOToConfig(&dto)
		if err != nil {
			return nil, fmt.Errorf("failed to convert DTO at index %d: %w", i, err)
		}
		childConfigs[i] = config
	}
	return childConfigs, nil
}

// convertDTOToConfig converts a single DTO to task.Config
func (uc *CreateState) convertDTOToConfig(dto *TaskConfigDTO) (task.Config, error) {
	config := task.Config{}
	config.ID = dto.ID
	config.Type = task.Type(dto.Type)
	config.Action = dto.Action

	if dto.With != nil {
		input := core.Input(dto.With)
		config.With = &input
	}
	if dto.Env != nil {
		envMap := core.EnvMap(dto.Env)
		config.Env = &envMap
	}

	if err := uc.setAgentConfig(&config, dto); err != nil {
		return config, err
	}

	if err := uc.setToolConfig(&config, dto); err != nil {
		return config, err
	}

	return config, nil
}

// setAgentConfig reconstructs agent config if present in DTO
func (uc *CreateState) setAgentConfig(
	config *task.Config,
	dto *TaskConfigDTO,
) error { //nolint:unparam // error return reserved for future validation
	if dto.Agent == nil {
		return nil
	}

	agentID, ok := dto.Agent["id"].(string)
	if !ok {
		return nil
	}

	agentConfig := &agent.Config{ID: agentID}

	if agentWith, ok := dto.Agent["with"].(map[string]any); ok {
		input := core.Input(agentWith)
		agentConfig.With = &input
	}
	if agentEnv, ok := dto.Agent["env"].(map[string]string); ok {
		envMap := core.EnvMap(agentEnv)
		agentConfig.Env = &envMap
	}

	config.Agent = agentConfig
	return nil
}

// setToolConfig reconstructs tool config if present in DTO
func (uc *CreateState) setToolConfig(
	config *task.Config,
	dto *TaskConfigDTO,
) error { //nolint:unparam // error return reserved for future validation
	if dto.Tool == nil {
		return nil
	}

	toolID, ok := dto.Tool["id"].(string)
	if !ok {
		return nil
	}

	toolConfig := &tool.Config{ID: toolID}

	if toolWith, ok := dto.Tool["with"].(map[string]any); ok {
		input := core.Input(toolWith)
		toolConfig.With = &input
	}
	if toolEnv, ok := dto.Tool["env"].(map[string]string); ok {
		envMap := core.EnvMap(toolEnv)
		toolConfig.Env = &envMap
	}

	config.Tool = toolConfig
	return nil
}

// validateChildConfigs validates that each child config has required fields
func (uc *CreateState) validateChildConfigs(childConfigs []task.Config) error {
	for i := range childConfigs {
		if childConfigs[i].ID == "" {
			return fmt.Errorf("child config at index %d missing required ID field", i)
		}
	}
	return nil
}

// createChildTasksInTransaction creates all child tasks atomically
func (uc *CreateState) createChildTasksInTransaction(
	ctx context.Context,
	parentState *task.State,
	childConfigs []task.Config,
) error {
	// Prepare all child states first
	var childStates []*task.State
	for i := range childConfigs {
		childConfig := &childConfigs[i]
		childTaskExecID := core.MustNewID()
		// Create child partial state by recursively processing the child config
		childPartialState, err := uc.processChildConfig(childConfig)
		if err != nil {
			return fmt.Errorf("failed to process child config %s: %w", childConfig.ID, err)
		}
		// Create child state input with parent reference
		childStateInput := &task.CreateStateInput{
			WorkflowID:     parentState.WorkflowID,
			WorkflowExecID: parentState.WorkflowExecID,
			TaskID:         childConfig.ID,
			TaskExecID:     childTaskExecID,
		}
		// Set parent relationship in partial state
		parentID := parentState.TaskExecID
		childPartialState.ParentStateID = &parentID
		// Create child state (without persisting yet)
		childState := task.CreateBasicState(childStateInput, childPartialState)
		childStates = append(childStates, childState)
	}

	// Create all child states atomically in a single transaction
	return uc.taskRepo.CreateChildStatesInTransaction(ctx, parentState.TaskExecID, childStates)
}

// processChildConfig processes a child task config to create its partial state
func (uc *CreateState) processChildConfig(childConfig *task.Config) (*task.PartialState, error) {
	// Use the existing processComponent logic but for child config
	baseEnv := childConfig.Env
	executionType := childConfig.GetExecType()
	agentConfig := childConfig.GetAgent()
	toolConfig := childConfig.GetTool()

	switch {
	case childConfig.Type == task.TaskTypeParallel:
		// Nested parallel task - not yet supported in new architecture
		return nil, fmt.Errorf("nested parallel tasks not yet supported")
	case agentConfig != nil:
		return uc.processAgent(agentConfig, executionType, childConfig.Action)
	case toolConfig != nil:
		return uc.processTool(toolConfig, executionType)
	default:
		var actionID *string
		if childConfig.Action != "" {
			actionID = &childConfig.Action
		}
		return &task.PartialState{
			Component:     core.ComponentTask,
			ExecutionType: executionType,
			Input:         childConfig.With,
			ActionID:      actionID,
			MergedEnv:     baseEnv,
		}, nil
	}
}
