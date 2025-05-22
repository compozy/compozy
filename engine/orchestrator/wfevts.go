package orchestrator

import (
	"github.com/compozy/compozy/engine/domain/workflow"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
)

func (o *Orchestrator) SendWorkflowExecutionStarted(cmd *pbwf.WorkflowTriggerCommand) error {
	return workflow.SendExecutionStarted(o.natsClient, cmd)
}
