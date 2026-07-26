package hooks

import "github.com/compozy/agh/internal/network/participation"

// PermissionSet captures concrete permission atoms that spawned children may only narrow.
type PermissionSet struct {
	Tools           []string `json:"tools,omitempty"`
	Skills          []string `json:"skills,omitempty"`
	MCPServers      []string `json:"mcp_servers,omitempty"`
	WorkspacePaths  []string `json:"workspace_paths,omitempty"`
	NetworkChannels []string `json:"network_channels,omitempty"`
	SandboxProfiles []string `json:"sandbox_profiles,omitempty"`
}

// SpawnContext carries spawn identifiers shared across spawn lifecycle hooks.
type SpawnContext struct {
	ParentSessionID              string              `json:"parent_session_id,omitempty"`
	RootSessionID                string              `json:"root_session_id,omitempty"`
	ChildSessionID               string              `json:"child_session_id,omitempty"`
	WorkspaceID                  string              `json:"workspace_id,omitempty"`
	Workspace                    string              `json:"workspace,omitempty"`
	AgentName                    string              `json:"agent_name,omitempty"`
	SpawnRole                    string              `json:"spawn_role,omitempty"`
	SpawnDepth                   int                 `json:"spawn_depth,omitempty"`
	TTLSeconds                   int64               `json:"ttl_seconds,omitempty"`
	AutoStopOnParent             bool                `json:"auto_stop_on_parent,omitempty"`
	TaskID                       string              `json:"task_id,omitempty"`
	RunID                        string              `json:"run_id,omitempty"`
	WorkflowID                   string              `json:"workflow_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation,omitempty"`
	SoulSnapshotID               string              `json:"soul_snapshot_id,omitempty"`
	SoulDigest                   string              `json:"soul_digest,omitempty"`
	ParentSoulDigest             string              `json:"parent_soul_digest,omitempty"`
}

// SpawnPreCreatePayload is delivered before a child session is created.
type SpawnPreCreatePayload struct {
	PayloadBase
	SpawnContext
	ParentPermissions *PermissionSet `json:"parent_permissions"`
	ChildPermissions  *PermissionSet `json:"child_permissions"`
	Denied            bool           `json:"denied,omitempty"`
	DenyReason        string         `json:"deny_reason,omitempty"`
}

// SpawnLifecyclePayload is shared by committed spawn lifecycle hooks.
type SpawnLifecyclePayload struct {
	PayloadBase
	SpawnContext
	ParentPermissions *PermissionSet `json:"parent_permissions,omitempty"`
	ChildPermissions  *PermissionSet `json:"child_permissions,omitempty"`
	StopReason        string         `json:"stop_reason,omitempty"`
	ReapReason        string         `json:"reap_reason,omitempty"`
	Error             string         `json:"error,omitempty"`
}

// SpawnCreatedPayload is delivered after a child session is created.
type SpawnCreatedPayload = SpawnLifecyclePayload

// SpawnParentStoppedPayload is delivered when parent-stop reaps a child session.
type SpawnParentStoppedPayload = SpawnLifecyclePayload

// SpawnTTLExpiredPayload is delivered when TTL expiry reaps a child session.
type SpawnTTLExpiredPayload = SpawnLifecyclePayload

// SpawnReapedPayload is delivered after a child session is reaped.
type SpawnReapedPayload = SpawnLifecyclePayload

// SpawnCreatePatch mutates or denies child-session spawn requests.
type SpawnCreatePatch struct {
	ControlPatch
	AgentName        *string        `json:"agent_name,omitempty"`
	SpawnRole        *string        `json:"spawn_role,omitempty"`
	TTLSeconds       *int64         `json:"ttl_seconds,omitempty"`
	ChildPermissions *PermissionSet `json:"child_permissions,omitempty"`
}

// SpawnObservationPatch is the observation patch surface for committed spawn lifecycle hooks.
type SpawnObservationPatch = AutonomyObservationPatch
