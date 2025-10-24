package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// EventType enumerates execution stream event categories surfaced to clients.
type EventType string

const (
	EventTypeStatus          EventType = "status"
	EventTypeLLMChunk        EventType = "llm_chunk"
	EventTypeToolCall        EventType = "tool_call"
	EventTypeWarning         EventType = "warning"
	EventTypeError           EventType = "error"
	EventTypeStructuredDelta EventType = "structured_delta"
	EventTypeComplete        EventType = "complete"
)

// Event captures a logical event before transport encoding.
type Event struct {
	Type EventType
	Data any
}

// Envelope is the transport representation persisted and broadcast to subscribers.
type Envelope struct {
	ID        int64           `json:"id"`
	ExecID    core.ID         `json:"exec_id"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"ts"`
	Data      json.RawMessage `json:"data"`
}

// Publisher exposes methods to publish and replay execution events.
type Publisher interface {
	Publish(ctx context.Context, execID core.ID, event Event) (Envelope, error)
	Replay(ctx context.Context, execID core.ID, afterID int64, limit int) ([]Envelope, error)
	Channel(execID core.ID) string
}

// NewEnvelope constructs an envelope from the provided event data.
func NewEnvelope(id int64, execID core.ID, event Event, ts time.Time) (Envelope, error) {
	if execID.IsZero() {
		return Envelope{}, fmt.Errorf("streaming: exec id is required")
	}
	if event.Type == "" {
		return Envelope{}, fmt.Errorf("streaming: event type is required")
	}
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return Envelope{}, fmt.Errorf("streaming: marshal payload: %w", err)
	}
	return Envelope{
		ID:        id,
		ExecID:    execID,
		Type:      event.Type,
		Timestamp: ts.UTC(),
		Data:      payload,
	}, nil
}
