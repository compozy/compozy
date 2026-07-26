package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

// NetworkTaskStatusProjectionPayload is the immutable transition snapshot
// stored for one event and its exact recipients.
type NetworkTaskStatusProjectionPayload struct {
	EventID                  string             `json:"event_id"`
	EventType                string             `json:"event_type"`
	TaskID                   string             `json:"task_id"`
	RunID                    string             `json:"run_id,omitempty"`
	TaskStatus               string             `json:"task_status"`
	RunStatus                string             `json:"run_status,omitempty"`
	NetworkParticipation     participation.Spec `json:"network_participation"`
	ConversationAvailable    bool               `json:"conversation_available"`
	ConversationAvailability string             `json:"conversation_availability"`
	ProjectedAt              string             `json:"projected_at"`
}

// NetworkTaskStatusPublication is one typed task transition bound to its Network origin thread.
type NetworkTaskStatusPublication struct {
	EventID        string
	EventType      string
	TaskID         string
	RunID          string
	WorkspaceID    string
	Channel        string
	ThreadID       string
	ProjectionJSON json.RawMessage
	ProjectedAt    time.Time
}

// Validate ensures a publication is thread-scoped and contains typed JSON.
func (p NetworkTaskStatusPublication) Validate() error {
	for _, field := range []struct {
		value string
		name  string
	}{
		{value: p.EventID, name: "event_id"},
		{value: p.EventType, name: "event_type"},
		{value: p.TaskID, name: "task_id"},
	} {
		if err := requireField(field.value, "network task status publication "+field.name); err != nil {
			return err
		}
	}
	if err := (NetworkConversationRef{
		WorkspaceID: p.WorkspaceID,
		Channel:     p.Channel,
		Surface:     NetworkSurfaceThread,
		ThreadID:    p.ThreadID,
	}).Validate(); err != nil {
		return err
	}
	if len(p.ProjectionJSON) == 0 || !json.Valid(p.ProjectionJSON) {
		return fmt.Errorf("store: network task status publication projection_json must be valid JSON")
	}
	if p.ProjectedAt.IsZero() {
		return fmt.Errorf("store: network task status publication projected_at is required")
	}
	return nil
}

// NetworkTaskStatusProjection is one recipient-scoped durable projection.
type NetworkTaskStatusProjection struct {
	NetworkTaskStatusPublication
	RecipientSessionID string
}

// NetworkTaskStatusProjectionQuery filters durable recipient projections.
type NetworkTaskStatusProjectionQuery struct {
	WorkspaceID        string
	RecipientSessionID string
	TaskID             string
	Limit              int
}

// Validate keeps projection reads workspace-scoped and bounded.
func (q NetworkTaskStatusProjectionQuery) Validate() error {
	if err := requireField(q.WorkspaceID, "network task status projection workspace_id"); err != nil {
		return err
	}
	if strings.TrimSpace(q.RecipientSessionID) == "" && strings.TrimSpace(q.TaskID) == "" {
		return fmt.Errorf("store: network task status projection query requires recipient_session_id or task_id")
	}
	return requirePositiveLimit(q.Limit, "network task status projection limit")
}
