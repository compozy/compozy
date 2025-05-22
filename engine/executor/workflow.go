package executor

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/stmanager"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

type WorkflowExecutor struct {
	stm *stmanager.Manager
	nc  *nats.Client
}

func NewWorkflowExecutor(nc *nats.Client, stm *stmanager.Manager) *WorkflowExecutor {
	return &WorkflowExecutor{nc: nc, stm: stm}
}

func (e *WorkflowExecutor) Start(ctx context.Context) error {
	if err := e.subscribeWorkflowExecute(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to WorkflowExecuteCmd: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Execute Workflow
// -----------------------------------------------------------------------------

func (e *WorkflowExecutor) subscribeWorkflowExecute(ctx context.Context) error {
	subCtx := context.Background()
	cs, err := e.nc.GetConsumerCmd(ctx, nats.ComponentWorkflow, nats.CmdTypeExecute)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}
	subOpts := nats.DefaultSubscribeOpts(cs)
	errCh := nats.SubscribeConsumer(subCtx, e.handleWorkflowExecute, subOpts)
	go func() {
		for err := range errCh {
			if err != nil {
				logger.Error("Error in workflow execute subscription", "error", err)
			}
		}
	}()
	return nil
}

func (e *WorkflowExecutor) handleWorkflowExecute(_ string, data []byte, _ jetstream.Msg) error {
	var cmd pbwf.WorkflowExecuteCommand
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal WorkflowExecuteCommand: %w", err)
	}

	corrId := common.ID(cmd.GetMetadata().CorrelationId)
	execId := common.ID(cmd.GetWorkflow().ExecId)
	st, err := e.stm.GetWorkflowState(corrId, execId)
	if err != nil {
		return fmt.Errorf("failed to get workflow state: %w", err)
	}

	if st.GetStatus() != nats.StatusRunning {
		return fmt.Errorf("workflow is not in running state")
	}

	// TODO: Handle workflow execution
	return nil
}
