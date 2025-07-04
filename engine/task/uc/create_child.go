package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
)

// CreateChildTasksInput follows Temporal best practices by passing minimal data
type CreateChildTasksInput struct {
	ParentStateID  core.ID `json:"parent_state_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	WorkflowID     string  `json:"workflow_id"`
}

type CreateChildTasks struct {
	taskRepo     task.Repository
	configStore  services.ConfigStore
	task2Factory task2.Factory
}

func NewCreateChildTasksUC(
	taskRepo task.Repository,
	configStore services.ConfigStore,
	task2Factory task2.Factory,
) *CreateChildTasks {
	return &CreateChildTasks{
		taskRepo:     taskRepo,
		configStore:  configStore,
		task2Factory: task2Factory,
	}
}

func (uc *CreateChildTasks) Execute(ctx context.Context, input *CreateChildTasksInput) error {
	parentState, err := uc.taskRepo.GetState(ctx, input.ParentStateID)
	if err != nil {
		return fmt.Errorf("failed to retrieve parent state: %w", err)
	}

	if err := uc.validateParentState(parentState); err != nil {
		return err
	}

	// Load parent task config to get its environment
	parentConfig, err := uc.configStore.Get(ctx, parentState.TaskExecID.String())
	if err != nil {
		return fmt.Errorf("failed to load parent task config: %w", err)
	}

	switch parentState.ExecutionType {
	case task.ExecutionParallel:
		return uc.createParallelChildren(ctx, parentState, parentConfig)
	case task.ExecutionCollection:
		return uc.createCollectionChildren(ctx, parentState, parentConfig)
	case task.ExecutionComposite:
		return uc.createCompositeChildren(ctx, parentState, parentConfig)
	default:
		return fmt.Errorf("unsupported execution type for child creation: %s", parentState.ExecutionType)
	}
}

func (uc *CreateChildTasks) createParallelChildren(
	ctx context.Context,
	parentState *task.State,
	parentConfig *task.Config,
) error {
	// Create config repository from factory
	configRepo, err := uc.task2Factory.CreateTaskConfigRepository(uc.configStore)
	if err != nil {
		return fmt.Errorf("failed to create task config repository: %w", err)
	}

	metadataAny, err := configRepo.LoadParallelMetadata(ctx, parentState.TaskExecID)
	if err != nil {
		return err
	}

	metadata, ok := metadataAny.(*task2core.ParallelTaskMetadata)
	if !ok {
		return fmt.Errorf(
			"invalid metadata type for parallel task: expected *ParallelTaskMetadata, got %T",
			metadataAny,
		)
	}

	if err := uc.validateChildConfigs(metadata.ChildConfigs); err != nil {
		return err
	}

	return uc.createChildStatesInTransaction(ctx, parentState, parentConfig, metadata.ChildConfigs)
}

func (uc *CreateChildTasks) createCollectionChildren(
	ctx context.Context,
	parentState *task.State,
	parentConfig *task.Config,
) error {
	// Create config repository from factory
	configRepo, err := uc.task2Factory.CreateTaskConfigRepository(uc.configStore)
	if err != nil {
		return fmt.Errorf("failed to create task config repository: %w", err)
	}

	metadataAny, err := configRepo.LoadCollectionMetadata(ctx, parentState.TaskExecID)
	if err != nil {
		return err
	}

	metadata, ok := metadataAny.(*task2core.CollectionTaskMetadata)
	if !ok {
		return fmt.Errorf(
			"invalid metadata type for collection task: expected *CollectionTaskMetadata, got %T",
			metadataAny,
		)
	}

	if err := uc.validateChildConfigs(metadata.ChildConfigs); err != nil {
		return err
	}

	return uc.createChildStatesInTransaction(ctx, parentState, parentConfig, metadata.ChildConfigs)
}

func (uc *CreateChildTasks) createCompositeChildren(
	ctx context.Context,
	parentState *task.State,
	parentConfig *task.Config,
) error {
	// Create config repository from factory
	configRepo, err := uc.task2Factory.CreateTaskConfigRepository(uc.configStore)
	if err != nil {
		return fmt.Errorf("failed to create task config repository: %w", err)
	}

	metadataAny, err := configRepo.LoadCompositeMetadata(ctx, parentState.TaskExecID)
	if err != nil {
		return err
	}

	metadata, ok := metadataAny.(*task2core.CompositeTaskMetadata)
	if !ok {
		return fmt.Errorf(
			"invalid metadata type for composite task: expected *CompositeTaskMetadata, got %T",
			metadataAny,
		)
	}

	if err := uc.validateChildConfigs(metadata.ChildConfigs); err != nil {
		return err
	}
	return uc.createChildStatesInTransaction(ctx, parentState, parentConfig, metadata.ChildConfigs)
}

// validateParentState validates that the parent state can have child tasks
func (uc *CreateChildTasks) validateParentState(parentState *task.State) error {
	if !parentState.CanHaveChildren() {
		return fmt.Errorf("state %s is not a parent task", parentState.TaskExecID)
	}
	return nil
}

func (uc *CreateChildTasks) validateChildConfigs(childConfigs []*task.Config) error {
	for i := range childConfigs {
		if childConfigs[i].ID == "" {
			return fmt.Errorf("child config at index %d missing required ID field", i)
		}
	}
	return nil
}

// childConfigRef holds a reference to a child config and its execution ID
type childConfigRef struct {
	id  core.ID
	cfg *task.Config
}

// createChildStatesInTransaction creates all child tasks atomically
func (uc *CreateChildTasks) createChildStatesInTransaction(
	ctx context.Context,
	parentState *task.State,
	parentConfig *task.Config,
	childConfigs []*task.Config,
) error {
	log := logger.FromContext(ctx)
	// Collect configs to save after transaction succeeds
	var configsToSave []childConfigRef

	// Prepare all child states first
	var childStates []*task.State
	for i := range childConfigs {
		childConfig := childConfigs[i]
		childTaskExecID := core.MustNewID()

		// Create child partial state by recursively processing the child config
		// Pass parent's environment for inheritance
		childPartialState, err := uc.processChildConfig(childConfig, parentConfig.Env)
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

		// Collect config to save after transaction succeeds
		configsToSave = append(configsToSave, childConfigRef{
			id:  childTaskExecID,
			cfg: childConfig,
		})
	}

	// Create all child states atomically in a single transaction
	err := uc.taskRepo.CreateChildStatesInTransaction(ctx, parentState.TaskExecID, childStates)
	if err != nil {
		return err
	}

	// Save configs only after database transaction succeeds
	var savedConfigIDs []core.ID
	for _, c := range configsToSave {
		if err := uc.configStore.Save(ctx, c.id.String(), c.cfg); err != nil {
			// Best-effort rollback: delete any configs already saved
			for _, savedID := range savedConfigIDs {
				if deleteErr := uc.configStore.Delete(ctx, savedID.String()); deleteErr != nil {
					log.Warn("Failed to rollback config during error recovery",
						"config_id", savedID,
						"rollback_error", deleteErr,
					)
				}
			}
			return fmt.Errorf("failed to save child config %s after transaction (rolled back %d configs): %w",
				c.cfg.ID, len(savedConfigIDs), err)
		}
		savedConfigIDs = append(savedConfigIDs, c.id)
	}

	return nil
}

// processChildConfig processes a child task config to create its partial state
func (uc *CreateChildTasks) processChildConfig(
	childConfig *task.Config,
	parentEnv *core.EnvMap,
) (*task.PartialState, error) {
	// Merge parent environment with child environment
	mergedEnv := uc.mergeEnvironments(parentEnv, childConfig.Env)

	executionType := childConfig.GetExecType()
	agentConfig := childConfig.GetAgent()
	toolConfig := childConfig.GetTool()

	switch {
	case childConfig.Type == task.TaskTypeParallel ||
		childConfig.Type == task.TaskTypeCollection ||
		childConfig.Type == task.TaskTypeComposite:
		// Container tasks - create basic state for tracking, actual execution handled by executeChild
		return &task.PartialState{
			Component:     core.ComponentTask,
			ExecutionType: executionType,
			Input:         childConfig.With,
			MergedEnv:     mergedEnv,
		}, nil
	case agentConfig != nil:
		return uc.processAgent(agentConfig, executionType, childConfig.Action, childConfig.With, parentEnv)
	case toolConfig != nil:
		return uc.processTool(toolConfig, executionType, childConfig.With, parentEnv)
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
			MergedEnv:     mergedEnv,
		}, nil
	}
}

func (uc *CreateChildTasks) processAgent(
	agentConfig *agent.Config,
	executionType task.ExecutionType,
	actionID string,
	childInput *core.Input,
	parentEnv *core.EnvMap,
) (*task.PartialState, error) {
	agentID := agentConfig.ID
	// Use childInput if provided (for collection children), otherwise use agent's With
	input := childInput
	if input == nil {
		input = agentConfig.With
	}
	// Merge parent environment with agent environment
	mergedEnv := uc.mergeEnvironments(parentEnv, agentConfig.Env)
	return &task.PartialState{
		Component:     core.ComponentAgent,
		ExecutionType: executionType,
		AgentID:       &agentID,
		ActionID:      &actionID,
		Input:         input,
		MergedEnv:     mergedEnv,
	}, nil
}

func (uc *CreateChildTasks) processTool(
	toolConfig *tool.Config,
	executionType task.ExecutionType,
	childInput *core.Input,
	parentEnv *core.EnvMap,
) (*task.PartialState, error) {
	toolID := toolConfig.ID
	// Use childInput if provided (for collection children), otherwise use tool's With
	input := childInput
	if input == nil {
		input = toolConfig.With
	}
	// Merge parent environment with tool environment
	mergedEnv := uc.mergeEnvironments(parentEnv, toolConfig.Env)
	return &task.PartialState{
		Component:     core.ComponentTool,
		ExecutionType: executionType,
		ToolID:        &toolID,
		Input:         input,
		MergedEnv:     mergedEnv,
	}, nil
}

// mergeEnvironments merges parent and child environment variables
// Child environment variables take precedence over parent ones
func (uc *CreateChildTasks) mergeEnvironments(parentEnv, childEnv *core.EnvMap) *core.EnvMap {
	if parentEnv == nil && childEnv == nil {
		return nil
	}
	// Start with a new empty map
	merged := make(core.EnvMap)
	// Copy parent environment variables first
	if parentEnv != nil {
		for k, v := range *parentEnv {
			merged[k] = v
		}
	}
	// Override with child environment variables
	if childEnv != nil {
		for k, v := range *childEnv {
			merged[k] = v
		}
	}
	// Return nil if the map is empty
	if len(merged) == 0 {
		return nil
	}
	return &merged
}
