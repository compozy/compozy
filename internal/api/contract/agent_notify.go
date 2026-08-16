package contract

import "time"

type AgentNotifyRequest struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

type AgentNotifyResponse struct {
	Outcome      string `json:"outcome"`
	RetryAfterMS int64  `json:"retry_after_ms,omitempty"`
}

type OperatorNotificationEventPayload struct {
	NotificationID string    `json:"notification_id"`
	SessionID      string    `json:"session_id"`
	WorkspaceID    string    `json:"workspace_id"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	At             time.Time `json:"at"`
}
