package events

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CmdDispatch struct {
	*pb.CmdTaskDispatch
	publisher core.EventPublisher
}

func NewCmdDispatch(
	nc *nats.Client,
	parentMetadata *pb.WorkflowMetadata,
	taskID string,
) (*CmdDispatch, error) {
	tExecID, err := core.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate task execution ID: %w", err)
	}
	source := core.SourceWorkflowExecute
	metadata := &pb.TaskMetadata{
		Version:        core.GetVersion(),
		Source:         source.String(),
		WorkflowId:     parentMetadata.WorkflowId,
		WorkflowExecId: parentMetadata.WorkflowExecId,
		TaskId:         taskID,
		TaskExecId:     tExecID.String(),
		Time:           timestamppb.Now(),
	}
	cmd := &pb.CmdTaskDispatch{
		Metadata: metadata,
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	publisher := nats.NewEventPublisher(nc)
	return &CmdDispatch{CmdTaskDispatch: cmd, publisher: publisher}, nil
}

func (d *CmdDispatch) Publish(ctx context.Context) error {
	return d.publisher.Publish(ctx, d)
}
