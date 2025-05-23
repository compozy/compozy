package agent

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

func RandomMetadata(workflowID string, taskID string, agentID string) *pb.AgentMetadata {
	corrID := common.MustNewID()
	wExecID := common.MustNewID()
	tExecID := common.MustNewID()
	agentExecID := common.MustNewID()
	wStateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)
	taskStateID := state.NewID(nats.ComponentTask, corrID, tExecID)
	agentStateID := state.NewID(nats.ComponentAgent, corrID, agentExecID)
	return &pb.AgentMetadata{
		Time:            timepb.Now(),
		Source:          "",
		CorrelationId:   corrID.String(),
		WorkflowId:      workflowID,
		WorkflowExecId:  wExecID.String(),
		WorkflowStateId: wStateID.String(),
		TaskId:          taskID,
		TaskExecId:      tExecID.String(),
		TaskStateId:     taskStateID.String(),
		AgentId:         agentID,
		AgentExecId:     agentExecID.String(),
		AgentStateId:    agentStateID.String(),
		Subject:         "",
	}
}

func GetAgentStateID(metadata *pb.AgentMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	agentExecID := common.ID(metadata.AgentExecId)
	return state.NewID(nats.ComponentAgent, corrID, agentExecID)
}

func GetTaskStateID(metadata *pb.AgentMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	taskExecID := common.ID(metadata.TaskExecId)
	return state.NewID(nats.ComponentTask, corrID, taskExecID)
}

func GetWorkflowStateID(metadata *pb.AgentMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	workflowExecID := common.ID(metadata.WorkflowExecId)
	return state.NewID(nats.ComponentWorkflow, corrID, workflowExecID)
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

type Context struct {
	Metadata     *pb.AgentMetadata `json:"metadata"`
	TaskEnv      common.EnvMap     `json:"task_env"`
	AgentEnv     common.EnvMap     `json:"agent_env"`
	TriggerInput *common.Input     `json:"trigger_input"`
	TaskInput    *common.Input     `json:"task_input"`
	AgentInput   *common.Input     `json:"agent_input"`
}

func NewContext(
	metadata *pb.AgentMetadata,
	taskEnv, agentEnv common.EnvMap,
	triggerInput, taskInput, agentInput *common.Input,
) (*Context, error) {
	return &Context{
		Metadata:     metadata,
		TaskEnv:      taskEnv,
		AgentEnv:     agentEnv,
		TriggerInput: triggerInput,
		TaskInput:    taskInput,
		AgentInput:   agentInput,
	}, nil
}

func (c *Context) GetMetadata() *pb.AgentMetadata {
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

func (c *Context) GetAgentID() string {
	return c.Metadata.AgentId
}

func (c *Context) GetAgentExecID() common.ID {
	return common.ID(c.Metadata.AgentExecId)
}

func (c *Context) GetAgentStateID() state.ID {
	return GetAgentStateID(c.Metadata)
}

func (c *Context) GetSubject() string {
	return c.Metadata.Subject
}

func (c *Context) GetTaskEnv() common.EnvMap {
	return c.TaskEnv
}

func (c *Context) GetAgentEnv() common.EnvMap {
	return c.AgentEnv
}

func (c *Context) GetTriggerInput() *common.Input {
	return c.TriggerInput
}

func (c *Context) GetTaskInput() *common.Input {
	return c.TaskInput
}

func (c *Context) GetAgentInput() *common.Input {
	return c.AgentInput
}
