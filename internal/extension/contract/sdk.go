package contract

import (
	apicontract "github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/hooks"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/tools"
)

const (
	sdkAgentHeartbeatPolicyResolvedPayloadValue = "AgentHeartbeatPolicyResolvedPayload"
	sdkAgentHeartbeatWakeAfterPayloadValue      = "AgentHeartbeatWakeAfterPayload"
	sdkAgentHeartbeatWakeBeforePayloadValue     = "AgentHeartbeatWakeBeforePayload"
	sdkAgentLifecyclePatchValue                 = "AgentLifecyclePatch"
	sdkAgentLifecyclePayloadValue               = "AgentLifecyclePayload"
	sdkAgentPreStartPayloadValue                = "AgentPreStartPayload"
	sdkAgentSoulMutationAfterPayloadValue       = "AgentSoulMutationAfterPayload"
	sdkAgentSoulMutationResponseValue           = "AgentSoulMutationResponse"
	sdkAgentSoulPayloadValue                    = "AgentSoulPayload"
	sdkAgentSoulSnapshotResolvedPayloadValue    = "AgentSoulSnapshotResolvedPayload"
	sdkAgentStartPatchValue                     = "AgentStartPatch"
	sdkAuthoredContextObservationPatchValue     = "AuthoredContextObservationPatch"
	sdkAuthoredContextProvenanceValue           = "AuthoredContextProvenance"
	sdkAuthoredMutationProvenanceValue          = "AuthoredMutationProvenance"
	sdkAutomationFirePatchValue                 = "AutomationFirePatch"
	sdkAutomationJobPostFirePayloadValue        = "AutomationJobPostFirePayload"
	sdkAutomationJobPreFirePayloadValue         = "AutomationJobPreFirePayload"
	sdkAutomationObservationPatchValue          = "AutomationObservationPatch"
	sdkAutomationRunCompletedPayloadValue       = "AutomationRunCompletedPayload"
	sdkAutomationRunFailedPayloadValue          = "AutomationRunFailedPayload"
	sdkAutomationSchedulePayloadValue           = "AutomationSchedulePayload"
	sdkAutomationTriggerPostFirePayloadValue    = "AutomationTriggerPostFirePayload"
	sdkAutomationTriggerPreFirePayloadValue     = "AutomationTriggerPreFirePayload"
	sdkAutonomyMatcherValue                     = "AutonomyMatcher"
	sdkAutonomyObservationPatchValue            = "AutonomyObservationPatch"
	sdkBridgeInstanceValue                      = "BridgeInstance"
	sdkContextBlockValue                        = "ContextBlock"
	sdkContextCompactPayloadValue               = "ContextCompactPayload"
	sdkContextCompactionPatchValue              = "ContextCompactionPatch"
	sdkControlPatchValue                        = "ControlPatch"
	sdkCoordinatorContextValue                  = "CoordinatorContext"
	sdkCoordinatorLifecyclePayloadValue         = "CoordinatorLifecyclePayload"
	sdkCoordinatorPreSpawnPayloadValue          = "CoordinatorPreSpawnPayload"
	sdkCoordinatorSpawnPatchValue               = "CoordinatorSpawnPatch"
	sdkEventRecordPatchValue                    = "EventRecordPatch"
	sdkEventRecordPayloadValue                  = "EventRecordPayload"
	sdkHeartbeatMutationResponseValue           = "HeartbeatMutationResponse"
	sdkHeartbeatPolicyPayloadValue              = "HeartbeatPolicyPayload"
	sdkInputPreSubmitPatchValue                 = "InputPreSubmitPatch"
	sdkInputPreSubmitPayloadValue               = "InputPreSubmitPayload"
	sdkLoopContextValue                         = "LoopContext"
	sdkLoopControlPatchValue                    = "LoopControlPatch"
	sdkLoopGatePostPayloadValue                 = "LoopGatePostPayload"
	sdkLoopGatePrePayloadValue                  = "LoopGatePrePayload"
	sdkLoopGatePrePatchValue                    = "LoopGatePrePatch"
	sdkLoopGenerationPostPayloadValue           = "LoopGenerationPostPayload"
	sdkLoopGenerationPrePayloadValue            = "LoopGenerationPrePayload"
	sdkLoopGenerationPrePatchValue              = "LoopGenerationPrePatch"
	sdkLoopLifecyclePayloadValue                = "LoopLifecyclePayload"
	sdkLoopNodeTerminalPayloadValue             = "LoopNodeTerminalPayload"
	sdkLoopObservationPatchValue                = "LoopObservationPatch"
	sdkLoopStartedPayloadValue                  = "LoopStartedPayload"
	sdkLoopTerminalPayloadValue                 = "LoopTerminalPayload"
	sdkMessagePatchValue                        = "MessagePatch"
	sdkMessagePayloadValue                      = "MessagePayload"
	sdkNetworkMessagePersistedPayloadValue      = "NetworkMessagePersistedPayload"
	sdkNetworkObservationPatchValue             = "NetworkObservationPatch"
	sdkNetworkPayloadValue                      = "NetworkPayload"
	sdkNetworkWorkClosedPayloadValue            = "NetworkWorkClosedPayload"
	sdkPayloadBaseValue                         = "PayloadBase"
	sdkPermissionOptionValue                    = "PermissionOption"
	sdkPermissionRequestPatchValue              = "PermissionRequestPatch"
	sdkPermissionRequestPayloadValue            = "PermissionRequestPayload"
	sdkPermissionResolutionPayloadValue         = "PermissionResolutionPayload"
	sdkPermissionSetValue                       = "PermissionSet"
	sdkPermissionToolCallValue                  = "PermissionToolCall"
	sdkPromptPatchValue                         = "PromptPatch"
	sdkPromptPayloadValue                       = "PromptPayload"
	sdkResourceRecordValue                      = "ResourceRecord"
	sdkSandboxObservationPatchValue             = "SandboxObservationPatch"
	sdkSandboxPreparePatchValue                 = "SandboxPreparePatch"
	sdkSandboxPreparePayloadValue               = "SandboxPreparePayload"
	sdkSandboxProfilePayloadValue               = "SandboxProfilePayload"
	sdkSandboxReadyPayloadValue                 = "SandboxReadyPayload"
	sdkSandboxStopPatchValue                    = "SandboxStopPatch"
	sdkSandboxStopPayloadValue                  = "SandboxStopPayload"
	sdkSandboxSyncAfterPayloadValue             = "SandboxSyncAfterPayload"
	sdkSandboxSyncBeforePatchValue              = "SandboxSyncBeforePatch"
	sdkSandboxSyncBeforePayloadValue            = "SandboxSyncBeforePayload"
	sdkSessionContextValue                      = "SessionContext"
	sdkSessionCreatePatchValue                  = "SessionCreatePatch"
	sdkSessionHealthUpdateAfterPayloadValue     = "SessionHealthUpdateAfterPayload"
	sdkSessionLifecyclePayloadValue             = "SessionLifecyclePayload"
	sdkSessionMessagePersistedPayloadValue      = "SessionMessagePersistedPayload"
	sdkSpawnContextValue                        = "SpawnContext"
	sdkSpawnCreatePatchValue                    = "SpawnCreatePatch"
	sdkSpawnLifecyclePayloadValue               = "SpawnLifecyclePayload"
	sdkSpawnPreCreatePayloadValue               = "SpawnPreCreatePayload"
	sdkTaskBlockedPayloadValue                  = "TaskBlockedPayload"
	sdkTaskContextValue                         = "TaskContext"
	sdkTaskNeedsAttentionPayloadValue           = "TaskNeedsAttentionPayload"
	sdkTaskObservationPatchValue                = "TaskObservationPatch"
	sdkTaskRecoveredPayloadValue                = "TaskRecoveredPayload"
	sdkTaskStatusChangedPayloadValue            = "TaskStatusChangedPayload"
	sdkTaskRunClaimCriteriaValue                = "TaskRunClaimCriteria"
	sdkTaskRunContextValue                      = "TaskRunContext"
	sdkTaskRunEnqueuedPayloadValue              = "TaskRunEnqueuedPayload"
	sdkTaskRunLeasePayloadValue                 = "TaskRunLeasePayload"
	sdkTaskRunPostClaimPayloadValue             = "TaskRunPostClaimPayload"
	sdkTaskRunPreClaimPatchValue                = "TaskRunPreClaimPatch"
	sdkTaskRunPreClaimPayloadValue              = "TaskRunPreClaimPayload"
	sdkTaskUnblockedPayloadValue                = "TaskUnblockedPayload"
	sdkToolCallPatchValue                       = "ToolCallPatch"
	sdkToolCallRefValue                         = "ToolCallRef"
	sdkToolLocationValue                        = "ToolLocation"
	sdkToolPostCallPayloadValue                 = "ToolPostCallPayload"
	sdkToolPostErrorPayloadValue                = "ToolPostErrorPayload"
	sdkToolPreCallPayloadValue                  = "ToolPreCallPayload"
	sdkToolResultPatchValue                     = "ToolResultPatch"
	sdkTurnContextValue                         = "TurnContext"
	sdkTurnPatchValue                           = "TurnPatch"
	sdkTurnPayloadValue                         = "TurnPayload"
)

// HookContractSpec binds one hook event to its payload and patch contracts.
type HookContractSpec struct {
	Event   hooks.HookEvent
	Payload NamedType
	Patch   NamedType
}

var sdkRootTypes = []NamedType{
	{Name: "InitializeRequest", Value: subprocess.InitializeRequest{}},
	{Name: "InitializeExtension", Value: subprocess.InitializeExtension{}},
	{Name: "InitializeCapabilities", Value: subprocess.InitializeCapabilities{}},
	{Name: "InitializeMethods", Value: subprocess.InitializeMethods{}},
	{Name: "InitializeRuntime", Value: subprocess.InitializeRuntime{}},
	{Name: "InitializeBridgeRuntime", Value: subprocess.InitializeBridgeRuntime{}},
	{Name: "InitializeBridgeBoundSecret", Value: subprocess.InitializeBridgeBoundSecret{}},
	{Name: "InitializeResponse", Value: subprocess.InitializeResponse{}},
	{Name: "InitializeExtensionInfo", Value: subprocess.InitializeExtensionInfo{}},
	{Name: "AcceptedCapabilities", Value: subprocess.AcceptedCapabilities{}},
	{Name: "InitializeSupports", Value: subprocess.InitializeSupports{}},
	{Name: "ResourceKind", Value: resources.ResourceKind("")},
	{Name: "ResourceScopeKind", Value: resources.ResourceScopeKind("")},
	{Name: "ResourceScope", Value: resources.ResourceScope{}},
	{Name: "ResourceSourceKind", Value: resources.ResourceSourceKind("")},
	{Name: "ResourceSource", Value: resources.ResourceSource{}},
	{Name: "ResourceOwnerKind", Value: resources.ResourceOwnerKind("")},
	{Name: "ResourceOwner", Value: resources.ResourceOwner{}},
	{Name: sdkResourceRecordValue, Value: ResourceRecord{}},
	{Name: "ResourceGetParams", Value: ResourceGetParams{}},
	{Name: "ResourcesListParams", Value: ResourcesListParams{}},
	{Name: "ResourceSnapshotRecord", Value: ResourceSnapshotRecord{}},
	{Name: "ResourcesSnapshotParams", Value: ResourcesSnapshotParams{}},
	{Name: "AuthoredValidationStatus", Value: apicontract.AuthoredValidationStatus("")},
	{Name: "AuthoredDiagnosticSeverity", Value: apicontract.AuthoredDiagnosticSeverity("")},
	{Name: "AuthoredContextDiagnosticPayload", Value: apicontract.AuthoredContextDiagnosticPayload{}},
	{Name: "AgentSoulSectionPayload", Value: apicontract.AgentSoulSectionPayload{}},
	{Name: sdkAgentSoulPayloadValue, Value: apicontract.AgentSoulPayload{}},
	{Name: "AgentSoulValidateRequest", Value: apicontract.AgentSoulValidateRequest{}},
	{Name: "AgentSoulPutRequest", Value: apicontract.AgentSoulPutRequest{}},
	{Name: "AgentSoulDeleteRequest", Value: apicontract.AgentSoulDeleteRequest{}},
	{Name: "AgentSoulRollbackRequest", Value: apicontract.AgentSoulRollbackRequest{}},
	{Name: "AgentSoulHistoryRequest", Value: apicontract.AgentSoulHistoryRequest{}},
	{Name: "AgentSoulHistoryResponse", Value: apicontract.AgentSoulHistoryResponse{}},
	{Name: sdkAgentSoulMutationResponseValue, Value: apicontract.AgentSoulMutationResponse{}},
	{Name: "SessionSoulRefreshRequest", Value: apicontract.SessionSoulRefreshRequest{}},
	{Name: sdkHeartbeatPolicyPayloadValue, Value: apicontract.HeartbeatPolicyPayload{}},
	{Name: "HeartbeatValidateRequest", Value: apicontract.HeartbeatValidateRequest{}},
	{Name: "HeartbeatPutRequest", Value: apicontract.HeartbeatPutRequest{}},
	{Name: "HeartbeatDeleteRequest", Value: apicontract.HeartbeatDeleteRequest{}},
	{Name: "HeartbeatRollbackRequest", Value: apicontract.HeartbeatRollbackRequest{}},
	{Name: "HeartbeatHistoryRequest", Value: apicontract.HeartbeatHistoryRequest{}},
	{Name: "HeartbeatHistoryResponse", Value: apicontract.HeartbeatHistoryResponse{}},
	{Name: sdkHeartbeatMutationResponseValue, Value: apicontract.HeartbeatMutationResponse{}},
	{Name: "HeartbeatStatusRequest", Value: apicontract.HeartbeatStatusRequest{}},
	{Name: "HeartbeatStatusResponse", Value: apicontract.HeartbeatStatusResponse{}},
	{Name: "HeartbeatWakeRequest", Value: apicontract.HeartbeatWakeRequest{}},
	{Name: "HeartbeatWakeResponse", Value: apicontract.HeartbeatWakeResponse{}},
	{Name: "SessionHealthPayload", Value: apicontract.SessionHealthPayload{}},
	{Name: "SessionHealthResponse", Value: apicontract.SessionHealthResponse{}},
	{Name: "SessionStatusResponse", Value: apicontract.SessionStatusResponse{}},
	{Name: "SessionInspectResponse", Value: apicontract.SessionInspectResponse{}},
	{Name: "HeartbeatWakeStatePayload", Value: apicontract.HeartbeatWakeStatePayload{}},
	{Name: "HeartbeatWakeEventPayload", Value: apicontract.HeartbeatWakeEventPayload{}},
	{Name: "HeartbeatWakeDecisionPayload", Value: apicontract.HeartbeatWakeDecisionPayload{}},
	{Name: "ShutdownRequest", Value: subprocess.ShutdownRequest{}},
	{Name: "ShutdownResponse", Value: subprocess.ShutdownResponse{}},
	{Name: sdkBridgeInstanceValue, Value: bridgepkg.BridgeInstance{}},
	{Name: "BridgeStatus", Value: bridgepkg.BridgeStatus("")},
	{Name: "BridgeScope", Value: bridgepkg.Scope("")},
	{Name: "RoutingPolicy", Value: bridgepkg.RoutingPolicy{}},
	{Name: "RoutingKey", Value: bridgepkg.RoutingKey{}},
	{Name: "InboundEventFamily", Value: bridgepkg.InboundEventFamily("")},
	{Name: inboundMessageEnvelopeTypeName, Value: bridgepkg.InboundMessageEnvelope{}},
	{Name: "InboundCommand", Value: bridgepkg.InboundCommand{}},
	{Name: "InboundAction", Value: bridgepkg.InboundAction{}},
	{Name: "InboundReaction", Value: bridgepkg.InboundReaction{}},
	{Name: "InboundEditOperation", Value: bridgepkg.InboundEditOperation("")},
	{Name: "InboundEdit", Value: bridgepkg.InboundEdit{}},
	{Name: "DeliveryEvent", Value: bridgepkg.DeliveryEvent{}},
	{Name: "DeliveryRequest", Value: bridgepkg.DeliveryRequest{}},
	{Name: "DeliveryAck", Value: bridgepkg.DeliveryAck{}},
	{Name: "DeliverySnapshot", Value: bridgepkg.DeliverySnapshot{}},
	{Name: "DeliveryTarget", Value: bridgepkg.DeliveryTarget{}},
	{Name: "BridgeTargetSnapshotRequest", Value: bridgepkg.BridgeTargetSnapshotRequest{}},
	{Name: "BridgeTargetSnapshotResponse", Value: bridgepkg.BridgeTargetSnapshotResponse{}},
	{Name: "BridgeTargetSnapshot", Value: bridgepkg.BridgeTargetSnapshot{}},
	{Name: "BridgeTargetType", Value: bridgepkg.BridgeTargetType("")},
	{Name: "DeliveryMode", Value: bridgepkg.DeliveryMode("")},
	{Name: "DeliveryOperation", Value: bridgepkg.DeliveryOperation("")},
	{Name: "DeliveryMessageReference", Value: bridgepkg.DeliveryMessageReference{}},
	{Name: "DeliveryErrorDetail", Value: bridgepkg.DeliveryErrorDetail{}},
	{Name: "DeliveryResumeState", Value: bridgepkg.DeliveryResumeState{}},
	{Name: "MessageSender", Value: bridgepkg.MessageSender{}},
	{Name: "MessageContent", Value: bridgepkg.MessageContent{}},
	{Name: "MessageAttachment", Value: bridgepkg.MessageAttachment{}},
	{Name: "Tool", Value: tools.Tool{}},
	{Name: "ToolID", Value: tools.ToolID("")},
	{Name: "RiskClass", Value: tools.RiskClass("")},
	{Name: "ToolContent", Value: tools.ToolContent{}},
	{Name: "ArtifactRef", Value: tools.ArtifactRef{}},
	{Name: "Redaction", Value: tools.Redaction{}},
	{Name: "ToolResult", Value: tools.ToolResult{}},
	{Name: "ExtensionToolRuntimeDescriptor", Value: tools.ExtensionToolRuntimeDescriptor{}},
	{Name: "ExtensionProvideToolsResponse", Value: tools.ExtensionProvideToolsResponse{}},
	{Name: "ExtensionToolCallRequest", Value: tools.ExtensionToolCallRequest{}},
	{Name: "ExtensionToolCallResponse", Value: tools.ExtensionToolCallResponse{}},
	{Name: "ModelSourceListParams", Value: ModelSourceListParams{}},
	{Name: "ModelSourceListResponse", Value: ModelSourceListResponse{}},
	{Name: "ModelSourceRow", Value: ModelSourceRow{}},
	{Name: "MemoryScope", Value: memcontract.Scope("")},
	{Name: "HookEventFamily", Value: hooks.HookEventFamily("")},
	{Name: "HookRunOutcome", Value: hooks.HookRunOutcome("")},
	{Name: "HookSkillSource", Value: hooks.HookSkillSource("")},
	{Name: sdkPayloadBaseValue, Value: hooks.PayloadBase{}},
	{Name: sdkSessionContextValue, Value: hooks.SessionContext{}},
	{Name: sdkTurnContextValue, Value: hooks.TurnContext{}},
	{Name: sdkContextBlockValue, Value: hooks.ContextBlock{}},
	{Name: sdkToolCallRefValue, Value: hooks.ToolCallRef{}},
	{Name: sdkToolLocationValue, Value: hooks.ToolLocation{}},
	{Name: sdkPermissionOptionValue, Value: hooks.PermissionOption{}},
	{Name: sdkPermissionToolCallValue, Value: hooks.PermissionToolCall{}},
	{Name: sdkControlPatchValue, Value: hooks.ControlPatch{}},
	{Name: sdkSessionLifecyclePayloadValue, Value: hooks.SessionLifecyclePayload{}},
	{Name: sdkSessionCreatePatchValue, Value: hooks.SessionCreatePatch{}},
	{Name: sdkSandboxProfilePayloadValue, Value: hooks.SandboxProfilePayload{}},
	{Name: sdkSandboxPreparePayloadValue, Value: hooks.SandboxPreparePayload{}},
	{Name: sdkSandboxReadyPayloadValue, Value: hooks.SandboxReadyPayload{}},
	{Name: sdkSandboxSyncBeforePayloadValue, Value: hooks.SandboxSyncBeforePayload{}},
	{Name: sdkSandboxSyncAfterPayloadValue, Value: hooks.SandboxSyncAfterPayload{}},
	{Name: sdkSandboxStopPayloadValue, Value: hooks.SandboxStopPayload{}},
	{Name: sdkSandboxPreparePatchValue, Value: hooks.SandboxPreparePatch{}},
	{Name: sdkSandboxSyncBeforePatchValue, Value: hooks.SandboxSyncBeforePatch{}},
	{Name: sdkSandboxObservationPatchValue, Value: hooks.SandboxObservationPatch{}},
	{Name: sdkSandboxStopPatchValue, Value: hooks.SandboxStopPatch{}},
	{Name: sdkInputPreSubmitPayloadValue, Value: hooks.InputPreSubmitPayload{}},
	{Name: sdkInputPreSubmitPatchValue, Value: hooks.InputPreSubmitPatch{}},
	{Name: sdkPromptPayloadValue, Value: hooks.PromptPayload{}},
	{Name: sdkPromptPatchValue, Value: hooks.PromptPatch{}},
	{Name: sdkEventRecordPayloadValue, Value: hooks.EventRecordPayload{}},
	{Name: sdkEventRecordPatchValue, Value: hooks.EventRecordPatch{}},
	{Name: sdkAutomationSchedulePayloadValue, Value: hooks.AutomationSchedulePayload{}},
	{Name: sdkAutomationJobPreFirePayloadValue, Value: hooks.AutomationJobPreFirePayload{}},
	{Name: sdkAutomationJobPostFirePayloadValue, Value: hooks.AutomationJobPostFirePayload{}},
	{Name: sdkAutomationTriggerPreFirePayloadValue, Value: hooks.AutomationTriggerPreFirePayload{}},
	{Name: sdkAutomationTriggerPostFirePayloadValue, Value: hooks.AutomationTriggerPostFirePayload{}},
	{Name: sdkAutomationRunCompletedPayloadValue, Value: hooks.AutomationRunCompletedPayload{}},
	{Name: sdkAutomationRunFailedPayloadValue, Value: hooks.AutomationRunFailedPayload{}},
	{Name: sdkAutomationFirePatchValue, Value: hooks.AutomationFirePatch{}},
	{Name: sdkAutomationObservationPatchValue, Value: hooks.AutomationObservationPatch{}},
	{Name: sdkAgentPreStartPayloadValue, Value: hooks.AgentPreStartPayload{}},
	{Name: sdkAgentLifecyclePayloadValue, Value: hooks.AgentLifecyclePayload{}},
	{Name: sdkAgentStartPatchValue, Value: hooks.AgentStartPatch{}},
	{Name: sdkAgentLifecyclePatchValue, Value: hooks.AgentLifecyclePatch{}},
	{Name: sdkAuthoredContextProvenanceValue, Value: hooks.AuthoredContextProvenance{}},
	{Name: sdkAuthoredMutationProvenanceValue, Value: hooks.AuthoredMutationProvenance{}},
	{Name: sdkAgentSoulSnapshotResolvedPayloadValue, Value: hooks.AgentSoulSnapshotResolvedPayload{}},
	{Name: sdkAgentSoulMutationAfterPayloadValue, Value: hooks.AgentSoulMutationAfterPayload{}},
	{Name: sdkAgentHeartbeatPolicyResolvedPayloadValue, Value: hooks.AgentHeartbeatPolicyResolvedPayload{}},
	{Name: sdkAgentHeartbeatWakeBeforePayloadValue, Value: hooks.AgentHeartbeatWakeBeforePayload{}},
	{Name: sdkAgentHeartbeatWakeAfterPayloadValue, Value: hooks.AgentHeartbeatWakeAfterPayload{}},
	{Name: sdkSessionHealthUpdateAfterPayloadValue, Value: hooks.SessionHealthUpdateAfterPayload{}},
	{Name: sdkAuthoredContextObservationPatchValue, Value: hooks.AuthoredContextObservationPatch{}},
	{Name: sdkNetworkPayloadValue, Value: hooks.NetworkPayload{}},
	{Name: sdkNetworkObservationPatchValue, Value: hooks.NetworkObservationPatch{}},
	{Name: sdkTurnPayloadValue, Value: hooks.TurnPayload{}},
	{Name: sdkTurnPatchValue, Value: hooks.TurnPatch{}},
	{Name: sdkMessagePayloadValue, Value: hooks.MessagePayload{}},
	{Name: sdkSessionMessagePersistedPayloadValue, Value: hooks.SessionMessagePersistedPayload{}},
	{Name: sdkMessagePatchValue, Value: hooks.MessagePatch{}},
	{Name: sdkToolPreCallPayloadValue, Value: hooks.ToolPreCallPayload{}},
	{Name: sdkToolPostCallPayloadValue, Value: hooks.ToolPostCallPayload{}},
	{Name: sdkToolPostErrorPayloadValue, Value: hooks.ToolPostErrorPayload{}},
	{Name: sdkToolCallPatchValue, Value: hooks.ToolCallPatch{}},
	{Name: sdkToolResultPatchValue, Value: hooks.ToolResultPatch{}},
	{Name: sdkPermissionRequestPayloadValue, Value: hooks.PermissionRequestPayload{}},
	{Name: sdkPermissionResolutionPayloadValue, Value: hooks.PermissionResolutionPayload{}},
	{Name: sdkPermissionRequestPatchValue, Value: hooks.PermissionRequestPatch{}},
	{Name: sdkContextCompactPayloadValue, Value: hooks.ContextCompactPayload{}},
	{Name: sdkContextCompactionPatchValue, Value: hooks.ContextCompactionPatch{}},
	{Name: sdkAutonomyObservationPatchValue, Value: hooks.AutonomyObservationPatch{}},
	{Name: sdkCoordinatorContextValue, Value: hooks.CoordinatorContext{}},
	{Name: sdkCoordinatorPreSpawnPayloadValue, Value: hooks.CoordinatorPreSpawnPayload{}},
	{Name: sdkCoordinatorLifecyclePayloadValue, Value: hooks.CoordinatorLifecyclePayload{}},
	{Name: sdkCoordinatorSpawnPatchValue, Value: hooks.CoordinatorSpawnPatch{}},
	{Name: sdkTaskRunClaimCriteriaValue, Value: hooks.TaskRunClaimCriteria{}},
	{Name: sdkTaskRunContextValue, Value: hooks.TaskRunContext{}},
	{Name: sdkTaskRunEnqueuedPayloadValue, Value: hooks.TaskRunEnqueuedPayload{}},
	{Name: sdkTaskRunPreClaimPayloadValue, Value: hooks.TaskRunPreClaimPayload{}},
	{Name: sdkTaskRunPostClaimPayloadValue, Value: hooks.TaskRunPostClaimPayload{}},
	{Name: sdkTaskRunLeasePayloadValue, Value: hooks.TaskRunLeasePayload{}},
	{Name: sdkTaskRunPreClaimPatchValue, Value: hooks.TaskRunPreClaimPatch{}},
	{Name: sdkLoopContextValue, Value: hooks.LoopContext{}},
	{Name: sdkLoopLifecyclePayloadValue, Value: hooks.LoopLifecyclePayload{}},
	{Name: sdkLoopStartedPayloadValue, Value: hooks.LoopStartedPayload{}},
	{Name: sdkLoopTerminalPayloadValue, Value: hooks.LoopTerminalPayload{}},
	{Name: sdkLoopGenerationPrePayloadValue, Value: hooks.LoopGenerationPrePayload{}},
	{Name: sdkLoopGenerationPostPayloadValue, Value: hooks.LoopGenerationPostPayload{}},
	{Name: sdkLoopGatePrePayloadValue, Value: hooks.LoopGatePrePayload{}},
	{Name: sdkLoopGatePostPayloadValue, Value: hooks.LoopGatePostPayload{}},
	{Name: sdkLoopNodeTerminalPayloadValue, Value: hooks.LoopNodeTerminalPayload{}},
	{Name: sdkLoopControlPatchValue, Value: hooks.LoopControlPatch{}},
	{Name: sdkLoopGenerationPrePatchValue, Value: hooks.LoopGenerationPrePatch{}},
	{Name: sdkLoopGatePrePatchValue, Value: hooks.LoopGatePrePatch{}},
	{Name: sdkLoopObservationPatchValue, Value: hooks.LoopObservationPatch{}},
	{Name: sdkPermissionSetValue, Value: hooks.PermissionSet{}},
	{Name: sdkSpawnContextValue, Value: hooks.SpawnContext{}},
	{Name: sdkSpawnPreCreatePayloadValue, Value: hooks.SpawnPreCreatePayload{}},
	{Name: sdkSpawnLifecyclePayloadValue, Value: hooks.SpawnLifecyclePayload{}},
	{Name: sdkSpawnCreatePatchValue, Value: hooks.SpawnCreatePatch{}},
	{Name: sdkTaskContextValue, Value: hooks.TaskContext{}},
	{Name: sdkTaskBlockedPayloadValue, Value: hooks.TaskBlockedPayload{}},
	{Name: sdkTaskUnblockedPayloadValue, Value: hooks.TaskUnblockedPayload{}},
	{Name: sdkTaskNeedsAttentionPayloadValue, Value: hooks.TaskNeedsAttentionPayload{}},
	{Name: sdkTaskRecoveredPayloadValue, Value: hooks.TaskRecoveredPayload{}},
	{Name: sdkTaskStatusChangedPayloadValue, Value: hooks.TaskStatusChangedPayload{}},
	{Name: sdkTaskObservationPatchValue, Value: hooks.TaskObservationPatch{}},
	{Name: sdkAutonomyMatcherValue, Value: hooks.AutonomyMatcher{}},
	{Name: "NetworkMatcher", Value: hooks.NetworkMatcher{}},
	{Name: "CompactionMatcher", Value: hooks.CompactionMatcher{}},
	{Name: "HookMatcher", Value: hooks.HookMatcher{}},
	{Name: "HookDecl", Value: hooks.HookDecl{}},
}
