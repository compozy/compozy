package hooks

// SandboxProfilePayload is the sandbox profile snapshot exposed to sandbox hooks.
type SandboxProfilePayload struct {
	Profile        string            `json:"profile,omitempty"`
	Backend        string            `json:"backend,omitempty"`
	SyncMode       string            `json:"sync_mode,omitempty"`
	Persistence    string            `json:"persistence,omitempty"`
	RuntimeRootDir string            `json:"runtime_root,omitempty"`
	DestroyOnStop  bool              `json:"destroy_on_stop,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	SecretEnv      map[string]string `json:"secret_env,omitempty"`
}

// SandboxPreparePayload is delivered before a session sandbox is prepared.
type SandboxPreparePayload struct {
	PayloadBase
	SessionContext
	SandboxID           string                `json:"sandbox_id,omitempty"`
	Backend             string                `json:"backend,omitempty"`
	Profile             SandboxProfilePayload `json:"profile"`
	LocalRootDir        string                `json:"local_root,omitempty"`
	LocalAdditionalDirs []string              `json:"local_additional_dirs,omitempty"`
	AgentCommand        string                `json:"agent_command,omitempty"`
	AgentEnv            []string              `json:"agent_env,omitempty"`
	Permissions         string                `json:"permissions,omitempty"`
	ResumeACPState      string                `json:"resume_acp_state,omitempty"`
	EnvOverrides        map[string]string     `json:"env_overrides,omitempty"`
	Denied              bool                  `json:"denied,omitempty"`
	DenyReason          string                `json:"deny_reason,omitempty"`
}

// SandboxReadyPayload is delivered after a sandbox has been prepared and synchronized.
type SandboxReadyPayload struct {
	PayloadBase
	SessionContext
	SandboxID             string   `json:"sandbox_id,omitempty"`
	Backend               string   `json:"backend,omitempty"`
	Profile               string   `json:"profile,omitempty"`
	InstanceID            string   `json:"instance_id,omitempty"`
	RuntimeRootDir        string   `json:"runtime_root,omitempty"`
	RuntimeAdditionalDirs []string `json:"runtime_additional_dirs,omitempty"`
}

// SandboxSyncBeforePayload is delivered before a sandbox sync operation runs.
type SandboxSyncBeforePayload struct {
	PayloadBase
	SessionContext
	SandboxID       string   `json:"sandbox_id,omitempty"`
	Backend         string   `json:"backend,omitempty"`
	Profile         string   `json:"profile,omitempty"`
	InstanceID      string   `json:"instance_id,omitempty"`
	RuntimeRootDir  string   `json:"runtime_root,omitempty"`
	Direction       string   `json:"direction,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
	Denied          bool     `json:"denied,omitempty"`
	DenyReason      string   `json:"deny_reason,omitempty"`
}

// SandboxSyncAfterPayload is delivered after a sandbox sync operation finishes.
type SandboxSyncAfterPayload struct {
	PayloadBase
	SessionContext
	SandboxID        string   `json:"sandbox_id,omitempty"`
	Backend          string   `json:"backend,omitempty"`
	Profile          string   `json:"profile,omitempty"`
	InstanceID       string   `json:"instance_id,omitempty"`
	RuntimeRootDir   string   `json:"runtime_root,omitempty"`
	Direction        string   `json:"direction,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	FilesSynced      int      `json:"files_synced,omitempty"`
	BytesTransferred int64    `json:"bytes_transferred,omitempty"`
	DurationMS       int64    `json:"duration_ms,omitempty"`
	Errors           []string `json:"errors,omitempty"`
}

// SandboxStopPayload is delivered before sandbox teardown.
type SandboxStopPayload struct {
	PayloadBase
	SessionContext
	SandboxID      string `json:"sandbox_id,omitempty"`
	Backend        string `json:"backend,omitempty"`
	Profile        string `json:"profile,omitempty"`
	InstanceID     string `json:"instance_id,omitempty"`
	RuntimeRootDir string `json:"runtime_root,omitempty"`
	StopReason     string `json:"stop_reason,omitempty"`
	WillDestroy    bool   `json:"will_destroy,omitempty"`
	Denied         bool   `json:"denied,omitempty"`
	DenyReason     string `json:"deny_reason,omitempty"`
}

// SandboxPreparePatch mutates or denies sandbox preparation.
type SandboxPreparePatch struct {
	ControlPatch
	EnvOverrides map[string]string `json:"env_overrides,omitempty"`
}

// SandboxSyncBeforePatch mutates or denies sandbox sync.
type SandboxSyncBeforePatch struct {
	ControlPatch
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
}

// SandboxObservationPatch is the no-op patch surface for sandbox observation hooks.
type SandboxObservationPatch struct{}

// SandboxReadyPatch is the ready patch surface.
type SandboxReadyPatch = SandboxObservationPatch

// SandboxSyncAfterPatch is the sync-after patch surface.
type SandboxSyncAfterPatch = SandboxObservationPatch

// SandboxStopPatch mutates or denies sandbox teardown.
type SandboxStopPatch struct {
	ControlPatch
}
