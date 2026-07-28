package store

import "time"

// PermissionLogEntry is an audit log entry for a daemon permission decision.
type PermissionLogEntry struct {
	ID         string
	SessionID  string
	AgentName  string
	Action     string
	Resource   string
	Decision   string
	PolicyUsed string
	Timestamp  time.Time
}

// Validate ensures the permission audit entry is complete.
func (e PermissionLogEntry) Validate() error {
	if err := requireField(e.SessionID, "permission log session id"); err != nil {
		return err
	}
	if err := requireField(e.AgentName, "permission log agent name"); err != nil {
		return err
	}
	if err := requireField(e.Action, "permission log action"); err != nil {
		return err
	}
	if err := requireField(e.Resource, "permission log resource"); err != nil {
		return err
	}
	if err := requireField(e.Decision, "permission log decision"); err != nil {
		return err
	}
	return requireField(e.PolicyUsed, "permission log policy")
}

// PermissionLogQuery filters permission audit queries.
type PermissionLogQuery struct {
	SessionID string
	AgentName string
	Decision  string
	Since     time.Time
	Limit     int
}

// Validate ensures the query uses sane bounds.
func (q PermissionLogQuery) Validate() error {
	return requirePositiveLimit(q.Limit, "permission log limit")
}
