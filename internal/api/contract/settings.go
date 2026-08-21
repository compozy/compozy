package contract

type SettingsScopeKind string

const (
	SettingsScopeUser      SettingsScopeKind = "user"
	SettingsScopeProfile   SettingsScopeKind = "profile"
	SettingsScopeWorkspace SettingsScopeKind = "workspace"
	SettingsScopeAgent     SettingsScopeKind = "agent"
)

type SettingsUserScopeKind string

const (
	SettingsUserScope SettingsUserScopeKind = "user"
)

type SettingsAgentScopeKind string

const (
	SettingsAgentScopeUser  SettingsAgentScopeKind = "user"
	SettingsAgentScopeAgent SettingsAgentScopeKind = "agent"
)

type SettingsLayeredScopeKind string

const (
	SettingsLayeredScopeUser      SettingsLayeredScopeKind = "user"
	SettingsLayeredScopeProfile   SettingsLayeredScopeKind = "profile"
	SettingsLayeredScopeWorkspace SettingsLayeredScopeKind = "workspace"
)

type SettingsWorkspaceScopeKind string

const (
	SettingsWorkspaceScopeUser      SettingsWorkspaceScopeKind = "user"
	SettingsWorkspaceScopeWorkspace SettingsWorkspaceScopeKind = "workspace"
)

type SettingsSectionName string

const (
	SettingsSectionGeneral         SettingsSectionName = "general"
	SettingsSectionPersona         SettingsSectionName = "persona"
	SettingsSectionMemory          SettingsSectionName = "memory"
	SettingsSectionRoles           SettingsSectionName = "roles"
	SettingsSectionSkills          SettingsSectionName = "skills"
	SettingsSectionAutomation      SettingsSectionName = "automation"
	SettingsSectionNetwork         SettingsSectionName = "network"
	SettingsSectionWindowManager   SettingsSectionName = "window-manager"
	SettingsSectionCmdPalette      SettingsSectionName = "cmd-palette"
	SettingsSectionAttention       SettingsSectionName = "attention"
	SettingsSectionShell           SettingsSectionName = "shell"
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
	SettingsApplyTargetPersona         SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionPersona)
	SettingsApplyTargetMemory          SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionMemory)
	SettingsApplyTargetRoles           SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionRoles)
	SettingsApplyTargetSkills          SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionSkills)
	SettingsApplyTargetAutomation      SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionAutomation)
	SettingsApplyTargetNetwork         SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionNetwork)
	SettingsApplyTargetWindowManager   SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionWindowManager)
	SettingsApplyTargetCmdPalette      SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionCmdPalette)
	SettingsApplyTargetAttention       SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionAttention)
	SettingsApplyTargetShell           SettingsApplyTargetName = SettingsApplyTargetName(SettingsSectionShell)
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
	SettingsWriteTargetProfileConfig       SettingsWriteTargetKind = "profile-config"
	SettingsWriteTargetWorkspaceConfig     SettingsWriteTargetKind = "workspace-config"
	SettingsWriteTargetGlobalMCPSidecar    SettingsWriteTargetKind = "global-mcp-sidecar"
	SettingsWriteTargetProfileMCPSidecar   SettingsWriteTargetKind = "profile-mcp-sidecar"
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
	SettingsSourceBuiltinProvider            SettingsSourceKind = "builtin-provider"
	SettingsSourceGlobalConfig               SettingsSourceKind = "global-config"
	SettingsSourceProfileConfig              SettingsSourceKind = "profile-config"
	SettingsSourceWorkspaceConfig            SettingsSourceKind = "workspace-config"
	SettingsSourceWorkspaceProfileConfig     SettingsSourceKind = "workspace-profile-config"
	SettingsSourceGlobalMCPSidecar           SettingsSourceKind = "global-mcp-sidecar"
	SettingsSourceProfileMCPSidecar          SettingsSourceKind = "profile-mcp-sidecar"
	SettingsSourceWorkspaceMCPSidecar        SettingsSourceKind = "workspace-mcp-sidecar"
	SettingsSourceWorkspaceProfileMCPSidecar SettingsSourceKind = "workspace-profile-mcp-sidecar"
	SettingsSourceGlobalAgentFile            SettingsSourceKind = "global-agent-file"
	SettingsSourceWorkspaceAgentFile         SettingsSourceKind = "workspace-agent-file"
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
	SettingsUpdateStatusUpToDate    SettingsUpdateStatusKind = "up-to-date"
	SettingsUpdateStatusAvailable   SettingsUpdateStatusKind = "available"
	SettingsUpdateStatusAccepted    SettingsUpdateStatusKind = "accepted"
	SettingsUpdateStatusApplying    SettingsUpdateStatusKind = "applying"
	SettingsUpdateStatusStaged      SettingsUpdateStatusKind = "staged"
	SettingsUpdateStatusUpdated     SettingsUpdateStatusKind = "updated"
	SettingsUpdateStatusBlocked     SettingsUpdateStatusKind = "blocked"
	SettingsUpdateStatusUnsupported SettingsUpdateStatusKind = "unsupported"
	SettingsUpdateStatusFailed      SettingsUpdateStatusKind = "failed"
	SettingsUpdateStatusCanceled    SettingsUpdateStatusKind = "canceled"
)

type SettingsUpdateTarget string

const (
	SettingsUpdateTargetRuntime SettingsUpdateTarget = "runtime"
	SettingsUpdateTargetApp     SettingsUpdateTarget = "app"
)

type SettingsUpdateApplyStatus string

const (
	SettingsUpdateApplyAccepted SettingsUpdateApplyStatus = "accepted"
	SettingsUpdateApplyBlocked  SettingsUpdateApplyStatus = "blocked"
	SettingsUpdateApplyFailed   SettingsUpdateApplyStatus = "failed"
)

type SettingsUpdatePhase string

const (
	SettingsUpdatePhasePending          SettingsUpdatePhase = "pending"
	SettingsUpdatePhaseDownloading      SettingsUpdatePhase = "downloading"
	SettingsUpdatePhaseVerifying        SettingsUpdatePhase = "verifying"
	SettingsUpdatePhaseSwapping         SettingsUpdatePhase = "swapping"
	SettingsUpdatePhaseRestarting       SettingsUpdatePhase = "restarting"
	SettingsUpdatePhaseHealthChecking   SettingsUpdatePhase = "health-checking"
	SettingsUpdatePhaseFinalized        SettingsUpdatePhase = "finalized"
	SettingsUpdatePhaseRolledBack       SettingsUpdatePhase = "rolled-back"
	SettingsUpdatePhaseFailed           SettingsUpdatePhase = "failed"
	SettingsUpdatePhaseStaged           SettingsUpdatePhase = "staged"
	SettingsUpdatePhaseApplying         SettingsUpdatePhase = "applying"
	SettingsUpdatePhaseInstallerHandoff SettingsUpdatePhase = "installer-handoff"
	SettingsUpdatePhaseRestarted        SettingsUpdatePhase = "restarted"
	SettingsUpdatePhaseVerified         SettingsUpdatePhase = "verified"
)

type SettingsUpdateWaitingState string

const (
	SettingsUpdateWaitingNone   SettingsUpdateWaitingState = ""
	SettingsUpdateWaitingForApp SettingsUpdateWaitingState = "waiting-for-app"
)

type SettingsUpdateActor string

const (
	SettingsUpdateActorCLI    SettingsUpdateActor = "cli"
	SettingsUpdateActorDaemon SettingsUpdateActor = "daemon"
	SettingsUpdateActorWeb    SettingsUpdateActor = "web"
	SettingsUpdateActorShell  SettingsUpdateActor = "shell"
)

type SettingsUpdateInstallMethod string

const (
	SettingsUpdateInstallDirectBinary SettingsUpdateInstallMethod = "direct-binary"
	SettingsUpdateInstallHomebrew     SettingsUpdateInstallMethod = "homebrew"
	SettingsUpdateInstallNPM          SettingsUpdateInstallMethod = "npm"
	SettingsUpdateInstallAPT          SettingsUpdateInstallMethod = "apt"
	SettingsUpdateInstallDNF          SettingsUpdateInstallMethod = "dnf"
	SettingsUpdateInstallRPM          SettingsUpdateInstallMethod = "rpm"
	SettingsUpdateInstallScoop        SettingsUpdateInstallMethod = "scoop"
	SettingsUpdateInstallGo           SettingsUpdateInstallMethod = "go-install"
	SettingsUpdateInstallDesktopApp   SettingsUpdateInstallMethod = "desktop-app"
	SettingsUpdateInstallUnknown      SettingsUpdateInstallMethod = "unknown"
)
