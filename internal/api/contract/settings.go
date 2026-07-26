package contract

type SettingsScopeKind string

const (
	SettingsScopeGlobal    SettingsScopeKind = "global"
	SettingsScopeWorkspace SettingsScopeKind = "workspace"
	SettingsScopeAgent     SettingsScopeKind = "agent"
)

type SettingsGlobalScopeKind string

const (
	SettingsGlobalScope SettingsGlobalScopeKind = "global"
)

type SettingsAgentScopeKind string

const (
	SettingsAgentScopeGlobal SettingsAgentScopeKind = "global"
	SettingsAgentScopeAgent  SettingsAgentScopeKind = "agent"
)

type SettingsWorkspaceScopeKind string

const (
	SettingsWorkspaceScopeGlobal    SettingsWorkspaceScopeKind = "global"
	SettingsWorkspaceScopeWorkspace SettingsWorkspaceScopeKind = "workspace"
)

type SettingsSectionName string

const (
	SettingsSectionGeneral         SettingsSectionName = "general"
	SettingsSectionMemory          SettingsSectionName = "memory"
	SettingsSectionRoles           SettingsSectionName = "roles"
	SettingsSectionSkills          SettingsSectionName = "skills"
	SettingsSectionAutomation      SettingsSectionName = "automation"
	SettingsSectionNetwork         SettingsSectionName = "network"
	SettingsSectionWindowManager   SettingsSectionName = "window-manager"
	SettingsSectionObservability   SettingsSectionName = "observability"
	SettingsSectionHooksExtensions SettingsSectionName = "hooks-extensions"
)

type SettingsCollectionName string

const (
	SettingsCollectionProviders  SettingsCollectionName = "providers"
	SettingsCollectionMCPServers SettingsCollectionName = "mcp-servers"
	SettingsCollectionSandboxes  SettingsCollectionName = "sandboxes"
	SettingsCollectionHooks      SettingsCollectionName = "hooks"
)

type SettingsApplyTargetName string

const (
	SettingsApplyTargetGeneral         SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionGeneral)
	SettingsApplyTargetMemory          SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionMemory)
	SettingsApplyTargetRoles           SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionRoles)
	SettingsApplyTargetSkills          SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionSkills)
	SettingsApplyTargetAutomation      SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionAutomation)
	SettingsApplyTargetNetwork         SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionNetwork)
	SettingsApplyTargetWindowManager   SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionWindowManager)
	SettingsApplyTargetObservability   SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionObservability)
	SettingsApplyTargetHooksExtensions SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionHooksExtensions)
	SettingsApplyTargetProviders       SettingsApplyTargetName = SettingsApplyTargetName(SettingsCollectionProviders)
	SettingsApplyTargetMCPServers      SettingsApplyTargetName = SettingsApplyTargetName(SettingsCollectionMCPServers)
	SettingsApplyTargetSandboxes       SettingsApplyTargetName = SettingsApplyTargetName(SettingsCollectionSandboxes)
	SettingsApplyTargetHooks           SettingsApplyTargetName = SettingsApplyTargetName(SettingsCollectionHooks)
)

type SettingsWriteTargetKind string

const (
	SettingsWriteTargetGlobalConfig        SettingsWriteTargetKind = "global-config"
	SettingsWriteTargetWorkspaceConfig     SettingsWriteTargetKind = "workspace-config"
	SettingsWriteTargetGlobalMCPSidecar    SettingsWriteTargetKind = "global-mcp-sidecar"
	SettingsWriteTargetWorkspaceMCPSidecar SettingsWriteTargetKind = "workspace-mcp-sidecar"
	SettingsWriteTargetGlobalAgentFile     SettingsWriteTargetKind = "global-agent-file"
	SettingsWriteTargetWorkspaceAgentFile  SettingsWriteTargetKind = "workspace-agent-file"
)

type SettingsTargetSelector string

const (
	SettingsTargetAuto    SettingsTargetSelector = "auto"
	SettingsTargetConfig  SettingsTargetSelector = "config"
	SettingsTargetSidecar SettingsTargetSelector = "sidecar"
)

type SettingsMutationBehavior string

const (
	SettingsMutationBehaviorAppliedNow      SettingsMutationBehavior = "applied_now"
	SettingsMutationBehaviorRestartRequired SettingsMutationBehavior = "restart_required"
	SettingsMutationBehaviorActionTrigger   SettingsMutationBehavior = "action_trigger"
)

type SettingsApplyLifecycle string

const (
	SettingsApplyLifecycleLive               SettingsApplyLifecycle = "live"
	SettingsApplyLifecycleLiveAdd            SettingsApplyLifecycle = "live-add"
	SettingsApplyLifecycleLiveRemoveIfUnused SettingsApplyLifecycle = "live-remove-if-unused"
	SettingsApplyLifecycleRestartRequired    SettingsApplyLifecycle = "restart-required"
	SettingsApplyLifecycleSessionRebind      SettingsApplyLifecycle = "session-rebind"
)

type ConfigApplyStatus string

const (
	ConfigApplyStatusPendingApply ConfigApplyStatus = "pending_apply"
	ConfigApplyStatusApplied      ConfigApplyStatus = "applied"
	ConfigApplyStatusBlocked      ConfigApplyStatus = "blocked"
	ConfigApplyStatusFailed       ConfigApplyStatus = "failed"
)

type SettingsApplyNextAction string

const (
	SettingsApplyNextActionNone          SettingsApplyNextAction = "none"
	SettingsApplyNextActionRestartDaemon SettingsApplyNextAction = "restart-daemon"
	SettingsApplyNextActionNewSession    SettingsApplyNextAction = "new-session"
	SettingsApplyNextActionRetry         SettingsApplyNextAction = "retry"
)

type SettingsPermissionMode string

const (
	SettingsPermissionModeDenyAll      SettingsPermissionMode = "deny-all"
	SettingsPermissionModeApproveReads SettingsPermissionMode = "approve-reads"
	SettingsPermissionModeApproveAll   SettingsPermissionMode = "approve-all"
)

type SettingsSourceKind string

const (
	SettingsSourceBuiltinProvider     SettingsSourceKind = "builtin-provider"
	SettingsSourceGlobalConfig        SettingsSourceKind = "global-config"
	SettingsSourceWorkspaceConfig     SettingsSourceKind = "workspace-config"
	SettingsSourceGlobalMCPSidecar    SettingsSourceKind = "global-mcp-sidecar"
	SettingsSourceWorkspaceMCPSidecar SettingsSourceKind = "workspace-mcp-sidecar"
	SettingsSourceGlobalAgentFile     SettingsSourceKind = "global-agent-file"
	SettingsSourceWorkspaceAgentFile  SettingsSourceKind = "workspace-agent-file"
)

type RestartOperationStatus string

const (
	RestartOperationPending        RestartOperationStatus = "pending"
	RestartOperationStopping       RestartOperationStatus = "stopping"
	RestartOperationWaitingRelease RestartOperationStatus = "waiting_release"
	RestartOperationStarting       RestartOperationStatus = "starting"
	RestartOperationReady          RestartOperationStatus = "ready"
	RestartOperationFailed         RestartOperationStatus = "failed"
)

type SettingsStreamTransport string

const (
	SettingsStreamTransportSSE SettingsStreamTransport = "sse"
)

type SettingsUpdateStatusKind string

const (
	SettingsUpdateStatusCurrent     SettingsUpdateStatusKind = "current"
	SettingsUpdateStatusAvailable   SettingsUpdateStatusKind = "available"
	SettingsUpdateStatusUpdated     SettingsUpdateStatusKind = "updated"
	SettingsUpdateStatusDeferred    SettingsUpdateStatusKind = "deferred"
	SettingsUpdateStatusUnsupported SettingsUpdateStatusKind = "unsupported"
	SettingsUpdateStatusFailed      SettingsUpdateStatusKind = "failed"
)
