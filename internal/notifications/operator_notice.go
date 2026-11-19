package notifications

import "time"

// OperatorNotice is the sanitized payload of an operator notification.
type OperatorNotice struct {
	Kind           string
	Action         string
	Resource       string
	NotificationID string
	SessionID      string
	ProfileID      string
	WorkspaceID    string
	Title          string
	Body           string
	At             time.Time
}
