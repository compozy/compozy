package transcript

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// ReplayPosition identifies a canonical message and the event that starts it.
type ReplayPosition struct {
	Message  Message
	Sequence int64
}

// ReplayLayout gives the canonical message order using content-excluded event shapes.
func ReplayLayout(ctx context.Context, events []store.SessionEvent) ([]ReplayPosition, error) {
	var messages []Message
	var assistant assistantBuffer
	tools := make(map[string]*toolLifecycle)
	positions := make(map[string]int64, len(events))
	calls, results := make(map[string]int64), make(map[string]int64)
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		parsed := parseEvent(event)
		positions[parsed.ID] = event.Sequence
		if parsed.Type == acp.EventTypeToolCall || parsed.Type == acp.EventTypeToolResult {
			key := toolLifecycleKey(parsed)
			if calls[key] == 0 {
				calls[key] = event.Sequence
			}
			if parsed.Type == acp.EventTypeToolResult && results[key] == 0 {
				results[key] = event.Sequence
			}
		}
		processTranscriptEvent(&messages, &assistant, tools, parsed)
	}
	flushAssistantBuffer(&messages, &assistant)
	result := make([]ReplayPosition, 0, len(messages))
	for _, message := range messages {
		sequence := positions[message.ID]
		switch message.Role {
		case RoleToolCall:
			sequence = calls[message.ID]
		case RoleToolResult:
			sequence = results[message.ID]
		}
		result = append(result, ReplayPosition{Message: message, Sequence: sequence})
	}
	return result, nil
}
