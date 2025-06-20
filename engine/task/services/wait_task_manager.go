package services

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

// WaitTaskManager manages wait task lifecycle and signal processing
type WaitTaskManager struct {
	taskRepo      task.Repository
	configStore   ConfigStore
	taskResponder *TaskResponder
	parentUpdater *ParentStatusUpdater
}

// NewWaitTaskManager creates a new WaitTaskManager
func NewWaitTaskManager(
	taskRepo task.Repository,
	configStore ConfigStore,
	taskResponder *TaskResponder,
	parentUpdater *ParentStatusUpdater,
) *WaitTaskManager {
	return &WaitTaskManager{
		taskRepo:      taskRepo,
		configStore:   configStore,
		taskResponder: taskResponder,
		parentUpdater: parentUpdater,
	}
}

// UpdateWaitTaskStatus updates the status of a wait task
func (m *WaitTaskManager) UpdateWaitTaskStatus(
	ctx context.Context,
	taskExecID core.ID,
	status core.StatusType,
	output *core.Output,
) error {
	log := logger.FromContext(ctx)
	// Get current task state
	taskState, err := m.taskRepo.GetState(ctx, taskExecID)
	if err != nil {
		return fmt.Errorf("failed to get task state: %w", err)
	}
	// Create a copy to avoid race conditions if the repo stores pointers
	updatedState := &task.State{
		Component:      taskState.Component,
		Status:         status, // Update status
		TaskID:         taskState.TaskID,
		TaskExecID:     taskState.TaskExecID,
		WorkflowID:     taskState.WorkflowID,
		WorkflowExecID: taskState.WorkflowExecID,
		ParentStateID:  taskState.ParentStateID,
		ExecutionType:  taskState.ExecutionType,
		AgentID:        taskState.AgentID,
		ActionID:       taskState.ActionID,
		ToolID:         taskState.ToolID,
		Input:          taskState.Input,
		Output:         taskState.Output,
		Error:          taskState.Error,
		CreatedAt:      taskState.CreatedAt,
		UpdatedAt:      time.Now(),
	}
	// Update output if provided
	if output != nil {
		updatedState.Output = output
	}
	// Persist updated state
	if err := m.taskRepo.UpsertState(ctx, updatedState); err != nil {
		return fmt.Errorf("failed to update task state: %w", err)
	}
	// Update parent if this is a child task
	if updatedState.ParentStateID != nil {
		log.Debug("Updating parent state after wait task status change",
			"parentStateID", updatedState.ParentStateID,
			"childStatus", status)
		// Update parent status after child status change
		if _, err := m.parentUpdater.UpdateParentStatus(ctx, &UpdateParentStatusInput{
			ParentStateID: *updatedState.ParentStateID,
			Strategy:      task.StrategyWaitAll, // Default strategy for wait tasks
			Recursive:     true,
			ChildState:    updatedState,
		}); err != nil {
			log.Error("Failed to update parent state",
				"parentStateID", updatedState.ParentStateID,
				"error", err)
			// Don't fail the operation, parent update is best-effort
		}
	}
	log.Debug("Updated wait task status",
		"taskID", taskState.TaskID,
		"taskExecID", taskExecID,
		"status", status)
	return nil
}

// PrepareWaitTaskResponse prepares the response for a wait task
func (m *WaitTaskManager) PrepareWaitTaskResponse(
	ctx context.Context,
	taskState *task.State,
	workflowConfig any, // Using interface{} to avoid circular dependency with workflow package
) (*task.MainTaskResponse, error) {
	// Note: workflowConfig must be *workflow.Config - we use interface{} to prevent import cycle
	// Load task config
	taskConfig, err := m.configStore.Get(ctx, taskState.TaskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to load task config: %w", err)
	}
	// Type assert the workflow config
	wfConfig, ok := workflowConfig.(*workflow.Config)
	if !ok {
		return nil, fmt.Errorf("invalid workflow config type")
	}
	// Use task responder to prepare the response
	response, err := m.taskResponder.HandleMainTask(ctx, &MainTaskResponseInput{
		WorkflowConfig: wfConfig,
		TaskState:      taskState,
		TaskConfig:     taskConfig,
		ExecutionError: nil, // Wait tasks handle their own errors
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare response: %w", err)
	}
	return response, nil
}

// ValidateWaitTaskSignal validates if a signal can be processed by a wait task
func (m *WaitTaskManager) ValidateWaitTaskSignal(
	ctx context.Context,
	taskExecID core.ID,
	signalName string,
) error {
	// Get task state
	taskState, err := m.taskRepo.GetState(ctx, taskExecID)
	if err != nil {
		return fmt.Errorf("failed to get task state: %w", err)
	}
	// Check task status - wait tasks can be in PENDING, RUNNING, or WAITING state
	if taskState.Status != core.StatusPending &&
		taskState.Status != core.StatusRunning &&
		taskState.Status != core.StatusWaiting {
		return fmt.Errorf("task is not waiting for signals (status: %s)", taskState.Status)
	}
	// Load task config
	taskConfig, err := m.configStore.Get(ctx, taskState.TaskExecID.String())
	if err != nil {
		return fmt.Errorf("failed to load task config: %w", err)
	}
	// Validate task type
	if taskConfig.Type != task.TaskTypeWait {
		return fmt.Errorf("task is not a wait task (type: %s)", taskConfig.Type)
	}
	// Validate signal name
	if taskConfig.WaitFor != signalName {
		return fmt.Errorf("task is waiting for signal '%s', not '%s'", taskConfig.WaitFor, signalName)
	}
	return nil
}
