package events

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TriggerResponse struct {
	WorkflowID     string `json:"workflow_id"`
	WorkflowExecID string `json:"execution_id"`
}

type CmdTrigger struct {
	*pb.CmdWorkflowTrigger
	nc       *nats.Client
	Response *TriggerResponse
}

func NewCmdTrigger(nc *nats.Client, input *structpb.Struct, workflowID string) *CmdTrigger {
	workflowExecID := core.MustNewID()
	cmd := &pb.CmdWorkflowTrigger{
		Metadata: &pb.WorkflowMetadata{
			Source:         core.SourceOrchestrator.String(),
			WorkflowId:     workflowID,
			WorkflowExecId: workflowExecID.String(),
			Time:           timestamppb.Now(),
		},
		Details: &pb.CmdWorkflowTrigger_Details{
			TriggerInput: input,
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	response := &TriggerResponse{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID.String(),
	}
	return &CmdTrigger{
		CmdWorkflowTrigger: cmd,
		nc:                 nc,
		Response:           response,
	}
}

func (t *CmdTrigger) Publish(ctx context.Context) error {
	publisher := nats.NewEventPublisher(t.nc)
	return publisher.Publish(ctx, t)
}
