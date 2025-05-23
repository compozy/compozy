package workflow

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

func RandomMetadata(workflowID string) *pb.WorkflowMetadata {
	corrID := common.MustNewID()
	wExecID := common.MustNewID()
	wStateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)
	return &pb.WorkflowMetadata{
		Time:            timepb.Now(),
		Source:          "",
		CorrelationId:   corrID.String(),
		WorkflowId:      workflowID,
		WorkflowExecId:  wExecID.String(),
		WorkflowStateId: wStateID.String(),
		Subject:         "",
	}
}

// -----------------------------------------------------------------------------
// Context helper functions
// -----------------------------------------------------------------------------

func GetWorkflowStateID(metadata *pb.WorkflowMetadata) state.ID {
	corrID := common.ID(metadata.CorrelationId)
	workflowExecID := common.ID(metadata.WorkflowExecId)
	return state.NewID(nats.ComponentWorkflow, corrID, workflowExecID)
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

type Context struct {
	Metadata     *pb.WorkflowMetadata `json:"metadata"`
	TriggerInput *common.Input        `json:"trigger_input"`
	ProjectEnv   common.EnvMap        `json:"project_env"`
	WorkflowEnv  common.EnvMap        `json:"workflow_env"`
}

func NewContext(
	metadata *pb.WorkflowMetadata,
	triggerInput *common.Input,
	projectEnv, workflowEnv common.EnvMap,
) (*Context, error) {
	return &Context{
		Metadata:     metadata,
		TriggerInput: triggerInput,
		ProjectEnv:   projectEnv,
		WorkflowEnv:  workflowEnv,
	}, nil
}

// -----------------------------------------------------------------------------
// Basic getters
// -----------------------------------------------------------------------------

func (c *Context) GetMetadata() *pb.WorkflowMetadata {
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

func (c *Context) GetSubject() string {
	return c.Metadata.Subject
}

func (c *Context) GetTriggerInput() *common.Input {
	return c.TriggerInput
}

func (c *Context) GetProjectEnv() common.EnvMap {
	return c.ProjectEnv
}

func (c *Context) GetWorkflowEnv() common.EnvMap {
	return c.WorkflowEnv
}
