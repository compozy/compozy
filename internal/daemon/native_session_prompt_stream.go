package daemon

import (
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func drainNativeSessionPromptEvents(events <-chan acp.AgentEvent) error {
	var terminalErr error
	for event := range events {
		if terminalErr == nil && event.Type == acp.EventTypeError {
			terminalErr = nativeSessionPromptEventError(event)
		}
	}
	return terminalErr
}

func nativeSessionPromptEventError(event acp.AgentEvent) error {
	message := "session prompt ended with a runtime error"
	reason := toolspkg.ReasonBackendUnhealthy
	if event.Failure != nil && event.Failure.Kind == store.FailureProcess {
		message = "session prompt failed because the runtime process exited"
		reason = toolspkg.ReasonBackendDead
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeBackendFailed,
		toolspkg.ToolIDSessionPrompt,
		message,
		fmt.Errorf("%w: session prompt terminal error", toolspkg.ErrToolBackendFailed),
		reason,
	)
}
