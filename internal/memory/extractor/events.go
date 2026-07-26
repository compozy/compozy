package extractor

import (
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const (
	// EventStarted records that the runtime began extracting one transcript range.
	EventStarted = "memory.extractor.started"
	// EventCompleted records that extraction and inbox production finished.
	EventCompleted = "memory.extractor.completed"
	// EventFailed records extraction, inbox, decode, or controller handoff failures.
	EventFailed = "memory.extractor.failed"
	// EventCoalesced records bounded queue merging for one session.
	EventCoalesced = "memory.extractor.coalesced"
	// EventDropped records extractor work dropped before provider execution.
	EventDropped = "memory.extractor.dropped"
)

// Event is redaction-safe extractor telemetry persisted into memory_events.
type Event struct {
	Op          string
	Turn        memcontract.TurnRecord
	SessionID   string
	WorkspaceID string
	AgentID     string
	ActorKind   string
	DecisionID  string
	TargetID    string
	Metadata    map[string]string
	Error       string
	At          time.Time
}

// Normalize returns a copy with canonical identity fields filled from the turn.
func (e Event) Normalize(now func() time.Time) Event {
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	e.Op = strings.TrimSpace(e.Op)
	e.SessionID = firstNonEmpty(e.SessionID, e.Turn.SessionID)
	e.WorkspaceID = firstNonEmpty(e.WorkspaceID, e.Turn.WorkspaceID)
	e.AgentID = firstNonEmpty(e.AgentID, e.Turn.AgentID)
	e.ActorKind = firstNonEmpty(e.ActorKind, e.Turn.ActorKind, "system")
	e.DecisionID = strings.TrimSpace(e.DecisionID)
	e.TargetID = strings.TrimSpace(e.TargetID)
	e.Error = strings.TrimSpace(e.Error)
	if e.Metadata == nil {
		e.Metadata = map[string]string{}
	}
	if e.At.IsZero() {
		e.At = now().UTC()
	}
	return e
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
