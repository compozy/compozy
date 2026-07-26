package spec

import (
	"reflect"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/hooks"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
)

var schemaEnumValues = withSettingsWindowManagerSchemaEnumValues(
	withGoalSchemaEnumValues(map[reflect.Type][]string{
		reflect.TypeFor[contract.DrainState]():                       drainStateValues(),
		reflect.TypeFor[automationpkg.Scope]():                       automationScopeValues(),
		reflect.TypeFor[automationpkg.JobSource]():                   automationSourceValues(),
		reflect.TypeFor[automationpkg.ScheduleMode]():                automationScheduleModeValues(),
		reflect.TypeFor[automationpkg.SchedulerCatchUpPolicy]():      automationSchedulerCatchUpPolicyValues(),
		reflect.TypeFor[automationpkg.RetryStrategy]():               automationRetryStrategyValues(),
		reflect.TypeFor[automationpkg.RunStatus]():                   automationRunStatusValues(),
		reflect.TypeFor[taskpkg.Scope]():                             taskScopeValues(),
		reflect.TypeFor[taskpkg.Status]():                            taskStatusValues(),
		reflect.TypeFor[taskpkg.Priority]():                          taskPriorityValues(),
		reflect.TypeFor[taskpkg.ApprovalPolicy]():                    taskApprovalPolicyValues(),
		reflect.TypeFor[taskpkg.ApprovalState]():                     taskApprovalStateValues(),
		reflect.TypeFor[taskpkg.RunStatus]():                         taskRunStatusValues(),
		reflect.TypeFor[taskpkg.ActorKind]():                         taskActorKindValues(),
		reflect.TypeFor[taskpkg.OwnerKind]():                         taskOwnerKindValues(),
		reflect.TypeFor[taskpkg.OriginKind]():                        taskOriginKindValues(),
		reflect.TypeFor[taskpkg.DependencyKind]():                    taskDependencyKindValues(),
		reflect.TypeFor[taskpkg.BlockKind]():                         taskBlockKindValues(),
		reflect.TypeFor[taskpkg.BlockedSource]():                     taskBlockedSourceValues(),
		reflect.TypeFor[taskpkg.CoordinatorMode]():                   taskCoordinatorModeValues(),
		reflect.TypeFor[taskpkg.WorkerMode]():                        taskWorkerModeValues(),
		reflect.TypeFor[taskpkg.SandboxMode]():                       taskSandboxModeValues(),
		reflect.TypeFor[taskpkg.RuntimeMode]():                       taskRuntimeModeValues(),
		reflect.TypeFor[taskpkg.ReviewPolicy]():                      taskReviewPolicyValues(),
		reflect.TypeFor[taskpkg.RunReviewStatus]():                   taskRunReviewStatusValues(),
		reflect.TypeFor[taskpkg.RunReviewOutcome]():                  taskRunReviewOutcomeValues(),
		reflect.TypeFor[contract.TaskInboxLane]():                    taskInboxLaneValues(),
		reflect.TypeFor[contract.CoordinationMessageKind]():          coordinationMessageKindValues(),
		reflect.TypeFor[contract.AgentCreateScope]():                 agentCreateScopeValues(),
		reflect.TypeFor[contract.AgentOrigin]():                      agentOriginValues(),
		reflect.TypeFor[contract.CoordinatorConfigSource]():          coordinatorConfigSourceValues(),
		reflect.TypeFor[contract.RoleResolutionMode]():               roleResolutionModeValues(),
		reflect.TypeFor[contract.LoopSource]():                       loopSourceValues(),
		reflect.TypeFor[contract.LoopRunStatus]():                    loopRunStatusValues(),
		reflect.TypeFor[contract.LoopRunEventKind]():                 loopRunEventKindValues(),
		reflect.TypeFor[contract.LoopWatchEventKind]():               contract.LoopWatchEventKindValues(),
		reflect.TypeFor[contract.LoopNodeClass]():                    loopNodeClassValues(),
		reflect.TypeFor[contract.LoopReattemptStrategy]():            loopReattemptStrategyValues(),
		reflect.TypeFor[contract.LoopBudgetExceeded]():               loopBudgetExceededValues(),
		reflect.TypeFor[contract.LoopGateDecision]():                 loopGateDecisionValues(),
		reflect.TypeFor[contract.LoopLintSeverity]():                 loopLintSeverityValues(),
		reflect.TypeFor[contract.AuthoredValidationStatus]():         contract.AuthoredValidationStatusValues(),
		reflect.TypeFor[contract.AuthoredDiagnosticSeverity]():       contract.AuthoredDiagnosticSeverityValues(),
		reflect.TypeFor[contract.AgentSoulRevisionAction]():          contract.AgentSoulRevisionActionValues(),
		reflect.TypeFor[contract.HeartbeatRevisionOperation]():       contract.HeartbeatRevisionOperationValues(),
		reflect.TypeFor[contract.HeartbeatActorKind]():               contract.HeartbeatActorKindValues(),
		reflect.TypeFor[contract.SessionHealthState]():               contract.SessionHealthStateValues(),
		reflect.TypeFor[contract.SessionHealthStatus]():              contract.SessionHealthStatusValues(),
		reflect.TypeFor[contract.SessionHealthIneligibilityReason](): contract.SessionHealthIneligibilityReasonValues(),
		reflect.TypeFor[contract.HeartbeatWakeSource]():              contract.HeartbeatWakeSourceValues(),
		reflect.TypeFor[contract.HeartbeatWakeResult]():              contract.HeartbeatWakeResultValues(),
		reflect.TypeFor[contract.HeartbeatWakeReason]():              contract.HeartbeatWakeReasonValues(),
		reflect.TypeFor[contract.WindowManagerErrorCode]():           contract.WindowManagerErrorCodeValues(),
		reflect.TypeFor[contract.WindowManagerCommandID]():           contract.WindowManagerCommandIDValues(),
		reflect.TypeFor[windowmanager.DesktopPurpose](): {
			string(windowmanager.DesktopPurposeStandard),
			string(windowmanager.DesktopPurposeFocus),
		},
		reflect.TypeFor[windowmanager.NodeKind](): {
			string(windowmanager.NodeKindLeaf),
			string(windowmanager.NodeKindSplit),
			string(windowmanager.NodeKindStack),
		},
		reflect.TypeFor[windowmanager.Axis](): {
			string(windowmanager.AxisHorizontal),
			string(windowmanager.AxisVertical),
		},
		reflect.TypeFor[windowmanager.WindowPlacement](): {
			string(windowmanager.WindowPlacementTiled),
			string(windowmanager.WindowPlacementStacked),
			string(windowmanager.WindowPlacementFloating),
		},
		reflect.TypeFor[windowmanager.FocusDirection](): {
			string(windowmanager.FocusLeft),
			string(windowmanager.FocusRight),
			string(windowmanager.FocusUp),
			string(windowmanager.FocusDown),
		},
		reflect.TypeFor[windowmanager.DropPlacement](): {
			string(windowmanager.DropFloating),
			string(windowmanager.DropBefore),
			string(windowmanager.DropAfter),
			string(windowmanager.DropLeft),
			string(windowmanager.DropRight),
			string(windowmanager.DropTop),
			string(windowmanager.DropBottom),
			string(windowmanager.DropCenter),
		},
		reflect.TypeFor[windowmanager.Arrangement](): {
			string(windowmanager.ArrangementHorizontal),
			string(windowmanager.ArrangementVertical),
			string(windowmanager.ArrangementGrid),
			string(windowmanager.ArrangementStack),
		},
		reflect.TypeFor[contract.SkillDiagnosticState]():       skillDiagnosticStateValues(),
		reflect.TypeFor[contract.SkillActivationReasonCode]():  skillActivationReasonCodeValues(),
		reflect.TypeFor[contract.SkillVerificationStatus]():    skillVerificationStatusValues(),
		reflect.TypeFor[contract.BridgeSendTestStatus]():       contract.BridgeSendTestStatusValues(),
		reflect.TypeFor[hooks.HookEvent]():                     hookEventValues(),
		reflect.TypeFor[hooks.HookEventFamily]():               hookEventFamilyValues(),
		reflect.TypeFor[hooks.HookMode]():                      hookModeValues(),
		reflect.TypeFor[hooks.HookRunOutcome]():                hookOutcomeValues(),
		reflect.TypeFor[hooks.HookSkillSource]():               hookSkillSourceValues(),
		reflect.TypeFor[hooks.HookExecutorKind]():              hookExecutorKindValues(),
		reflect.TypeFor[hooks.HookSource]():                    hookSourceValues(),
		reflect.TypeFor[memcontract.Type]():                    memoryTypeValues(),
		reflect.TypeFor[memcontract.Scope]():                   memoryScopeValues(),
		reflect.TypeFor[memcontract.AgentTier]():               memoryAgentTierValues(),
		reflect.TypeFor[memcontract.Origin]():                  memoryOriginValues(),
		reflect.TypeFor[memcontract.Operation]():               memoryOperationValues(),
		reflect.TypeFor[memcontract.DecisionSource]():          memoryDecisionSourceValues(),
		reflect.TypeFor[memcontract.Trigger]():                 memoryTriggerValues(),
		reflect.TypeFor[contract.MemoryDecisionOp]():           memoryDecisionOpValues(),
		reflect.TypeFor[contract.MemoryProviderState]():        memoryProviderStateValues(),
		reflect.TypeFor[contract.MemoryDreamState]():           memoryDreamStateValues(),
		reflect.TypeFor[contract.MemoryExtractorState]():       memoryExtractorStateValues(),
		reflect.TypeFor[contract.SettingsScopeKind]():          settingsScopeValues(),
		reflect.TypeFor[contract.SettingsGlobalScopeKind]():    settingsGlobalScopeValues(),
		reflect.TypeFor[contract.SettingsAgentScopeKind]():     settingsAgentScopeValues(),
		reflect.TypeFor[contract.SettingsWorkspaceScopeKind](): settingsWorkspaceScopeValues(),
		reflect.TypeFor[contract.SettingsSectionName]():        settingsSectionValues(),
		reflect.TypeFor[contract.SettingsApplyTargetName]():    settingsApplyTargetValues(),
		reflect.TypeFor[contract.SettingsCollectionName]():     settingsCollectionValues(),
		reflect.TypeFor[contract.SettingsWriteTargetKind]():    settingsWriteTargetValues(),
		reflect.TypeFor[contract.SettingsTargetSelector]():     settingsTargetSelectorValues(),
		reflect.TypeFor[contract.SettingsMutationBehavior]():   settingsMutationBehaviorValues(),
		reflect.TypeFor[contract.SettingsApplyLifecycle]():     settingsApplyLifecycleValues(),
		reflect.TypeFor[contract.ConfigApplyStatus]():          configApplyStatusValues(),
		reflect.TypeFor[contract.SettingsApplyNextAction]():    settingsApplyNextActionValues(),
		reflect.TypeFor[contract.SettingsPermissionMode]():     settingsPermissionModeValues(),
		reflect.TypeFor[contract.SettingsSourceKind]():         settingsSourceKindValues(),
		reflect.TypeFor[contract.RestartOperationStatus]():     restartOperationStatusValues(),
		reflect.TypeFor[contract.SettingsStreamTransport]():    settingsStreamTransportValues(),
		reflect.TypeFor[contract.SettingsUpdateStatusKind]():   settingsUpdateStatusValues(),
		reflect.TypeFor[contract.SettingsMCPAuthBeginMode](): {
			string(contract.SettingsMCPAuthBeginModeAutomatic),
			string(contract.SettingsMCPAuthBeginModeManual),
		},
		reflect.TypeFor[contract.SettingsMCPInstallNextStep](): {
			string(contract.SettingsMCPInstallNextStepNone),
			string(contract.SettingsMCPInstallNextStepAuthorize),
		},
		reflect.TypeFor[resources.ResourceScopeKind]():        resourceScopeKindValues(),
		reflect.TypeFor[bridgepkg.Scope]():                    bridgeScopeValues(),
		reflect.TypeFor[bridgepkg.BridgeInstanceSource]():     bridgeInstanceSourceValues(),
		reflect.TypeFor[bridgepkg.BridgeStatus]():             bridgeStatusValues(),
		reflect.TypeFor[bridgepkg.BridgeDMPolicy]():           bridgeDMPolicyValues(),
		reflect.TypeFor[bridgepkg.BridgeDegradationReason]():  bridgeDegradationReasonValues(),
		reflect.TypeFor[bridgepkg.BridgeDiagnosticKind]():     bridgeDiagnosticKindValues(),
		reflect.TypeFor[bridgepkg.BridgeDiagnosticSeverity](): bridgeDiagnosticSeverityValues(),
		reflect.TypeFor[bridgepkg.BridgeCheckStatus]():        bridgeCheckStatusValues(),
		reflect.TypeFor[bridgepkg.DeliveryMode]():             deliveryModeValues(),
		reflect.TypeFor[modelcatalog.ReasoningEffort]():       modelcatalog.ReasoningEffortValues(),
		reflect.TypeFor[modelcatalog.ReasoningSource]():       modelcatalog.ReasoningSourceValues(),
		reflect.TypeFor[modelcatalog.CostStatus]():            modelcatalog.CostStatusValues(),
		reflect.TypeFor[modelcatalog.CostSource]():            modelcatalog.CostSourceValues(),
		reflect.TypeFor[session.Type]():                       sessionTypeValues(),
		reflect.TypeFor[session.State]():                      sessionStateValues(),
		reflect.TypeFor[store.StopReason]():                   stopReasonValues(),
		reflect.TypeFor[tools.ToolSource]():                   toolSourceValues(),
		reflect.TypeFor[tools.BackendKind]():                  toolBackendKindValues(),
		reflect.TypeFor[tools.Visibility]():                   toolVisibilityValues(),
		reflect.TypeFor[tools.RiskClass]():                    toolRiskClassValues(),
		reflect.TypeFor[tools.ReasonCode]():                   toolReasonCodeValues(),
		reflect.TypeFor[tools.ErrorCode]():                    toolErrorCodeValues(),
		reflect.TypeFor[tools.ToolCallEventKind]():            toolCallEventKindValues(),
		reflect.TypeFor[tools.ApprovalGrantDecision]():        toolApprovalGrantDecisionValues(),
		reflect.TypeFor[tools.ApprovalGrantManagementScope](): toolApprovalGrantManagementScopeValues(),
		reflect.TypeFor[extensionprotocol.HostAPIMethod]():    hostAPIMethodValues(),
		reflect.TypeFor[participation.Mode]():                 participationModeValues(),
		reflect.TypeFor[participation.ChannelStrategy]():      participationChannelStrategyValues(),
		reflect.TypeFor[participation.Source]():               participationSourceValues(),
		reflect.TypeFor[participation.OwnerKind]():            participationOwnerKindValues(),
	}),
)

func roleResolutionModeValues() []string {
	return []string{
		string(contract.RoleResolutionModeBuiltin),
		string(contract.RoleResolutionModeCatalog),
		string(contract.RoleResolutionModeInherit),
	}
}

func drainStateValues() []string {
	return []string{string(contract.DrainStateActive), string(contract.DrainStateDraining)}
}

func toolApprovalGrantDecisionValues() []string {
	return []string{string(tools.ApprovalGrantAllow), string(tools.ApprovalGrantReject)}
}

func toolApprovalGrantManagementScopeValues() []string {
	return []string{string(tools.ApprovalGrantScopeAgent), string(tools.ApprovalGrantScopeTool)}
}
