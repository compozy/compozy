package wfexecutor

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

func (e *Executor) subscribeExecute(ctx context.Context) error {
	subCtx := context.Background()
	cs, err := e.nc.GetConsumerCmd(ctx, nats.ComponentWorkflow, nats.CmdTypeExecute)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}
	subOpts := nats.DefaultSubscribeOpts(cs)
	errCh := nats.SubscribeConsumer(subCtx, e.handleExecute, subOpts)
	go func() {
		for err := range errCh {
			if err != nil {
				logger.Error("Error in workflow execute subscription", "error", err)
			}
		}
	}()
	return nil
}

func (e *Executor) handleExecute(_ string, data []byte, _ jetstream.Msg) error {
	var cmd pbwf.WorkflowExecuteCommand
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal WorkflowExecuteCommand: %w", err)
	}

	corrID := common.ID(cmd.GetMetadata().GetCorrelationId())
	execID := common.ID(cmd.GetWorkflow().GetExecId())
	_, err := e.stm.LoadWorkflowState(corrID, execID)
	if err != nil {
		return fmt.Errorf("failed to load workflow state: %w", err)
	}
	// TODO: Handle workflow execution
	return nil
}
