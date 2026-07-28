package contract

import "encoding/json"

// AgentTaskClaimNextRequest captures the agent-initiated atomic next-work request.
type AgentTaskClaimNextRequest struct {
	RunID                string   `json:"run_id,omitempty"`
	WorkspaceID          string   `json:"workspace_id,omitempty"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	PriorityMin          int      `json:"priority_min,omitempty"`
	LeaseSeconds         int64    `json:"lease_seconds,omitempty"`
	Wait                 bool     `json:"wait,omitempty"`
	IdempotencyKey       string   `json:"idempotency_key,omitempty"`
}

// AgentTaskClaimPayload is the synchronous claim response for the session-bound lease.
type AgentTaskClaimPayload struct {
	Task                TaskReferencePayload        `json:"task"`
	Run                 TaskRunPayload              `json:"run"`
	Lease               TaskRunLeaseSummaryPayload  `json:"lease"`
	CoordinationChannel *CoordinationChannelPayload `json:"coordination_channel,omitempty"`
}

// AgentTaskHeartbeatRequest extends the caller session's task-run lease.
type AgentTaskHeartbeatRequest struct {
	LeaseSeconds int64 `json:"lease_seconds,omitempty"`
}

// AgentTaskCompleteRequest completes the caller session's claimed task run.
type AgentTaskCompleteRequest struct {
	Result         json.RawMessage `json:"result,omitempty"`
	CreatedTaskIDs []string        `json:"created_task_ids,omitempty"`
}

// AgentTaskFailRequest fails the caller session's claimed task run.
type AgentTaskFailRequest struct {
	Error    string          `json:"error"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// AgentTaskReleaseRequest releases the caller session's claimed task run.
type AgentTaskReleaseRequest struct {
	Reason string `json:"reason,omitempty"`
}
