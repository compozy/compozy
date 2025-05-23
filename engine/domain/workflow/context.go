package workflow

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
}

type Payload interface {
	GetMetadata() *pbcommon.Metadata
	GetWorkflow() *pbcommon.WorkflowInfo
}

func InfoFromEvent(cmd Payload) (*Metadata, error) {
	workflowInfo := cmd.GetWorkflow()
	metadata := cmd.GetMetadata()
	corrID := common.ID(metadata.CorrelationId)
	workflowID := workflowInfo.Id
	wExecID := common.ID(workflowInfo.ExecId)
	wStateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)
	return &Metadata{
		Time:            metadata.Time,
		Source:          metadata.Source,
		CorrID:          corrID,
		WorkflowID:      workflowID,
		WorkflowExecID:  wExecID,
		WorkflowStateID: wStateID,
	}, nil
}

func RandomInfo(workflowID string) *Metadata {
	corrID := common.MustNewID()
	wExecID := common.MustNewID()
	return &Metadata{
		Time:            timepb.Now(),
		Source:          "",
		CorrID:          corrID,
		WorkflowID:      workflowID,
		WorkflowExecID:  wExecID,
		WorkflowStateID: state.NewID(nats.ComponentWorkflow, corrID, wExecID),
	}
}

func (i *Metadata) Metadata() *pbcommon.Metadata {
	return &pbcommon.Metadata{
		CorrelationId: i.CorrID.String(),
		Source:        i.Source,
		Time:          i.Time,
		State: &pbcommon.State{
			Id:       i.WorkflowStateID.String(),
			ParentId: nil, // Workflow has no parent
		},
	}
}

func (i *Metadata) Workflow() *pbcommon.WorkflowInfo {
	return &pbcommon.WorkflowInfo{
		Id:     i.WorkflowID,
		ExecId: i.WorkflowExecID.String(),
	}
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

type Context struct {
	*Metadata
	TriggerInput *common.Input `json:"trigger_input"`
	ProjectEnv   common.EnvMap `json:"project_env"`
	WorkflowEnv  common.EnvMap `json:"workflow_env"`
}

func NewContext(
	info *Metadata,
	triggerInput *common.Input,
	projectEnv, workflowEnv common.EnvMap,
) (*Context, error) {
	return &Context{
		Metadata:     info,
		TriggerInput: triggerInput,
		ProjectEnv:   projectEnv,
		WorkflowEnv:  workflowEnv,
	}, nil
}
