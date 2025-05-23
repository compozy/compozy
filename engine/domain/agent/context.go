package agent

import (
	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

type Metadata struct {
	Time            *timepb.Timestamp
	Source          string
	CorrID          common.ID
	WorkflowID      string
	WorkflowExecID  common.ID
	WorkflowStateID state.ID
	TaskID          string
	TaskExecID      common.ID
	TaskStateID     state.ID
	AgentID         string
	AgentExecID     common.ID
	AgentStateID    state.ID
}

type Payload interface {
	GetMetadata() *pbcommon.Metadata
	GetWorkflow() *pbcommon.WorkflowInfo
	GetTask() *pbcommon.TaskInfo
	GetAgent() *pbcommon.AgentInfo
}

func InfoFromEvent(cmd Payload) (*Metadata, error) {
	workflowInfo := cmd.GetWorkflow()
	taskInfo := cmd.GetTask()
	agentInfo := cmd.GetAgent()
	metadata := cmd.GetMetadata()
	corrID := common.ID(metadata.CorrelationId)
	workflowID := workflowInfo.Id
	wExecID := common.ID(workflowInfo.ExecId)
	taskID := taskInfo.Id
	taskExecID := common.ID(taskInfo.ExecId)
	agentID := agentInfo.Id
	agentExecID := common.ID(agentInfo.ExecId)
	wStateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)
	taskStateID := state.NewID(nats.ComponentTask, corrID, taskExecID)
	agentStateID := state.NewID(nats.ComponentAgent, corrID, agentExecID)
	return &Metadata{
		Time:            metadata.Time,
		Source:          metadata.Source,
		CorrID:          corrID,
		WorkflowID:      workflowID,
		WorkflowExecID:  wExecID,
		WorkflowStateID: wStateID,
		TaskID:          taskID,
		TaskExecID:      taskExecID,
		TaskStateID:     taskStateID,
		AgentID:         agentID,
		AgentExecID:     agentExecID,
		AgentStateID:    agentStateID,
	}, nil
}

func RandomInfo(workflowID string, taskID string, agentID string) *Metadata {
	corrID := common.MustNewID()
	wExecID := common.MustNewID()
	tExecID := common.MustNewID()
	agentExecID := common.MustNewID()
	return &Metadata{
		Time:            timepb.Now(),
		Source:          "",
		CorrID:          corrID,
		WorkflowID:      workflowID,
		WorkflowExecID:  wExecID,
		WorkflowStateID: state.NewID(nats.ComponentWorkflow, corrID, wExecID),
		TaskID:          taskID,
		TaskExecID:      tExecID,
		TaskStateID:     state.NewID(nats.ComponentTask, corrID, tExecID),
		AgentID:         agentID,
		AgentExecID:     agentExecID,
		AgentStateID:    state.NewID(nats.ComponentAgent, corrID, agentExecID),
	}
}

func (i *Metadata) Metadata() *pbcommon.Metadata {
	tStateID := i.TaskStateID.String()
	return &pbcommon.Metadata{
		CorrelationId: i.CorrID.String(),
		Source:        i.Source,
		Time:          i.Time,
		State: &pbcommon.State{
			Id:       i.AgentStateID.String(),
			ParentId: &tStateID,
		},
	}
}

func (i *Metadata) Workflow() *pbcommon.WorkflowInfo {
	return &pbcommon.WorkflowInfo{
		Id:     i.WorkflowID,
		ExecId: i.WorkflowExecID.String(),
	}
}

func (i *Metadata) Task() *pbcommon.TaskInfo {
	return &pbcommon.TaskInfo{
		Id:     i.TaskID,
		ExecId: i.TaskExecID.String(),
	}
}

func (i *Metadata) Agent() *pbcommon.AgentInfo {
	return &pbcommon.AgentInfo{
		Id:     i.AgentID,
		ExecId: i.AgentExecID.String(),
	}
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

type Context struct {
	*Metadata
	TaskEnv      common.EnvMap `json:"task_env"`
	AgentEnv     common.EnvMap `json:"agent_env"`
	TriggerInput *common.Input `json:"trigger_input"`
	TaskInput    *common.Input `json:"task_input"`
	AgentInput   *common.Input `json:"agent_input"`
}

func NewContext(
	info *Metadata,
	taskEnv, agentEnv common.EnvMap,
	triggerInput, taskInput, agentInput *common.Input,
) (*Context, error) {
	return &Context{
		Metadata:     info,
		TaskEnv:      taskEnv,
		AgentEnv:     agentEnv,
		TriggerInput: triggerInput,
		TaskInput:    taskInput,
		AgentInput:   agentInput,
	}, nil
}
