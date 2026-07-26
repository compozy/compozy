package hooks

import "github.com/compozy/agh/internal/network/participation"

// ContextCompactPayload is shared by context compaction hooks.
type ContextCompactPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	Reason        string         `json:"reason,omitempty"`
	Strategy      string         `json:"strategy,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	ContextBlocks []ContextBlock `json:"context_blocks,omitempty"`
}

// ContextPreCompactPayload is delivered before compaction.
type ContextPreCompactPayload = ContextCompactPayload

// ContextPostCompactPayload is delivered after compaction.
type ContextPostCompactPayload = ContextCompactPayload

// ContextCompactionPatch mutates or denies compaction behavior.
type ContextCompactionPatch struct {
	ControlPatch
	Reason        *string        `json:"reason,omitempty"`
	Strategy      *string        `json:"strategy,omitempty"`
	ContextBlocks []ContextBlock `json:"context_blocks,omitempty"`
}

// ContextPreCompactPatch is the pre-compact patch surface.
type ContextPreCompactPatch = ContextCompactionPatch

// ContextPostCompactPatch is the post-compact patch surface.
type ContextPostCompactPatch = ContextCompactionPatch

// AutonomyObservationPatch captures optional labels for committed autonomy lifecycle events.
type AutonomyObservationPatch struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// CoordinatorContext carries the coordinator identifiers shared across coordinator hooks.
type CoordinatorContext struct {
	WorkspaceID                  string              `json:"workspace_id,omitempty"`
	Workspace                    string              `json:"workspace,omitempty"`
	AgentName                    string              `json:"agent_name,omitempty"`
	CoordinatorSessionID         string              `json:"coordinator_session_id,omitempty"`
	TaskID                       string              `json:"task_id,omitempty"`
	RunID                        string              `json:"run_id,omitempty"`
	WorkflowID                   string              `json:"workflow_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation,omitempty"`
	Provider                     string              `json:"provider,omitempty"`
	Model                        string              `json:"model,omitempty"`
}

// CoordinatorPreSpawnPayload is delivered before the daemon creates a coordinator session.
type CoordinatorPreSpawnPayload struct {
	PayloadBase
	CoordinatorContext
	Reason     string `json:"reason,omitempty"`
	Denied     bool   `json:"denied,omitempty"`
	DenyReason string `json:"deny_reason,omitempty"`
}

// CoordinatorLifecyclePayload is shared by committed coordinator lifecycle hooks.
type CoordinatorLifecyclePayload struct {
	PayloadBase
	CoordinatorContext
	DecisionKind string `json:"decision_kind,omitempty"`
	Decision     string `json:"decision,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	Error        string `json:"error,omitempty"`
}

// CoordinatorSpawnedPayload is delivered after a coordinator session is created.
type CoordinatorSpawnedPayload = CoordinatorLifecyclePayload

// CoordinatorDecisionPayload is delivered when a coordinator records a semantic decision.
type CoordinatorDecisionPayload = CoordinatorLifecyclePayload

// CoordinatorStoppedPayload is delivered after a coordinator session stops.
type CoordinatorStoppedPayload = CoordinatorLifecyclePayload

// CoordinatorFailedPayload is delivered after a coordinator lifecycle failure.
type CoordinatorFailedPayload = CoordinatorLifecyclePayload

// CoordinatorSpawnPatch mutates or denies coordinator spawn requests.
type CoordinatorSpawnPatch struct {
	ControlPatch
	AgentName *string `json:"agent_name,omitempty"`
	Provider  *string `json:"provider,omitempty"`
	Model     *string `json:"model,omitempty"`
}

// CoordinatorObservationPatch is the observation patch surface for committed coordinator hooks.
type CoordinatorObservationPatch = AutonomyObservationPatch
