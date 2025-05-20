package state

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/tplengine"
)

// -----------------------------------------------------------------------------
// Initializer Interface
// -----------------------------------------------------------------------------

type Initializer interface {
	Initialize() (State, error)
	MergeEnv(parentEnv common.EnvMap, componentEnv common.EnvMap) (*common.EnvMap, error)
}

// -----------------------------------------------------------------------------
// Common Initializer Implementation
// -----------------------------------------------------------------------------

type CommonInitializer struct {
	Normalizer Normalizer
}

func NewCommonInitializer() *CommonInitializer {
	return &CommonInitializer{
		Normalizer: NewStateNormalizer(tplengine.FormatYAML),
	}
}

func (ci *CommonInitializer) MergeEnv(parentEnv common.EnvMap, componentEnv common.EnvMap) (*common.EnvMap, error) {
	result := make(common.EnvMap)
	result, err := result.Merge(parentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge parent env: %w", err)
	}
	result, err = result.Merge(componentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge component env: %w", err)
	}
	return &result, nil
}

// -----------------------------------------------------------------------------
// Workflow State Initializer
// -----------------------------------------------------------------------------

type WorkflowStateInitializer struct {
	*CommonInitializer
	WorkflowID   string
	ExecID       string
	TriggerInput common.Input
	ProjectEnv   common.EnvMap
	WorkflowEnv  common.EnvMap
}

func (wi *WorkflowStateInitializer) Initialize() (State, error) {
	env, err := wi.MergeEnv(wi.ProjectEnv, wi.WorkflowEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	state := &BaseState{
		ID:      NewID(nats.ComponentWorkflow, wi.WorkflowID, wi.ExecID),
		Status:  nats.StatusPending,
		Input:   make(common.Input),
		Output:  make(common.Output),
		Trigger: wi.TriggerInput,
		Env:     *env,
	}
	if err := wi.Normalizer.ParseTemplates(state); err != nil {
		return nil, err
	}
	return state, nil
}

// -----------------------------------------------------------------------------
// Task State Initializer
// -----------------------------------------------------------------------------

type TaskStateInitializer struct {
	*CommonInitializer
	TaskID         string
	ExecID         string
	WorkflowExecID string
	TriggerInput   common.Input
	WorkflowEnv    common.EnvMap
	TaskEnv        common.EnvMap
}

func (ti *TaskStateInitializer) Initialize() (State, error) {
	env, err := ti.MergeEnv(ti.WorkflowEnv, ti.TaskEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	state := &BaseState{
		ID:      NewID(nats.ComponentTask, ti.TaskID, ti.ExecID),
		Status:  nats.StatusPending,
		Input:   make(common.Input),
		Output:  make(common.Output),
		Trigger: ti.TriggerInput,
		Env:     *env,
	}
	if err := ti.Normalizer.ParseTemplates(state); err != nil {
		return nil, err
	}
	return state, nil
}

// -----------------------------------------------------------------------------
// Agent State Initializer
// -----------------------------------------------------------------------------

type AgentStateInitializer struct {
	*CommonInitializer
	AgentID        string
	ExecID         string
	TaskExecID     string
	WorkflowExecID string
	TriggerInput   common.Input
	TaskEnv        common.EnvMap
	AgentEnv       common.EnvMap
}

func (ai *AgentStateInitializer) Initialize() (State, error) {
	env, err := ai.MergeEnv(ai.TaskEnv, ai.AgentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	state := &BaseState{
		ID:      NewID(nats.ComponentAgent, ai.AgentID, ai.ExecID),
		Status:  nats.StatusPending,
		Input:   make(common.Input),
		Output:  make(common.Output),
		Trigger: ai.TriggerInput,
		Env:     *env,
	}
	if err := ai.Normalizer.ParseTemplates(state); err != nil {
		return nil, err
	}
	return state, nil
}

// -----------------------------------------------------------------------------
// Tool State Initializer
// -----------------------------------------------------------------------------

type ToolStateInitializer struct {
	*CommonInitializer
	ToolID         string
	ExecID         string
	TaskExecID     string
	WorkflowExecID string
	TriggerInput   common.Input
	TaskEnv        common.EnvMap
	ToolEnv        common.EnvMap
}

func (ti *ToolStateInitializer) Initialize() (State, error) {
	env, err := ti.MergeEnv(ti.TaskEnv, ti.ToolEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}

	state := &BaseState{
		ID:      NewID(nats.ComponentTool, ti.ToolID, ti.ExecID),
		Status:  nats.StatusPending,
		Input:   make(common.Input),
		Output:  make(common.Output),
		Env:     *env,
		Trigger: ti.TriggerInput,
	}
	if err := ti.Normalizer.ParseTemplates(state); err != nil {
		return nil, err
	}
	return state, nil
}

// -----------------------------------------------------------------------------
// State Type Aliases
// -----------------------------------------------------------------------------

// TaskState is a type alias for BaseState with WorkflowExecID
type TaskState struct {
	BaseState
	WorkflowExecID string `json:"workflow_exec_id"`
}

// AgentState is a type alias for BaseState with TaskExecID and WorkflowExecID
type AgentState struct {
	BaseState
	TaskExecID     string `json:"task_exec_id"`
	WorkflowExecID string `json:"workflow_exec_id"`
}

// ToolState is a type alias for BaseState with TaskExecID and WorkflowExecID
type ToolState struct {
	BaseState
	TaskExecID     string `json:"task_exec_id"`
	WorkflowExecID string `json:"workflow_exec_id"`
}
