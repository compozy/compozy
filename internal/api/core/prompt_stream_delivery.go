package core

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
)

// DeliverPromptEventStream forwards one accepted prompt until the client,
// daemon, provider, or stream encoder closes the delivery path.
func DeliverPromptEventStream(
	ctx context.Context,
	shutdown <-chan struct{},
	events <-chan acp.AgentEvent,
	cancel context.CancelFunc,
	encoder *PromptStreamEncoder,
	writer FlushWriter,
) {
	for {
		select {
		case <-ctx.Done():
			cancel()
			return
		case <-shutdown:
			cancel()
			return
		case event, ok := <-events:
			if !ok {
				if err := encoder.Emit(writer, acp.PromptStreamIncompleteEvent()); err != nil {
					return
				}
				return
			}
			if err := encoder.Emit(writer, event); err != nil {
				cancel()
				return
			}
			if event.Type == acp.EventTypeDone || event.Type == acp.EventTypeError {
				return
			}
		}
	}
}
