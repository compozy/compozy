package contract

import (
	apicontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

// AgentSoulGetParams identifies one workspace-visible Soul read model.
type AgentSoulGetParams struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	AgentName   string `json:"agent_name"`
}

// AgentSoulValidateParams validates current or proposed SOUL.md content.
type AgentSoulValidateParams = apicontract.AgentSoulValidateRequest

// AgentSoulPutParams creates or replaces SOUL.md through managed authoring.
type AgentSoulPutParams = apicontract.AgentSoulPutRequest

// AgentSoulDeleteParams deletes SOUL.md through managed authoring.
type AgentSoulDeleteParams = apicontract.AgentSoulDeleteRequest

// AgentSoulHistoryParams lists managed Soul authoring revisions.
type AgentSoulHistoryParams = apicontract.AgentSoulHistoryRequest

// AgentSoulRollbackParams restores a prior managed Soul revision.
type AgentSoulRollbackParams = apicontract.AgentSoulRollbackRequest

// AgentHeartbeatGetParams identifies one workspace-visible Heartbeat policy.
type AgentHeartbeatGetParams struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	AgentName   string `json:"agent_name"`
}

// AgentHeartbeatValidateParams validates proposed HEARTBEAT.md content.
type AgentHeartbeatValidateParams = apicontract.HeartbeatValidateRequest

// AgentHeartbeatPutParams creates or replaces HEARTBEAT.md through managed authoring.
type AgentHeartbeatPutParams = apicontract.HeartbeatPutRequest

// AgentHeartbeatDeleteParams deletes HEARTBEAT.md through managed authoring.
type AgentHeartbeatDeleteParams = apicontract.HeartbeatDeleteRequest

// AgentHeartbeatHistoryParams lists managed Heartbeat authoring revisions.
type AgentHeartbeatHistoryParams = apicontract.HeartbeatHistoryRequest

// AgentHeartbeatRollbackParams restores a prior Heartbeat revision or snapshot digest.
type AgentHeartbeatRollbackParams = apicontract.HeartbeatRollbackRequest

// AgentHeartbeatStatusParams composes Heartbeat policy, wake state, health, and wake audit.
type AgentHeartbeatStatusParams = apicontract.HeartbeatStatusRequest

// AgentHeartbeatWakeParams requests one managed advisory wake decision.
type AgentHeartbeatWakeParams = apicontract.HeartbeatWakeRequest

// AutomationJobsParams filters visible automation jobs.
type AutomationJobsParams struct {
	Scope       automationpkg.Scope `json:"scope,omitempty"`
	WorkspaceID string              `json:"workspace_id,omitempty"`
	Enabled     *bool               `json:"enabled,omitempty"`
	Query       string              `json:"q,omitempty"`
	Cursor      string              `json:"cursor,omitempty"`
	Limit       int                 `json:"limit,omitempty"`
}

// AutomationTriggersParams filters visible automation triggers.
type AutomationTriggersParams struct {
	Scope       automationpkg.Scope `json:"scope,omitempty"`
	WorkspaceID string              `json:"workspace_id,omitempty"`
	Event       string              `json:"event,omitempty"`
	Enabled     *bool               `json:"enabled,omitempty"`
	Query       string              `json:"q,omitempty"`
	Cursor      string              `json:"cursor,omitempty"`
	Limit       int                 `json:"limit,omitempty"`
}

// AutomationJobsResult is one bounded page of extension-visible automation jobs.
type AutomationJobsResult struct {
	Jobs []automationpkg.Job                  `json:"jobs"`
	Page apicontract.CountedCursorPagePayload `json:"page"`
}

// AutomationTriggersResult is one bounded page of redacted extension-visible automation triggers.
type AutomationTriggersResult struct {
	Triggers []apicontract.TriggerPayload         `json:"triggers"`
	Page     apicontract.CountedCursorPagePayload `json:"page"`
}

// AutomationRunsParams filters visible automation runs.
type AutomationRunsParams struct {
	JobID     string                  `json:"job_id,omitempty"`
	TriggerID string                  `json:"trigger_id,omitempty"`
	Status    automationpkg.RunStatus `json:"status,omitempty"`
	Limit     int                     `json:"limit,omitempty"`
}

// AutomationTargetParams identifies one automation resource by id.
type AutomationTargetParams struct {
	ID string `json:"id"`
}

// AutomationJobCreateParams starts a new dynamic automation job.
type AutomationJobCreateParams = apicontract.CreateJobRequest

// AutomationJobUpdateParams patches one automation job definition by id.
type AutomationJobUpdateParams struct {
	ID string `json:"id"`
	apicontract.UpdateJobRequest
}

// AutomationJobTriggerParams forces one immediate automation job run.
type AutomationJobTriggerParams struct {
	ID      string         `json:"id"`
	Payload map[string]any `json:"payload,omitempty"`
}

// AutomationJobRunsParams filters run history for one automation job.
type AutomationJobRunsParams struct {
	ID     string                  `json:"id"`
	Status automationpkg.RunStatus `json:"status,omitempty"`
	Limit  int                     `json:"limit,omitempty"`
}

// AutomationTriggerCreateParams starts a new dynamic automation trigger.
type AutomationTriggerCreateParams = apicontract.CreateTriggerRequest

// AutomationTriggerUpdateParams patches one automation trigger definition by id.
type AutomationTriggerUpdateParams struct {
	ID string `json:"id"`
	apicontract.UpdateTriggerRequest
}

// AutomationTriggerRunsParams filters run history for one automation trigger.
type AutomationTriggerRunsParams struct {
	ID     string                  `json:"id"`
	Status automationpkg.RunStatus `json:"status,omitempty"`
	Limit  int                     `json:"limit,omitempty"`
}

// AutomationTriggerFireParams injects one extension-originated trigger event.
type AutomationTriggerFireParams struct {
	Event       string              `json:"event"`
	Scope       automationpkg.Scope `json:"scope"`
	WorkspaceID string              `json:"workspace_id,omitempty"`
	Payload     map[string]any      `json:"payload,omitempty"`
}
