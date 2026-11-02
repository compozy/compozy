package contract

import "time"

type PermissionDailyRollupPayload struct {
	Day       string `json:"day"`
	AgentName string `json:"agent_name"`
	Decision  string `json:"decision"`
	Count     int64  `json:"count"`
}

type PermissionRollupResponse struct {
	Since   time.Time                      `json:"since"`
	Until   time.Time                      `json:"until"`
	Rollups []PermissionDailyRollupPayload `json:"rollups"`
}

// PermissionEntryPayload is one permission decision and who started the work behind it.
// An empty initiator means the row predates initiator tracking.
type PermissionEntryPayload struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"session_id"`
	AgentName     string    `json:"agent_name"`
	Action        string    `json:"action"`
	Resource      string    `json:"resource"`
	Decision      string    `json:"decision"`
	InitiatorKind string    `json:"initiator_kind"`
	InitiatorID   string    `json:"initiator_id"`
	Timestamp     time.Time `json:"timestamp"`
}

type PermissionEntriesResponse struct {
	Entries []PermissionEntryPayload `json:"entries"`
}
