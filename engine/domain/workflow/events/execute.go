package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendExecute(nc *nats.Client, triggerCmd *pbwf.CmdWorkflowTrigger) error {
	workflowID := common.ID(triggerCmd.GetWorkflow().Id)
	corrID := common.ID(triggerCmd.GetMetadata().CorrelationId)
	wExecID := common.ID(triggerCmd.GetWorkflow().ExecId)
	logger.With(
		"workflow_id", workflowID,
		"correlation_id", corrID,
		"workflow_execution_id", wExecID,
	).Debug("Sending CmdWorkflowExecute")

	cmd := pbwf.CmdWorkflowExecute{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         triggerCmd.GetMetadata().State,
		},
		Workflow: triggerCmd.GetWorkflow(),
		Details: &pbwf.CmdWorkflowExecute_Details{
			TriggerInput: triggerCmd.GetDetails().GetTriggerInput(),
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish CmdWorkflowExecute: %w", err)
	}

	return nil
}
