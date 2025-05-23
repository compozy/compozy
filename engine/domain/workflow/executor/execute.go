package executor

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	wfevts "github.com/compozy/compozy/engine/domain/workflow/events"
	"github.com/compozy/compozy/pkg/nats"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

func (e *Executor) subscribeExecute(ctx context.Context) error {
	comp := nats.ComponentWorkflow
	cmd := nats.CmdTypeExecute
	err := e.nc.SubscribeCmd(ctx, comp, cmd, e.handleExecute)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	return nil
}

func (e *Executor) handleExecute(_ string, data []byte, _ jetstream.Msg) error {
	// Unmarshal command from event
	var cmd pbwf.CmdWorkflowExecute
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal CmdWorkflowExecute: %w", err)
	}

	// Create workflow state
	_, wConfig, err := e.createAndValidateState(&cmd)
	if err != nil {
		return err
	}

	// Send WorkflowExecutionStart
	info := cmd.GetWorkflow()
	metadata := cmd.GetMetadata()
	if err := wfevts.SendStarted(e.nc, info, metadata); err != nil {
		return fmt.Errorf("failed to send WorkflowExecutionStarted: %w", err)
	}

	// Execute next task
	taskID := *cmd.GetDetails().TaskId
	if taskID == "" {
		taskID = wConfig.Tasks[0].ID
	}

	// TODO: Send task dispatch
	return nil
}

func (e *Executor) createAndValidateState(cmd *pbwf.CmdWorkflowExecute) (*workflow.State, *workflow.Config, error) {
	workflowID := cmd.GetWorkflow().Id
	triggerInputMap := cmd.GetDetails().GetTriggerInput().AsMap()
	if triggerInputMap == nil {
		return nil, nil, fmt.Errorf("trigger input is nil")
	}
	triggerInput := common.Input(triggerInputMap)
	metadata, err := workflow.InfoFromEvent(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow info: %w", err)
	}

	wConfig, err := workflow.FindConfig(e.workflows, workflowID)
	if err != nil {
		return nil, nil, err
	}

	// Create workflow state
	state, err := e.stManager.CreateWorkflowState(metadata, &triggerInput, e.pConfig, wConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create workflow state: %w", err)
	}

	// Validate workflow config
	if err := wConfig.ValidateParams(*state.Trigger); err != nil {
		return nil, nil, fmt.Errorf("failed to validate workflow: %w", err)
	}
	return state, wConfig, nil
}
