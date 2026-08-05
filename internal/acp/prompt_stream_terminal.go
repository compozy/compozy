package acp

import (
	"errors"

	"github.com/compozy/compozy/internal/store"
)

// ErrPromptStreamIncomplete identifies an ACP prompt stream that closed
// without a Done or Error event.
var ErrPromptStreamIncomplete = errors.New("acp prompt stream ended before a terminal event")

// PromptStreamIncompleteEvent returns the canonical terminal failure for an
// upstream stream that closes without declaring completion or failure.
func PromptStreamIncompleteEvent() AgentEvent {
	summary := ErrPromptStreamIncomplete.Error()
	return AgentEvent{
		Type:  EventTypeError,
		Error: summary,
		Failure: &store.SessionFailure{
			Kind:    store.FailureTransport,
			Summary: summary,
		},
	}
}
