package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
)

func SendExecute(nc *nats.Client, dispatchCmd *pb.CmdTaskDispatch) error {
	source := pb.SourceTypeOrchestrator
	metadata, err := dispatchCmd.GetMetadata().Clone(source)
	if err != nil {
		return fmt.Errorf("failed to clone metadata: %w", err)
	}
	cmd := &pb.CmdTaskExecute{
		Metadata: metadata,
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	if err := nc.PublishCmd(cmd); err != nil {
		return fmt.Errorf("failed to publish CmdTaskDispatch: %w", err)
	}

	logger.With("metadata", cmd.Metadata).Debug("Sent: TaskExecute")
	return nil
}
