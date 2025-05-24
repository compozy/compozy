package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
)

func SendExecute(nc *nats.Client, triggerCmd *pb.CmdWorkflowTrigger) error {
	source := pb.SourceTypeOrchestrator
	metadata, err := triggerCmd.GetMetadata().Clone(source)
	if err != nil {
		return fmt.Errorf("failed to clone metadata: %w", err)
	}
	cmd := &pb.CmdWorkflowExecute{
		Metadata: metadata,
		Details: &pb.CmdWorkflowExecute_Details{
			TriggerInput: triggerCmd.GetDetails().GetTriggerInput(),
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	if err := nc.PublishCmd(cmd); err != nil {
		return fmt.Errorf("failed to publish CmdWorkflowExecute: %w", err)
	}

	logger.With("metadata", cmd.Metadata).Debug("Sent: WorkflowExecute")
	return nil
}
