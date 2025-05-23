package task

import (
	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

// -----------------------------------------------------------------------------
// Helper functions
// -----------------------------------------------------------------------------

func RandomMetadata(workflowID string, taskID string) *pb.TaskMetadata {
	corrID := common.MustNewID()
	wExecID := common.MustNewID()
	tExecID := common.MustNewID()
	wStateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)
	taskStateID := state.NewID(nats.ComponentTask, corrID, tExecID)
	return &pb.TaskMetadata{
		Time:            timepb.Now(),
		Source:          "",
		CorrelationId:   corrID.String(),
		WorkflowId:      workflowID,
		WorkflowExecId:  wExecID.String(),
		WorkflowStateId: wStateID.String(),
		TaskId:          taskID,
		TaskExecId:      tExecID.String(),
		TaskStateId:     taskStateID.String(),
		Subject:         "",
	}
}

func GetTaskStateID(metadata *pb.TaskMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	taskExecID := common.ID(metadata.TaskExecId)
	return state.NewID(nats.ComponentTask, corrID, taskExecID)
}

func GetWorkflowStateID(metadata *pb.TaskMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	workflowExecID := common.ID(metadata.WorkflowExecId)
	return state.NewID(nats.ComponentWorkflow, corrID, workflowExecID)
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

type Context struct {
	Metadata     *pb.TaskMetadata `json:"metadata"`
	WorkflowEnv  common.EnvMap    `json:"workflow_env"`
	TaskEnv      common.EnvMap    `json:"task_env"`
	TriggerInput *common.Input    `json:"trigger_input"`
	TaskInput    *common.Input    `json:"task_input"`
}

func NewContext(
	metadata *pb.TaskMetadata,
	workflowEnv, taskEnv common.EnvMap,
	triggerInput, taskInput *common.Input,
) (*Context, error) {
	return &Context{
		Metadata:     metadata,
		WorkflowEnv:  workflowEnv,
		TaskEnv:      taskEnv,
		TriggerInput: triggerInput,
		TaskInput:    taskInput,
	}, nil
}

// -----------------------------------------------------------------------------
// Basic getters
// -----------------------------------------------------------------------------

func (c *Context) GetMetadata() *pb.TaskMetadata {
	return c.Metadata
}

func (c *Context) GetSource() string {
	return c.Metadata.Source
}

func (c *Context) GetCorrID() common.ID {
	return common.ID(c.Metadata.CorrelationId)
}

func (c *Context) GetWorkflowID() string {
	return c.Metadata.WorkflowId
}

func (c *Context) GetWorkflowExecID() common.ID {
	return common.ID(c.Metadata.WorkflowExecId)
}

func (c *Context) GetWorkflowStateID() state.ID {
	return GetWorkflowStateID(c.Metadata)
}

func (c *Context) GetTaskID() string {
	return c.Metadata.TaskId
}

func (c *Context) GetTaskExecID() common.ID {
	return common.ID(c.Metadata.TaskExecId)
}

func (c *Context) GetTaskStateID() state.ID {
	return GetTaskStateID(c.Metadata)
}

func (c *Context) GetSubject() string {
	return c.Metadata.Subject
}

func (c *Context) GetWorkflowEnv() common.EnvMap {
	return c.WorkflowEnv
}

func (c *Context) GetTaskEnv() common.EnvMap {
	return c.TaskEnv
}

func (c *Context) GetTriggerInput() *common.Input {
	return c.TriggerInput
}

func (c *Context) GetTaskInput() *common.Input {
	return c.TaskInput
}
