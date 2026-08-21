package spec

import (
	"github.com/compozy/compozy/internal/api/contract"

	"github.com/compozy/compozy/internal/resources"
)

func resourceScopeKindValues() []string {
	return []string{
		string(resources.ResourceScopeKindUser),
		string(resources.ResourceScopeKindWorkspace),
	}
}

func settingsScopeValues() []string {
	return []string{
		string(contract.SettingsScopeGlobal),
		string(contract.SettingsScopeWorkspace),
		string(contract.SettingsScopeAgent),
	}
}

func settingsGlobalScopeValues() []string {
	return []string{string(contract.SettingsGlobalScope)}
}

func settingsAgentScopeValues() []string {
	return []string{
		string(contract.SettingsAgentScopeGlobal),
		string(contract.SettingsAgentScopeAgent),
	}
}

func settingsWorkspaceScopeValues() []string {
	return []string{
		string(contract.SettingsWorkspaceScopeGlobal),
		string(contract.SettingsWorkspaceScopeWorkspace),
	}
}

func settingsSectionValues() []string {
	return []string{
		string(contract.SettingsSectionGeneral),
		string(contract.SettingsSectionMemory),
		string(contract.SettingsSectionRoles),
		string(contract.SettingsSectionSkills),
		string(contract.SettingsSectionAutomation),
		string(contract.SettingsSectionNetwork),
		string(contract.SettingsSectionWindowManager),
		string(contract.SettingsSectionCmdPalette),
		string(contract.SettingsSectionAttention),
		string(contract.SettingsSectionShell),
		string(contract.SettingsSectionObservability),
		string(contract.SettingsSectionHooksExtensions),
	}
}

func settingsApplyTargetValues() []string {
	return []string{
		string(contract.SettingsApplyTargetGeneral),
		string(contract.SettingsApplyTargetMemory),
		string(contract.SettingsApplyTargetRoles),
		string(contract.SettingsApplyTargetSkills),
		string(contract.SettingsApplyTargetAutomation),
		string(contract.SettingsApplyTargetNetwork),
		string(contract.SettingsApplyTargetWindowManager),
		string(contract.SettingsApplyTargetCmdPalette),
		string(contract.SettingsApplyTargetAttention),
		string(contract.SettingsApplyTargetShell),
		string(contract.SettingsApplyTargetObservability),
		string(contract.SettingsApplyTargetHooksExtensions),
		string(contract.SettingsApplyTargetProviders),
		string(contract.SettingsApplyTargetMCPServers),
		string(contract.SettingsApplyTargetSandboxes),
		string(contract.SettingsApplyTargetHooks),
	}
}

func settingsCollectionValues() []string {
	return []string{
		string(contract.SettingsCollectionProviders),
		string(contract.SettingsCollectionMCPServers),
		string(contract.SettingsCollectionSandboxes),
		string(contract.SettingsCollectionHooks),
	}
}

func settingsWriteTargetValues() []string {
	return []string{
		string(contract.SettingsWriteTargetGlobalConfig),
		string(contract.SettingsWriteTargetWorkspaceConfig),
		string(contract.SettingsWriteTargetGlobalMCPSidecar),
		string(contract.SettingsWriteTargetWorkspaceMCPSidecar),
		string(contract.SettingsWriteTargetGlobalAgentFile),
		string(contract.SettingsWriteTargetWorkspaceAgentFile),
	}
}

func settingsTargetSelectorValues() []string {
	return []string{
		string(contract.SettingsTargetAuto),
		string(contract.SettingsTargetConfig),
		string(contract.SettingsTargetSidecar),
	}
}

func settingsMutationBehaviorValues() []string {
	return []string{
		string(contract.SettingsMutationBehaviorAppliedNow),
		string(contract.SettingsMutationBehaviorRestartRequired),
		string(contract.SettingsMutationBehaviorActionTrigger),
	}
}

func settingsApplyLifecycleValues() []string {
	return []string{
		string(contract.SettingsApplyLifecycleLive),
		string(contract.SettingsApplyLifecycleLiveAdd),
		string(contract.SettingsApplyLifecycleLiveRemoveIfUnused),
		string(contract.SettingsApplyLifecycleRestartRequired),
		string(contract.SettingsApplyLifecycleSessionRebind),
	}
}

func configApplyStatusValues() []string {
	return []string{
		string(contract.ConfigApplyStatusPendingApply),
		string(contract.ConfigApplyStatusApplied),
		string(contract.ConfigApplyStatusBlocked),
		string(contract.ConfigApplyStatusFailed),
	}
}

func settingsApplyNextActionValues() []string {
	return []string{
		string(contract.SettingsApplyNextActionNone),
		string(contract.SettingsApplyNextActionRestartDaemon),
		string(contract.SettingsApplyNextActionNewSession),
		string(contract.SettingsApplyNextActionRetry),
	}
}

func settingsPermissionModeValues() []string {
	return []string{
		string(contract.SettingsPermissionModeDenyAll),
		string(contract.SettingsPermissionModeApproveReads),
		string(contract.SettingsPermissionModeApproveAll),
	}
}

func settingsSourceKindValues() []string {
	return []string{
		string(contract.SettingsSourceBuiltinProvider),
		string(contract.SettingsSourceGlobalConfig),
		string(contract.SettingsSourceWorkspaceConfig),
		string(contract.SettingsSourceGlobalMCPSidecar),
		string(contract.SettingsSourceWorkspaceMCPSidecar),
		string(contract.SettingsSourceGlobalAgentFile),
		string(contract.SettingsSourceWorkspaceAgentFile),
	}
}

func restartOperationStatusValues() []string {
	return []string{
		string(contract.RestartOperationPending),
		string(contract.RestartOperationStopping),
		string(contract.RestartOperationWaitingRelease),
		string(contract.RestartOperationStarting),
		string(contract.RestartOperationReady),
		string(contract.RestartOperationFailed),
	}
}

func settingsStreamTransportValues() []string {
	return []string{
		string(contract.SettingsStreamTransportSSE),
	}
}

func settingsUpdateStatusValues() []string {
	return []string{
		string(contract.SettingsUpdateStatusUpToDate),
		string(contract.SettingsUpdateStatusAvailable),
		string(contract.SettingsUpdateStatusAccepted),
		string(contract.SettingsUpdateStatusApplying),
		string(contract.SettingsUpdateStatusStaged),
		string(contract.SettingsUpdateStatusUpdated),
		string(contract.SettingsUpdateStatusBlocked),
		string(contract.SettingsUpdateStatusUnsupported),
		string(contract.SettingsUpdateStatusFailed),
		string(contract.SettingsUpdateStatusCanceled),
	}
}

func settingsUpdateTargetValues() []string {
	return []string{
		string(contract.SettingsUpdateTargetRuntime),
		string(contract.SettingsUpdateTargetApp),
	}
}

func settingsUpdateApplyStatusValues() []string {
	return []string{
		string(contract.SettingsUpdateApplyAccepted),
		string(contract.SettingsUpdateApplyBlocked),
		string(contract.SettingsUpdateApplyFailed),
	}
}

func settingsUpdatePhaseValues() []string {
	return []string{
		string(contract.SettingsUpdatePhasePending),
		string(contract.SettingsUpdatePhaseDownloading),
		string(contract.SettingsUpdatePhaseVerifying),
		string(contract.SettingsUpdatePhaseSwapping),
		string(contract.SettingsUpdatePhaseRestarting),
		string(contract.SettingsUpdatePhaseHealthChecking),
		string(contract.SettingsUpdatePhaseFinalized),
		string(contract.SettingsUpdatePhaseRolledBack),
		string(contract.SettingsUpdatePhaseFailed),
		string(contract.SettingsUpdatePhaseStaged),
		string(contract.SettingsUpdatePhaseApplying),
		string(contract.SettingsUpdatePhaseInstallerHandoff),
		string(contract.SettingsUpdatePhaseRestarted),
		string(contract.SettingsUpdatePhaseVerified),
	}
}

func settingsUpdateWaitingValues() []string {
	return []string{
		string(contract.SettingsUpdateWaitingNone),
		string(contract.SettingsUpdateWaitingForApp),
	}
}

func settingsUpdateActorValues() []string {
	return []string{
		string(contract.SettingsUpdateActorCLI),
		string(contract.SettingsUpdateActorDaemon),
		string(contract.SettingsUpdateActorWeb),
		string(contract.SettingsUpdateActorShell),
	}
}

func settingsUpdateInstallMethodValues() []string {
	return []string{
		string(contract.SettingsUpdateInstallDirectBinary),
		string(contract.SettingsUpdateInstallHomebrew),
		string(contract.SettingsUpdateInstallNPM),
		string(contract.SettingsUpdateInstallAPT),
		string(contract.SettingsUpdateInstallDNF),
		string(contract.SettingsUpdateInstallRPM),
		string(contract.SettingsUpdateInstallScoop),
		string(contract.SettingsUpdateInstallGo),
		string(contract.SettingsUpdateInstallDesktopApp),
		string(contract.SettingsUpdateInstallUnknown),
	}
}
