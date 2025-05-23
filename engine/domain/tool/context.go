package tool

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
	ToolID          string
	ToolExecID      common.ID
	ToolStateID     state.ID
}

type Payload interface {
	GetMetadata() *pbcommon.Metadata
	GetWorkflow() *pbcommon.WorkflowInfo
	GetTask() *pbcommon.TaskInfo
	GetTool() *pbcommon.ToolInfo
}

func InfoFromEvent(cmd Payload) (*Metadata, error) {
	workflowInfo := cmd.GetWorkflow()
	taskInfo := cmd.GetTask()
	toolInfo := cmd.GetTool()
	metadata := cmd.GetMetadata()
	corrID := common.ID(metadata.CorrelationId)
	workflowID := workflowInfo.Id
	wExecID := common.ID(workflowInfo.ExecId)
	taskID := taskInfo.Id
	taskExecID := common.ID(taskInfo.ExecId)
	toolID := toolInfo.Id
	toolExecID := common.ID(toolInfo.ExecId)
	wStateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)
	taskStateID := state.NewID(nats.ComponentTask, corrID, taskExecID)
	toolStateID := state.NewID(nats.ComponentTool, corrID, toolExecID)
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
		ToolID:          toolID,
		ToolExecID:      toolExecID,
		ToolStateID:     toolStateID,
	}, nil
}

func RandomInfo(workflowID string, taskID string, toolID string) *Metadata {
	corrID := common.MustNewID()
	wExecID := common.MustNewID()
	tExecID := common.MustNewID()
	toolExecID := common.MustNewID()
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
		ToolID:          toolID,
		ToolExecID:      toolExecID,
		ToolStateID:     state.NewID(nats.ComponentTool, corrID, toolExecID),
	}
}

func (i *Metadata) Metadata() *pbcommon.Metadata {
	tStateID := i.TaskStateID.String()
	return &pbcommon.Metadata{
		CorrelationId: i.CorrID.String(),
		Source:        i.Source,
		Time:          i.Time,
		State: &pbcommon.State{
			Id:       i.ToolStateID.String(),
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

func (i *Metadata) Tool() *pbcommon.ToolInfo {
	return &pbcommon.ToolInfo{
		Id:     i.ToolID,
		ExecId: i.ToolExecID.String(),
	}
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

type Context struct {
	*Metadata
	TaskEnv      common.EnvMap `json:"task_env"`
	ToolEnv      common.EnvMap `json:"tool_env"`
	TriggerInput *common.Input `json:"trigger_input"`
	TaskInput    *common.Input `json:"task_input"`
	ToolInput    *common.Input `json:"tool_input"`
}

func NewContext(
	info *Metadata,
	taskEnv, toolEnv common.EnvMap,
	triggerInput, taskInput, toolInput *common.Input,
) (*Context, error) {
	return &Context{
		Metadata:     info,
		TaskEnv:      taskEnv,
		ToolEnv:      toolEnv,
		TriggerInput: triggerInput,
		TaskInput:    taskInput,
		ToolInput:    toolInput,
	}, nil
}
