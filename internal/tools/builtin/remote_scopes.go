package builtin

import (
	"strings"

	"github.com/compozy/compozy/internal/remoteproto"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// RemoteToolScope declares the operator tools that can run as a remote device.
// New tools opt in after their inputs and caller requirements have been reviewed.
func RemoteToolScope(id string) (remoteproto.Scope, bool) {
	toolID := toolspkg.ToolID(strings.TrimSpace(id))
	if remoteSessionTool(toolID) {
		return remoteproto.ScopeSessions, true
	}
	switch toolID {
	case toolspkg.ToolIDTerminalExec, toolspkg.ToolIDTerminalOpen, toolspkg.ToolIDTerminalRead,
		toolspkg.ToolIDTerminalWrite, toolspkg.ToolIDTerminalSignal, toolspkg.ToolIDTerminalClose,
		toolspkg.ToolIDTerminalWait, toolspkg.ToolIDTerminalList:
		return remoteproto.ScopeTerminal, true
	case toolspkg.ToolIDSessionChanges,
		toolspkg.ToolIDWorktreeList,
		toolspkg.ToolIDWorktreeInspect,
		toolspkg.ToolIDLoopDiff:
		return remoteproto.ScopeFilesR, true
	case toolspkg.ToolIDWorktreeCreate, toolspkg.ToolIDWorktreeRemove:
		return remoteproto.ScopeFilesW, true
	case toolspkg.ToolIDTaskList, toolspkg.ToolIDTaskRead, toolspkg.ToolIDTaskCreate,
		toolspkg.ToolIDTaskChildCreate, toolspkg.ToolIDTaskUpdate, toolspkg.ToolIDTaskCancel,
		toolspkg.ToolIDTaskBlocks, toolspkg.ToolIDTaskUnblock, toolspkg.ToolIDTaskRecover,
		toolspkg.ToolIDTaskExecutionProfileGet, toolspkg.ToolIDTaskExecutionProfileSet,
		toolspkg.ToolIDTaskExecutionProfileDelete, toolspkg.ToolIDTaskWorktreePolicySet,
		toolspkg.ToolIDTaskFanOutRuns:
		return remoteproto.ScopeTasks, true
	case toolspkg.ToolIDToolList, toolspkg.ToolIDToolSearch, toolspkg.ToolIDToolInfo,
		toolspkg.ToolIDSkillList, toolspkg.ToolIDSkillSearch, toolspkg.ToolIDSkillView,
		toolspkg.ToolIDAgentList, toolspkg.ToolIDAgentCreate, toolspkg.ToolIDAgentDraftInstructions,
		toolspkg.ToolIDWorkspaceArchive, toolspkg.ToolIDWorkspaceUnarchive,
		toolspkg.ToolIDListLogs, toolspkg.ToolIDObserveSearch,
		toolspkg.ToolIDConfigShow, toolspkg.ToolIDConfigList, toolspkg.ToolIDConfigGet,
		toolspkg.ToolIDConfigDiff, toolspkg.ToolIDConfigPath,
		toolspkg.ToolIDHooksList, toolspkg.ToolIDHooksInfo, toolspkg.ToolIDHooksEvents, toolspkg.ToolIDHooksRuns,
		toolspkg.ToolIDHooksCreate, toolspkg.ToolIDHooksUpdate, toolspkg.ToolIDHooksDelete,
		toolspkg.ToolIDHooksEnable, toolspkg.ToolIDHooksDisable,
		toolspkg.ToolIDProviderModelsCurate, toolspkg.ToolIDProviderModelsRefresh,
		toolspkg.ToolIDProviderUsageList, toolspkg.ToolIDProviderUsageRefresh,
		toolspkg.ToolIDExtensionsList, toolspkg.ToolIDExtensionsInfo, toolspkg.ToolIDExtensionsInventory,
		toolspkg.ToolIDExtensionsInstall, toolspkg.ToolIDExtensionsUpdate, toolspkg.ToolIDExtensionsRemove,
		toolspkg.ToolIDExtensionsEnable, toolspkg.ToolIDExtensionsDisable, toolspkg.ToolIDExtensionsInit,
		toolspkg.ToolIDExtensionsBuild, toolspkg.ToolIDExtensionsValidate, toolspkg.ToolIDExtensionsDev,
		toolspkg.ToolIDExtensionsReload, toolspkg.ToolIDExtensionsLogs, toolspkg.ToolIDExtensionsSearch,
		toolspkg.ToolIDExtensionsProvenance, toolspkg.ToolIDExtensionsRestore, toolspkg.ToolIDExtensionsRemovedList,
		toolspkg.ToolIDMarketplaceCategories, toolspkg.ToolIDMarketplaceSearch, toolspkg.ToolIDMarketplaceSources,
		toolspkg.ToolIDResourcesList, toolspkg.ToolIDResourcesInfo, toolspkg.ToolIDResourcesSnapshot,
		toolspkg.ToolIDMCPList, toolspkg.ToolIDMCPImport, toolspkg.ToolIDMCPAllow, toolspkg.ToolIDMCPStatus,
		toolspkg.ToolIDMCPAuthStatus, toolspkg.ToolIDSessionMCPList:
		return remoteproto.ScopeSettings, true
	default:
		return "", false
	}
}

func remoteSessionTool(id toolspkg.ToolID) bool {
	switch id {
	case toolspkg.ToolIDSessionList, toolspkg.ToolIDSessionCreate, toolspkg.ToolIDSessionPrompt,
		toolspkg.ToolIDSessionArchive, toolspkg.ToolIDSessionUnarchive, toolspkg.ToolIDSessionCleanup,
		toolspkg.ToolIDSessionLater, toolspkg.ToolIDSessionResumeLater, toolspkg.ToolIDSessionMarkSeen,
		toolspkg.ToolIDSessionRename, toolspkg.ToolIDSessionLink, toolspkg.ToolIDSessionUnlink,
		toolspkg.ToolIDSessionReparent, toolspkg.ToolIDSessionMove, toolspkg.ToolIDSessionMoveToProfile,
		toolspkg.ToolIDSessionReorder, toolspkg.ToolIDSessionFolderList, toolspkg.ToolIDSessionFolderCreate,
		toolspkg.ToolIDSessionFolderRename, toolspkg.ToolIDSessionFolderDelete, toolspkg.ToolIDSessionFolderReorder,
		toolspkg.ToolIDSessionStatus, toolspkg.ToolIDSessionHistory, toolspkg.ToolIDSessionEvents,
		toolspkg.ToolIDSessionDescribe, toolspkg.ToolIDSessionSearch, toolspkg.ToolIDSessionOutline,
		toolspkg.ToolIDSessionStop, toolspkg.ToolIDSessionApprove, toolspkg.ToolIDSessionCompact,
		toolspkg.ToolIDSessionPromptCancel, toolspkg.ToolIDSessionPendingInteractions,
		toolspkg.ToolIDSessionInputsList, toolspkg.ToolIDSessionInputsClear, toolspkg.ToolIDSessionInputReplace,
		toolspkg.ToolIDSessionInputCancel, toolspkg.ToolIDSessionInputPromote, toolspkg.ToolIDSessionRewind,
		toolspkg.ToolIDSessionProcessesList, toolspkg.ToolIDSessionProcessesTerminate, toolspkg.ToolIDSessionHealth,
		toolspkg.ToolIDSessionRuntimeSet, toolspkg.ToolIDSessionRuntimeClear,
		toolspkg.ToolIDSessionWorktreeBind, toolspkg.ToolIDSessionWorktreeUnbind,
		toolspkg.ToolIDWorkspaceList, toolspkg.ToolIDWorkspaceInfo, toolspkg.ToolIDWorkspaceDescribe,
		toolspkg.ToolIDWorkspaceFolderList, toolspkg.ToolIDWorkspaceFolderCreate, toolspkg.ToolIDWorkspaceFolderRename,
		toolspkg.ToolIDWorkspaceFolderDelete, toolspkg.ToolIDWorkspaceFolderReorder,
		toolspkg.ToolIDWorkspaceMove, toolspkg.ToolIDWorkspaceReorder, toolspkg.ToolIDProfileList,
		toolspkg.ToolIDProfileCurrent, toolspkg.ToolIDProviderModelsList, toolspkg.ToolIDProviderModelsStatus,
		toolspkg.ToolIDVaultList, toolspkg.ToolIDToolArtifactRead, toolspkg.ToolIDObserveMetrics,
		toolspkg.ToolIDAutomationJobsList, toolspkg.ToolIDAutomationJobsGet, toolspkg.ToolIDAutomationJobsCreate,
		toolspkg.ToolIDAutomationJobsUpdate, toolspkg.ToolIDAutomationJobsDelete, toolspkg.ToolIDAutomationJobsEnable,
		toolspkg.ToolIDAutomationJobsDisable, toolspkg.ToolIDAutomationJobsTrigger, toolspkg.ToolIDAutomationJobsHistory,
		toolspkg.ToolIDAutomationTriggersList, toolspkg.ToolIDAutomationTriggersGet,
		toolspkg.ToolIDAutomationTriggersCreate, toolspkg.ToolIDAutomationTriggersUpdate,
		toolspkg.ToolIDAutomationTriggersDelete, toolspkg.ToolIDAutomationTriggersEnable,
		toolspkg.ToolIDAutomationTriggersDisable, toolspkg.ToolIDAutomationTriggersHistory,
		toolspkg.ToolIDAutomationRunsList, toolspkg.ToolIDAutomationRunsGet, toolspkg.ToolIDAutomationRunsCancel,
		toolspkg.ToolIDAutomationSuggestionsList, toolspkg.ToolIDAutomationSuggestionsAccept,
		toolspkg.ToolIDAutomationSuggestionsDismiss,
		toolspkg.ToolIDLoopList, toolspkg.ToolIDLoopInspect, toolspkg.ToolIDLoopValidate, toolspkg.ToolIDLoopCreate,
		toolspkg.ToolIDLoopRun, toolspkg.ToolIDLoopStatus, toolspkg.ToolIDLoopRuns, toolspkg.ToolIDLoopTurns,
		toolspkg.ToolIDLoopCancel, toolspkg.ToolIDLoopNodes, toolspkg.ToolIDLoopNodePause, toolspkg.ToolIDLoopNodeResume,
		toolspkg.ToolIDLoopNodeCancel, toolspkg.ToolIDLoopNodeRequeue, toolspkg.ToolIDLoopPause, toolspkg.ToolIDLoopResume,
		toolspkg.ToolIDLoopConfigure, toolspkg.ToolIDLoopApprove, toolspkg.ToolIDLoopRequests, toolspkg.ToolIDLoopRequest,
		toolspkg.ToolIDLoopRespond, toolspkg.ToolIDLoopNodeAmend, toolspkg.ToolIDLoopRerun, toolspkg.ToolIDLoopFork,
		toolspkg.ToolIDLoopDelete:
		return true
	default:
		return false
	}
}
