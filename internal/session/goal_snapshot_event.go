package session

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// EventTypeGoalSnapshotChanged is the persisted session projection signal for Goal changes.
const EventTypeGoalSnapshotChanged = "goal_snapshot_changed"

// GoalEvent is one deterministic Goal projection event for a session database.
type GoalEvent struct {
	EventID         string
	SessionID       string
	SyntheticTurnID string
	AgentName       string
	Type            string
	Content         []byte
	CreatedAt       time.Time
}

// Validate enforces the closed Goal projection event shape.
func (e GoalEvent) Validate() error {
	if strings.TrimSpace(e.EventID) == "" || strings.TrimSpace(e.SessionID) == "" ||
		strings.TrimSpace(e.SyntheticTurnID) == "" || strings.TrimSpace(e.AgentName) == "" ||
		strings.TrimSpace(e.Type) != EventTypeGoalSnapshotChanged || len(e.Content) == 0 ||
		!json.Valid(e.Content) || e.CreatedAt.IsZero() {
		return errors.New("session: Goal snapshot event is invalid")
	}
	return nil
}
