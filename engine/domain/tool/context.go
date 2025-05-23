package tool

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

func RandomMetadata(workflowID string, taskID string, toolID string) *pb.ToolMetadata {
	corrID := common.MustNewID()
	wExecID := common.MustNewID()
	tExecID := common.MustNewID()
	toolExecID := common.MustNewID()
	wStateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)
	taskStateID := state.NewID(nats.ComponentTask, corrID, tExecID)
	toolStateID := state.NewID(nats.ComponentTool, corrID, toolExecID)
	return &pb.ToolMetadata{
		Time:            timepb.Now(),
		Source:          "",
		CorrelationId:   corrID.String(),
		WorkflowId:      workflowID,
		WorkflowExecId:  wExecID.String(),
		WorkflowStateId: wStateID.String(),
		TaskId:          taskID,
		TaskExecId:      tExecID.String(),
		TaskStateId:     taskStateID.String(),
		ToolId:          toolID,
		ToolExecId:      toolExecID.String(),
		ToolStateId:     toolStateID.String(),
		Subject:         "",
	}
}

// -----------------------------------------------------------------------------
// Context helper functions
// -----------------------------------------------------------------------------

func GetToolStateID(metadata *pb.ToolMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	toolExecID := common.ID(metadata.ToolExecId)
	return state.NewID(nats.ComponentTool, corrID, toolExecID)
}

func GetTaskStateID(metadata *pb.ToolMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	taskExecID := common.ID(metadata.TaskExecId)
	return state.NewID(nats.ComponentTask, corrID, taskExecID)
}

func GetWorkflowStateID(metadata *pb.ToolMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	workflowExecID := common.ID(metadata.WorkflowExecId)
	return state.NewID(nats.ComponentWorkflow, corrID, workflowExecID)
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

type Context struct {
	Metadata     *pb.ToolMetadata `json:"metadata"`
	TaskEnv      common.EnvMap    `json:"task_env"`
	ToolEnv      common.EnvMap    `json:"tool_env"`
	TriggerInput *common.Input    `json:"trigger_input"`
	TaskInput    *common.Input    `json:"task_input"`
	ToolInput    *common.Input    `json:"tool_input"`
}

func NewContext(
	metadata *pb.ToolMetadata,
	taskEnv, toolEnv common.EnvMap,
	triggerInput, taskInput, toolInput *common.Input,
) (*Context, error) {
	return &Context{
		Metadata:     metadata,
		TaskEnv:      taskEnv,
		ToolEnv:      toolEnv,
		TriggerInput: triggerInput,
		TaskInput:    taskInput,
		ToolInput:    toolInput,
	}, nil
}

// -----------------------------------------------------------------------------
// Basic getters
// -----------------------------------------------------------------------------

func (c *Context) GetMetadata() *pb.ToolMetadata {
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

func (c *Context) GetToolID() string {
	return c.Metadata.ToolId
}

func (c *Context) GetToolExecID() common.ID {
	return common.ID(c.Metadata.ToolExecId)
}

func (c *Context) GetToolStateID() state.ID {
	return GetToolStateID(c.Metadata)
}

func (c *Context) GetSubject() string {
	return c.Metadata.Subject
}

func (c *Context) GetTaskEnv() common.EnvMap {
	return c.TaskEnv
}

func (c *Context) GetToolEnv() common.EnvMap {
	return c.ToolEnv
}

func (c *Context) GetTriggerInput() *common.Input {
	return c.TriggerInput
}

func (c *Context) GetTaskInput() *common.Input {
	return c.TaskInput
}

func (c *Context) GetToolInput() *common.Input {
	return c.ToolInput
}
