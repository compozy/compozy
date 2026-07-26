package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
)

type EventSummary struct {
	ID          string
	SessionID   string
	WorkspaceID string
	Sequence    int64
	Type        string
	AgentName   string
	Provider    string
	Outcome     string
	Content     json.RawMessage
	EventCorrelation
	ParentSessionID string
	RootSessionID   string
	SpawnDepth      int
	Summary         string
	Timestamp       time.Time
}

// Validate ensures the summary contains the required identifying fields.
func (s EventSummary) Validate() error {
	eventType := strings.TrimSpace(s.Type)
	if err := requireField(eventType, "event summary type"); err != nil {
		return err
	}
	if err := eventspkg.ValidatePublicName(eventType); err != nil {
		return fmt.Errorf("store: invalid event summary type: %w", err)
	}
	if !eventspkg.ValidOutcome(s.Outcome) {
		return fmt.Errorf("store: invalid event summary outcome %q", s.Outcome)
	}
	if eventSummaryAllowsGlobalScope(eventType) {
		return nil
	}
	if err := requireField(s.WorkspaceID, "event summary workspace_id"); err != nil {
		return err
	}
	if err := requireField(s.SessionID, "event summary session id"); err != nil {
		return err
	}
	return requireField(s.AgentName, "event summary agent name")
}

func cloneNormalizedTimestamp(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func eventSummaryAllowsGlobalScope(eventType string) bool {
	return eventspkg.AllowsGlobalScope(eventType)
}

// EventSummaryQuery filters global event summary queries.
type EventSummaryQuery struct {
	SessionID     string
	WorkspaceID   string
	AgentName     string
	Type          string
	TaskID        string
	RunID         string
	ActorKind     string
	ActorID       string
	Provider      string
	Outcome       string
	Component     string
	ErrorOnly     bool
	AfterSequence int64
	Since         time.Time
	Limit         int
}

// Validate ensures the query uses sane bounds.
func (q EventSummaryQuery) Validate() error {
	if err := requirePositiveLimit(q.Limit, "event summary limit"); err != nil {
		return err
	}
	if q.AfterSequence < 0 {
		return fmt.Errorf("store: invalid event summary after sequence %d", q.AfterSequence)
	}
	if !eventspkg.ValidOutcome(q.Outcome) {
		return fmt.Errorf("store: invalid event summary outcome %q", q.Outcome)
	}
	if !eventspkg.ValidComponent(q.Component) {
		return fmt.Errorf("store: invalid event summary component %q", q.Component)
	}
	return nil
}
