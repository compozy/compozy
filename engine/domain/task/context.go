package task

import (
	"fmt"

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
}

type Payload interface {
	GetMetadata() *pbcommon.Metadata
	GetWorkflow() *pbcommon.WorkflowInfo
	GetTask() *pbcommon.TaskInfo
}

func InfoFromEvent(cmd Payload) (*Metadata, error) {
	workflowInfo := cmd.GetWorkflow()
	taskInfo := cmd.GetTask()
	metadata := cmd.GetMetadata()
	corrID := common.ID(metadata.CorrelationId)
	workflowID := workflowInfo.Id
	wExecID := common.ID(workflowInfo.ExecId)
	taskID := taskInfo.Id
	taskExecID := common.ID(taskInfo.ExecId)

	wStateID, err := state.IDFromString(*metadata.State.ParentId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow state ID: %w", err)
	}

	tStateID, err := state.IDFromString(metadata.State.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to parse task state ID: %w", err)
	}

	return &Metadata{
		Time:            metadata.Time,
		Source:          metadata.Source,
		CorrID:          corrID,
		WorkflowID:      workflowID,
		WorkflowExecID:  wExecID,
		WorkflowStateID: wStateID,
		TaskID:          taskID,
		TaskExecID:      taskExecID,
		TaskStateID:     tStateID,
	}, nil
}

func RandomInfo(workflowID string, taskID string) *Metadata {
	corrID := common.MustNewID()
	execID := common.MustNewID()
	return &Metadata{
		Time:            timepb.Now(),
		Source:          "",
		CorrID:          corrID,
		WorkflowID:      workflowID,
		WorkflowExecID:  execID,
		WorkflowStateID: state.NewID(nats.ComponentWorkflow, corrID, execID),
		TaskID:          taskID,
		TaskExecID:      execID,
		TaskStateID:     state.NewID(nats.ComponentTask, corrID, execID),
	}
}

func (i *Metadata) Metadata() *pbcommon.Metadata {
	wStateID := i.WorkflowStateID.String()
	return &pbcommon.Metadata{
		CorrelationId: i.CorrID.String(),
		Source:        i.Source,
		Time:          i.Time,
		State: &pbcommon.State{
			Id:       i.TaskStateID.String(),
			ParentId: &wStateID,
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

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

type Context struct {
	*Metadata
	WorkflowEnv  common.EnvMap `json:"workflow_env"`
	TaskEnv      common.EnvMap `json:"task_env"`
	TriggerInput *common.Input `json:"trigger_input"`
	TaskInput    *common.Input `json:"task_input"`
}

func NewContext(
	info *Metadata,
	workflowEnv, taskEnv common.EnvMap,
	triggerInput, taskInput *common.Input,
) (*Context, error) {
	return &Context{
		Metadata:     info,
		WorkflowEnv:  workflowEnv,
		TaskEnv:      taskEnv,
		TriggerInput: triggerInput,
		TaskInput:    taskInput,
	}, nil
}
