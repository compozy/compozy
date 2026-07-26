package spec

import (
	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/resources"
)

func resourceScopeKindValues() []string {
	return []string{
		string(resources.ResourceScopeKindGlobal),
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
		string(contract.SettingsUpdateStatusCurrent),
		string(contract.SettingsUpdateStatusAvailable),
		string(contract.SettingsUpdateStatusUpdated),
		string(contract.SettingsUpdateStatusDeferred),
		string(contract.SettingsUpdateStatusUnsupported),
		string(contract.SettingsUpdateStatusFailed),
	}
}
