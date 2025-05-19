package executor

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/compozy/compozy/engine/common"
	commonpb "github.com/compozy/compozy/pkg/pb/common"
	pb "github.com/compozy/compozy/pkg/pb/workflow"
)

func (e *Executor) emitExecStarted(_ context.Context, cmd *pb.WorkflowExecuteCommand) error {
	ev := &pb.WorkflowExecutionStartedEvent{
		Metadata: &commonpb.Metadata{
			CorrelationId:   cmd.GetMetadata().GetCorrelationId(),
			EventId:         common.GenerateEventID(),
			SourceComponent: ComponentName,
			EventTimestamp:  timestamppb.Now(),
		},
		Workflow: cmd.GetWorkflow(),
		Payload: &pb.WorkflowExecutionStartedEvent_Payload{
			Status:  WorkflowStatusExecuting,
			Context: createEventContext(cmd),
		},
	}

	data, err := protojson.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow execution started event: %w", err)
	}

	subject := ev.ToSubject()
	if err := e.natsConn.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish workflow execution started event: %w", err)
	}

	return nil
}

func (e *Executor) emitExecSuccess(
	_ context.Context,
	cmd *pb.WorkflowExecuteCommand,
	result *commonpb.Result,
	duration time.Duration,
) error {
	ev := &pb.WorkflowExecutionSuccessEvent{
		Metadata: &commonpb.Metadata{
			CorrelationId:   cmd.GetMetadata().GetCorrelationId(),
			EventId:         common.GenerateEventID(),
			SourceComponent: ComponentName,
			EventTimestamp:  timestamppb.Now(),
		},
		Workflow: cmd.GetWorkflow(),
		Payload: &pb.WorkflowExecutionSuccessEvent_Payload{
			Status:     WorkflowStatusCompleted,
			Result:     result,
			Context:    createEventContext(cmd),
			DurationMs: duration.Milliseconds(),
		},
	}

	data, err := protojson.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow execution success event: %w", err)
	}

	subject := ev.ToSubject()
	if err := e.natsConn.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish workflow execution success event: %w", err)
	}

	return nil
}

func (e *Executor) emitExecFailed(
	_ context.Context,
	cmd *pb.WorkflowExecuteCommand,
	result *commonpb.Result,
) error {
	ev := &pb.WorkflowExecutionFailedEvent{
		Metadata: &commonpb.Metadata{
			CorrelationId:   cmd.GetMetadata().GetCorrelationId(),
			EventId:         common.GenerateEventID(),
			SourceComponent: ComponentName,
			EventTimestamp:  timestamppb.Now(),
		},
		Workflow: cmd.GetWorkflow(),
		Payload: &pb.WorkflowExecutionFailedEvent_Payload{
			Status:     WorkflowStatusFailed,
			Result:     result,
			Context:    createEventContext(cmd),
			DurationMs: time.Since(cmd.GetMetadata().GetEventTimestamp().AsTime()).Milliseconds(),
		},
	}

	data, err := protojson.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow execution failed event: %w", err)
	}

	subject := ev.ToSubject()
	if err := e.natsConn.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish workflow execution failed event: %w", err)
	}

	return nil
}

func (e *Executor) emitExecTimedOut(_ context.Context, cmd *pb.WorkflowExecuteCommand) error {
	ev := &pb.WorkflowExecutionTimedOutEvent{
		Metadata: &commonpb.Metadata{
			CorrelationId:   cmd.GetMetadata().GetCorrelationId(),
			EventId:         common.GenerateEventID(),
			SourceComponent: ComponentName,
			EventTimestamp:  timestamppb.Now(),
		},
		Workflow: cmd.GetWorkflow(),
		Payload: &pb.WorkflowExecutionTimedOutEvent_Payload{
			Status:  WorkflowStatusTimedOut,
			Context: createEventContext(cmd),
			Result: &commonpb.Result{
				Error: &commonpb.ErrorResult{
					Message: "Workflow execution timed out",
					Code:    ptr("TIMEOUT"),
				},
			},
			DurationMs: e.timeout.Milliseconds(),
		},
	}

	data, err := protojson.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow execution timed out event: %w", err)
	}

	subject := ev.ToSubject()
	if err := e.natsConn.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish workflow execution timed out event: %w", err)
	}

	return nil
}
