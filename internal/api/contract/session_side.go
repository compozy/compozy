package contract

import "time"

type OpenSessionSideRequest struct {
	Message        string `json:"message"`
	Name           string `json:"name,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
}

type SessionSideItemPayload struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	State                  string     `json:"state"`
	Prompting              bool       `json:"prompting"`
	PendingPermissionCount int        `json:"pending_permission_count"`
	PendingClarifyCount    int        `json:"pending_clarify_count"`
	ArchivedAt             *time.Time `json:"archived_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type SessionSidesResponse struct {
	Sides []SessionSideItemPayload `json:"sides"`
}

type SessionSideCommandResult struct {
	SessionID            string `json:"session_id"`
	Seed                 string `json:"seed"`
	SourceTurnInProgress bool   `json:"source_turn_in_progress"`
}
