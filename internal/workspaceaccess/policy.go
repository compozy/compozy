package workspaceaccess

import "context"

// ActorKind identifies the trusted runtime origin of an access request.
type ActorKind string

const (
	ActorAgentSession ActorKind = "agent_session"
	ActorHuman        ActorKind = "human"
	ActorExtension    ActorKind = "extension"
	ActorAutomation   ActorKind = "automation"
	ActorNetworkPeer  ActorKind = "network_peer"
	ActorDaemon       ActorKind = "daemon"
)

// ActorRef carries daemon-validated actor identity into the policy.
type ActorRef struct {
	Kind        ActorKind
	SessionID   string
	WorkspaceID string
	AgentName   string
	Operator    bool
}

// Seam identifies the runtime boundary requesting cross-workspace access.
type Seam string

const (
	SeamIdentity     Seam = "identity"
	SeamTask         Seam = "task"
	SeamTool         Seam = "tool"
	SeamSpawn        Seam = "spawn"
	SeamCoordination Seam = "coordination"
)

// Request is the complete input to one workspace-access decision.
type Request struct {
	Actor             ActorRef
	TargetWorkspaceID string
	Seam              Seam
}

// DecisionSource records the policy branch that produced a decision.
type DecisionSource string

const (
	SourceOperator       DecisionSource = "operator"
	SourceSameWorkspace  DecisionSource = "same_workspace"
	SourcePermissionMode DecisionSource = "permission_mode"
	SourceSessionConsent DecisionSource = "session_consent"
	SourceDenied         DecisionSource = "denied"
)

// Decision is the fail-closed result of one authorization request.
type Decision struct {
	Allowed        bool
	Source         DecisionSource
	PromptEligible bool
}

// Policy is the single decision funnel for cross-workspace access.
type Policy interface {
	Authorize(ctx context.Context, req Request) (Decision, error)
}
