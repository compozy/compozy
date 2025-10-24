package workflow

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// StreamQueryName identifies the Temporal query handler for workflow stream state.
const StreamQueryName = "getStreamState"

const (
	// StreamEventWorkflowStart marks the beginning of a workflow execution.
	StreamEventWorkflowStart = "workflow_start"
	// StreamEventWorkflowStatus communicates status updates for a workflow.
	StreamEventWorkflowStatus = "workflow_status"
	// StreamEventComplete indicates a terminal successful completion.
	StreamEventComplete = "complete"
	// StreamEventError reports a terminal error condition for the workflow.
	StreamEventError = "error"
)

// StreamEvent represents a single SSE event emitted for workflow streams.
type StreamEvent struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"ts"`
	Data      json.RawMessage `json:"data"`
}

// StreamState holds the stream cursor and workflow status for queries.
type StreamState struct {
	Events []StreamEvent   `json:"events"`
	Status core.StatusType `json:"status"`
	nextID int64
}

// NewStreamState constructs a stream state with the provided status.
func NewStreamState(status core.StatusType) *StreamState {
	return &StreamState{Status: status}
}

// Append records a new event with an auto-incremented id and payload.
func (s *StreamState) Append(eventType string, ts time.Time, payload any) error {
	if s == nil {
		return errors.New("workflow stream state is nil")
	}
	if eventType == "" {
		return errors.New("workflow stream event type required")
	}
	var data []byte
	switch v := payload.(type) {
	case nil:
		data = nil
	case json.RawMessage:
		if v != nil {
			data = append([]byte(nil), v...)
		}
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return err
		}
		data = encoded
	}
	s.nextID++
	frame := StreamEvent{
		ID:        s.nextID,
		Type:      eventType,
		Timestamp: ts,
		Data:      json.RawMessage(data),
	}
	s.Events = append(s.Events, frame)
	return nil
}

// SetStatus updates the workflow status tracked by the stream state.
func (s *StreamState) SetStatus(status core.StatusType) {
	if s == nil {
		return
	}
	s.Status = status
}

// Clone creates a deep copy so queries can safely mutate independently.
func (s *StreamState) Clone() *StreamState {
	if s == nil {
		return nil
	}
	copyState := &StreamState{Status: s.Status, nextID: s.nextID}
	if len(s.Events) > 0 {
		copyState.Events = make([]StreamEvent, len(s.Events))
		copy(copyState.Events, s.Events)
	}
	return copyState
}

// LastEventID returns the most recent event identifier in the state.
func (s *StreamState) LastEventID() int64 {
	if s == nil || len(s.Events) == 0 {
		return 0
	}
	return s.Events[len(s.Events)-1].ID
}
