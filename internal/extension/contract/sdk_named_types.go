package contract

import (
	"fmt"

	"github.com/compozy/agh/internal/hooks"
)

var namedHookTypes = mergeNamedHookTypes(map[string]NamedType{
	sdkPayloadBaseValue:             {Name: sdkPayloadBaseValue, Value: hooks.PayloadBase{}},
	sdkSessionContextValue:          {Name: sdkSessionContextValue, Value: hooks.SessionContext{}},
	sdkTurnContextValue:             {Name: sdkTurnContextValue, Value: hooks.TurnContext{}},
	sdkContextBlockValue:            {Name: sdkContextBlockValue, Value: hooks.ContextBlock{}},
	sdkToolCallRefValue:             {Name: sdkToolCallRefValue, Value: hooks.ToolCallRef{}},
	sdkToolLocationValue:            {Name: sdkToolLocationValue, Value: hooks.ToolLocation{}},
	sdkPermissionOptionValue:        {Name: sdkPermissionOptionValue, Value: hooks.PermissionOption{}},
	sdkPermissionToolCallValue:      {Name: sdkPermissionToolCallValue, Value: hooks.PermissionToolCall{}},
	sdkControlPatchValue:            {Name: sdkControlPatchValue, Value: hooks.ControlPatch{}},
	sdkSessionLifecyclePayloadValue: {Name: sdkSessionLifecyclePayloadValue, Value: hooks.SessionLifecyclePayload{}},
	"SessionPreCreatePayload":       {Name: "SessionPreCreatePayload", Value: hooks.SessionPreCreatePayload{}},
	"SessionPostCreatePayload":      {Name: "SessionPostCreatePayload", Value: hooks.SessionPostCreatePayload{}},
	"SessionPreResumePayload":       {Name: "SessionPreResumePayload", Value: hooks.SessionPreResumePayload{}},
	"SessionPostResumePayload":      {Name: "SessionPostResumePayload", Value: hooks.SessionPostResumePayload{}},
	"SessionPreStopPayload":         {Name: "SessionPreStopPayload", Value: hooks.SessionPreStopPayload{}},
	"SessionPostStopPayload":        {Name: "SessionPostStopPayload", Value: hooks.SessionPostStopPayload{}},
	sdkSessionCreatePatchValue:      {Name: sdkSessionCreatePatchValue, Value: hooks.SessionCreatePatch{}},
	"SessionPostCreatePatch":        {Name: "SessionPostCreatePatch", Value: hooks.SessionPostCreatePatch{}},
	"SessionPreResumePatch":         {Name: "SessionPreResumePatch", Value: hooks.SessionPreResumePatch{}},
	"SessionPostResumePatch":        {Name: "SessionPostResumePatch", Value: hooks.SessionPostResumePatch{}},
	"SessionPreStopPatch":           {Name: "SessionPreStopPatch", Value: hooks.SessionPreStopPatch{}},
	"SessionPostStopPatch":          {Name: "SessionPostStopPatch", Value: hooks.SessionPostStopPatch{}},
	sdkSandboxProfilePayloadValue:   {Name: sdkSandboxProfilePayloadValue, Value: hooks.SandboxProfilePayload{}},
	sdkSandboxPreparePayloadValue:   {Name: sdkSandboxPreparePayloadValue, Value: hooks.SandboxPreparePayload{}},
	sdkSandboxReadyPayloadValue:     {Name: sdkSandboxReadyPayloadValue, Value: hooks.SandboxReadyPayload{}},
	sdkSandboxSyncBeforePayloadValue: {
		Name:  sdkSandboxSyncBeforePayloadValue,
		Value: hooks.SandboxSyncBeforePayload{},
	},
	sdkSandboxSyncAfterPayloadValue: {
		Name:  sdkSandboxSyncAfterPayloadValue,
		Value: hooks.SandboxSyncAfterPayload{},
	},
	sdkSandboxStopPayloadValue:  {Name: sdkSandboxStopPayloadValue, Value: hooks.SandboxStopPayload{}},
	sdkSandboxPreparePatchValue: {Name: sdkSandboxPreparePatchValue, Value: hooks.SandboxPreparePatch{}},
	sdkSandboxSyncBeforePatchValue: {
		Name:  sdkSandboxSyncBeforePatchValue,
		Value: hooks.SandboxSyncBeforePatch{},
	},
	sdkSandboxObservationPatchValue: {
		Name:  sdkSandboxObservationPatchValue,
		Value: hooks.SandboxObservationPatch{},
	},
	"SandboxReadyPatch":           {Name: "SandboxReadyPatch", Value: hooks.SandboxReadyPatch{}},
	"SandboxSyncAfterPatch":       {Name: "SandboxSyncAfterPatch", Value: hooks.SandboxSyncAfterPatch{}},
	sdkSandboxStopPatchValue:      {Name: sdkSandboxStopPatchValue, Value: hooks.SandboxStopPatch{}},
	sdkInputPreSubmitPayloadValue: {Name: sdkInputPreSubmitPayloadValue, Value: hooks.InputPreSubmitPayload{}},
	sdkInputPreSubmitPatchValue:   {Name: sdkInputPreSubmitPatchValue, Value: hooks.InputPreSubmitPatch{}},
	sdkPromptPayloadValue:         {Name: sdkPromptPayloadValue, Value: hooks.PromptPayload{}},
	sdkPromptPatchValue:           {Name: sdkPromptPatchValue, Value: hooks.PromptPatch{}},
	sdkEventRecordPayloadValue:    {Name: sdkEventRecordPayloadValue, Value: hooks.EventRecordPayload{}},
	"EventPreRecordPayload":       {Name: "EventPreRecordPayload", Value: hooks.EventPreRecordPayload{}},
	"EventPostRecordPayload":      {Name: "EventPostRecordPayload", Value: hooks.EventPostRecordPayload{}},
	sdkEventRecordPatchValue:      {Name: sdkEventRecordPatchValue, Value: hooks.EventRecordPatch{}},
	"EventPreRecordPatch":         {Name: "EventPreRecordPatch", Value: hooks.EventPreRecordPatch{}},
	"EventPostRecordPatch":        {Name: "EventPostRecordPatch", Value: hooks.EventPostRecordPatch{}},
	sdkAutomationSchedulePayloadValue: {
		Name:  sdkAutomationSchedulePayloadValue,
		Value: hooks.AutomationSchedulePayload{},
	},
	sdkAutomationJobPreFirePayloadValue: {
		Name:  sdkAutomationJobPreFirePayloadValue,
		Value: hooks.AutomationJobPreFirePayload{},
	},
	sdkAutomationJobPostFirePayloadValue: {
		Name:  sdkAutomationJobPostFirePayloadValue,
		Value: hooks.AutomationJobPostFirePayload{},
	},
	sdkAutomationTriggerPreFirePayloadValue: {
		Name:  sdkAutomationTriggerPreFirePayloadValue,
		Value: hooks.AutomationTriggerPreFirePayload{},
	},
	sdkAutomationTriggerPostFirePayloadValue: {
		Name:  sdkAutomationTriggerPostFirePayloadValue,
		Value: hooks.AutomationTriggerPostFirePayload{},
	},
	sdkAutomationRunCompletedPayloadValue: {
		Name:  sdkAutomationRunCompletedPayloadValue,
		Value: hooks.AutomationRunCompletedPayload{},
	},
	sdkAutomationRunFailedPayloadValue: {
		Name:  sdkAutomationRunFailedPayloadValue,
		Value: hooks.AutomationRunFailedPayload{},
	},
	sdkAutomationFirePatchValue: {
		Name:  sdkAutomationFirePatchValue,
		Value: hooks.AutomationFirePatch{},
	},
	sdkAutomationObservationPatchValue: {
		Name:  sdkAutomationObservationPatchValue,
		Value: hooks.AutomationObservationPatch{},
	},
	sdkAgentPreStartPayloadValue: {
		Name:  sdkAgentPreStartPayloadValue,
		Value: hooks.AgentPreStartPayload{},
	},
	sdkAgentLifecyclePayloadValue: {
		Name:  sdkAgentLifecyclePayloadValue,
		Value: hooks.AgentLifecyclePayload{},
	},
	"AgentSpawnedPayload": {
		Name:  "AgentSpawnedPayload",
		Value: hooks.AgentSpawnedPayload{},
	},
	"AgentCrashedPayload": {
		Name:  "AgentCrashedPayload",
		Value: hooks.AgentCrashedPayload{},
	},
	"AgentStoppedPayload": {
		Name:  "AgentStoppedPayload",
		Value: hooks.AgentStoppedPayload{},
	},
	sdkAgentStartPatchValue:     {Name: sdkAgentStartPatchValue, Value: hooks.AgentStartPatch{}},
	sdkAgentLifecyclePatchValue: {Name: sdkAgentLifecyclePatchValue, Value: hooks.AgentLifecyclePatch{}},
	"AgentSpawnedPatch":         {Name: "AgentSpawnedPatch", Value: hooks.AgentSpawnedPatch{}},
	"AgentCrashedPatch":         {Name: "AgentCrashedPatch", Value: hooks.AgentCrashedPatch{}},
	"AgentStoppedPatch":         {Name: "AgentStoppedPatch", Value: hooks.AgentStoppedPatch{}},
	sdkAuthoredContextProvenanceValue: {
		Name:  sdkAuthoredContextProvenanceValue,
		Value: hooks.AuthoredContextProvenance{},
	},
	sdkAuthoredMutationProvenanceValue: {
		Name:  sdkAuthoredMutationProvenanceValue,
		Value: hooks.AuthoredMutationProvenance{},
	},
	sdkAgentSoulSnapshotResolvedPayloadValue: {
		Name:  sdkAgentSoulSnapshotResolvedPayloadValue,
		Value: hooks.AgentSoulSnapshotResolvedPayload{},
	},
	sdkAgentSoulMutationAfterPayloadValue: {
		Name:  sdkAgentSoulMutationAfterPayloadValue,
		Value: hooks.AgentSoulMutationAfterPayload{},
	},
	sdkAgentHeartbeatPolicyResolvedPayloadValue: {
		Name:  sdkAgentHeartbeatPolicyResolvedPayloadValue,
		Value: hooks.AgentHeartbeatPolicyResolvedPayload{},
	},
	sdkAgentHeartbeatWakeBeforePayloadValue: {
		Name:  sdkAgentHeartbeatWakeBeforePayloadValue,
		Value: hooks.AgentHeartbeatWakeBeforePayload{},
	},
	sdkAgentHeartbeatWakeAfterPayloadValue: {
		Name:  sdkAgentHeartbeatWakeAfterPayloadValue,
		Value: hooks.AgentHeartbeatWakeAfterPayload{},
	},
	sdkSessionHealthUpdateAfterPayloadValue: {
		Name:  sdkSessionHealthUpdateAfterPayloadValue,
		Value: hooks.SessionHealthUpdateAfterPayload{},
	},
	sdkAuthoredContextObservationPatchValue: {
		Name:  sdkAuthoredContextObservationPatchValue,
		Value: hooks.AuthoredContextObservationPatch{},
	},
	sdkNetworkPayloadValue:       {Name: sdkNetworkPayloadValue, Value: hooks.NetworkPayload{}},
	"NetworkPeerJoinedPayload":   {Name: "NetworkPeerJoinedPayload", Value: hooks.NetworkPeerJoinedPayload{}},
	"NetworkPeerLeftPayload":     {Name: "NetworkPeerLeftPayload", Value: hooks.NetworkPeerLeftPayload{}},
	"NetworkThreadOpenedPayload": {Name: "NetworkThreadOpenedPayload", Value: hooks.NetworkThreadOpenedPayload{}},
	"NetworkDirectRoomOpenedPayload": {
		Name:  "NetworkDirectRoomOpenedPayload",
		Value: hooks.NetworkDirectRoomOpenedPayload{},
	},
	sdkNetworkMessagePersistedPayloadValue: {
		Name:  sdkNetworkMessagePersistedPayloadValue,
		Value: hooks.NetworkMessagePersistedPayload{},
	},
	"NetworkWorkOpenedPayload": {Name: "NetworkWorkOpenedPayload", Value: hooks.NetworkWorkOpenedPayload{}},
	"NetworkWorkTransitionedPayload": {
		Name:  "NetworkWorkTransitionedPayload",
		Value: hooks.NetworkWorkTransitionedPayload{},
	},
	sdkNetworkWorkClosedPayloadValue: {Name: sdkNetworkWorkClosedPayloadValue, Value: hooks.NetworkWorkClosedPayload{}},
	sdkNetworkObservationPatchValue:  {Name: sdkNetworkObservationPatchValue, Value: hooks.NetworkObservationPatch{}},
	sdkTurnPayloadValue:              {Name: sdkTurnPayloadValue, Value: hooks.TurnPayload{}},
	"TurnStartPayload":               {Name: "TurnStartPayload", Value: hooks.TurnStartPayload{}},
	"TurnEndPayload":                 {Name: "TurnEndPayload", Value: hooks.TurnEndPayload{}},
	sdkTurnPatchValue:                {Name: sdkTurnPatchValue, Value: hooks.TurnPatch{}},
	"TurnStartPatch":                 {Name: "TurnStartPatch", Value: hooks.TurnStartPatch{}},
	"TurnEndPatch":                   {Name: "TurnEndPatch", Value: hooks.TurnEndPatch{}},
	sdkMessagePayloadValue:           {Name: sdkMessagePayloadValue, Value: hooks.MessagePayload{}},
	"MessageStartPayload":            {Name: "MessageStartPayload", Value: hooks.MessageStartPayload{}},
	"MessageDeltaPayload":            {Name: "MessageDeltaPayload", Value: hooks.MessageDeltaPayload{}},
	"MessageEndPayload":              {Name: "MessageEndPayload", Value: hooks.MessageEndPayload{}},
	sdkSessionMessagePersistedPayloadValue: {
		Name:  sdkSessionMessagePersistedPayloadValue,
		Value: hooks.SessionMessagePersistedPayload{},
	},
	sdkMessagePatchValue:         {Name: sdkMessagePatchValue, Value: hooks.MessagePatch{}},
	"MessageStartPatch":          {Name: "MessageStartPatch", Value: hooks.MessageStartPatch{}},
	"MessageDeltaPatch":          {Name: "MessageDeltaPatch", Value: hooks.MessageDeltaPatch{}},
	"MessageEndPatch":            {Name: "MessageEndPatch", Value: hooks.MessageEndPatch{}},
	sdkToolPreCallPayloadValue:   {Name: sdkToolPreCallPayloadValue, Value: hooks.ToolPreCallPayload{}},
	sdkToolPostCallPayloadValue:  {Name: sdkToolPostCallPayloadValue, Value: hooks.ToolPostCallPayload{}},
	sdkToolPostErrorPayloadValue: {Name: sdkToolPostErrorPayloadValue, Value: hooks.ToolPostErrorPayload{}},
	sdkToolCallPatchValue:        {Name: sdkToolCallPatchValue, Value: hooks.ToolCallPatch{}},
	sdkToolResultPatchValue:      {Name: sdkToolResultPatchValue, Value: hooks.ToolResultPatch{}},
	"ToolPostErrorPatch":         {Name: "ToolPostErrorPatch", Value: hooks.ToolPostErrorPatch{}},
	sdkPermissionRequestPayloadValue: {
		Name:  sdkPermissionRequestPayloadValue,
		Value: hooks.PermissionRequestPayload{},
	},
	sdkPermissionResolutionPayloadValue: {
		Name:  sdkPermissionResolutionPayloadValue,
		Value: hooks.PermissionResolutionPayload{},
	},
	"PermissionResolvedPayload":    {Name: "PermissionResolvedPayload", Value: hooks.PermissionResolvedPayload{}},
	"PermissionDeniedPayload":      {Name: "PermissionDeniedPayload", Value: hooks.PermissionDeniedPayload{}},
	sdkPermissionRequestPatchValue: {Name: sdkPermissionRequestPatchValue, Value: hooks.PermissionRequestPatch{}},
	"PermissionResolvedPatch":      {Name: "PermissionResolvedPatch", Value: hooks.PermissionResolvedPatch{}},
	"PermissionDeniedPatch":        {Name: "PermissionDeniedPatch", Value: hooks.PermissionDeniedPatch{}},
	sdkContextCompactPayloadValue:  {Name: sdkContextCompactPayloadValue, Value: hooks.ContextCompactPayload{}},
	"ContextPreCompactPayload":     {Name: "ContextPreCompactPayload", Value: hooks.ContextPreCompactPayload{}},
	"ContextPostCompactPayload":    {Name: "ContextPostCompactPayload", Value: hooks.ContextPostCompactPayload{}},
	sdkContextCompactionPatchValue: {Name: sdkContextCompactionPatchValue, Value: hooks.ContextCompactionPatch{}},
	"ContextPreCompactPatch":       {Name: "ContextPreCompactPatch", Value: hooks.ContextPreCompactPatch{}},
	"ContextPostCompactPatch":      {Name: "ContextPostCompactPatch", Value: hooks.ContextPostCompactPatch{}},
	sdkAutonomyObservationPatchValue: {
		Name:  sdkAutonomyObservationPatchValue,
		Value: hooks.AutonomyObservationPatch{},
	},
	sdkCoordinatorContextValue: {Name: sdkCoordinatorContextValue, Value: hooks.CoordinatorContext{}},
	sdkCoordinatorPreSpawnPayloadValue: {
		Name:  sdkCoordinatorPreSpawnPayloadValue,
		Value: hooks.CoordinatorPreSpawnPayload{},
	},
	sdkCoordinatorLifecyclePayloadValue: {
		Name:  sdkCoordinatorLifecyclePayloadValue,
		Value: hooks.CoordinatorLifecyclePayload{},
	},
	"CoordinatorSpawnedPayload": {
		Name:  "CoordinatorSpawnedPayload",
		Value: hooks.CoordinatorSpawnedPayload{},
	},
	"CoordinatorDecisionPayload": {
		Name:  "CoordinatorDecisionPayload",
		Value: hooks.CoordinatorDecisionPayload{},
	},
	"CoordinatorStoppedPayload": {
		Name:  "CoordinatorStoppedPayload",
		Value: hooks.CoordinatorStoppedPayload{},
	},
	"CoordinatorFailedPayload": {
		Name:  "CoordinatorFailedPayload",
		Value: hooks.CoordinatorFailedPayload{},
	},
	sdkCoordinatorSpawnPatchValue: {Name: sdkCoordinatorSpawnPatchValue, Value: hooks.CoordinatorSpawnPatch{}},
	"CoordinatorObservationPatch": {Name: "CoordinatorObservationPatch", Value: hooks.CoordinatorObservationPatch{}},
	sdkTaskRunClaimCriteriaValue:  {Name: sdkTaskRunClaimCriteriaValue, Value: hooks.TaskRunClaimCriteria{}},
	sdkTaskRunContextValue:        {Name: sdkTaskRunContextValue, Value: hooks.TaskRunContext{}},
	sdkTaskRunEnqueuedPayloadValue: {
		Name:  sdkTaskRunEnqueuedPayloadValue,
		Value: hooks.TaskRunEnqueuedPayload{},
	},
	sdkTaskRunPreClaimPayloadValue: {
		Name:  sdkTaskRunPreClaimPayloadValue,
		Value: hooks.TaskRunPreClaimPayload{},
	},
	sdkTaskRunPostClaimPayloadValue: {
		Name:  sdkTaskRunPostClaimPayloadValue,
		Value: hooks.TaskRunPostClaimPayload{},
	},
	sdkTaskRunLeasePayloadValue: {
		Name:  sdkTaskRunLeasePayloadValue,
		Value: hooks.TaskRunLeasePayload{},
	},
	"TaskRunLeaseExtendedPayload": {
		Name:  "TaskRunLeaseExtendedPayload",
		Value: hooks.TaskRunLeaseExtendedPayload{},
	},
	"TaskRunLeaseExpiredPayload": {
		Name:  "TaskRunLeaseExpiredPayload",
		Value: hooks.TaskRunLeaseExpiredPayload{},
	},
	"TaskRunLeaseRecoveredPayload": {
		Name:  "TaskRunLeaseRecoveredPayload",
		Value: hooks.TaskRunLeaseRecoveredPayload{},
	},
	"TaskRunReleasedPayload": {
		Name:  "TaskRunReleasedPayload",
		Value: hooks.TaskRunReleasedPayload{},
	},
	"TaskRunCompletedPayload": {
		Name:  "TaskRunCompletedPayload",
		Value: hooks.TaskRunCompletedPayload{},
	},
	"TaskRunFailedPayload": {
		Name:  "TaskRunFailedPayload",
		Value: hooks.TaskRunFailedPayload{},
	},
	sdkTaskRunPreClaimPatchValue: {Name: sdkTaskRunPreClaimPatchValue, Value: hooks.TaskRunPreClaimPatch{}},
	"TaskRunObservationPatch":    {Name: "TaskRunObservationPatch", Value: hooks.TaskRunObservationPatch{}},
	sdkLoopContextValue:          {Name: sdkLoopContextValue, Value: hooks.LoopContext{}},
	sdkLoopLifecyclePayloadValue: {Name: sdkLoopLifecyclePayloadValue, Value: hooks.LoopLifecyclePayload{}},
	sdkLoopStartedPayloadValue:   {Name: sdkLoopStartedPayloadValue, Value: hooks.LoopStartedPayload{}},
	sdkLoopTerminalPayloadValue:  {Name: sdkLoopTerminalPayloadValue, Value: hooks.LoopTerminalPayload{}},
	sdkLoopGenerationPrePayloadValue: {
		Name:  sdkLoopGenerationPrePayloadValue,
		Value: hooks.LoopGenerationPrePayload{},
	},
	sdkLoopGenerationPostPayloadValue: {
		Name:  sdkLoopGenerationPostPayloadValue,
		Value: hooks.LoopGenerationPostPayload{},
	},
	sdkLoopGatePrePayloadValue: {
		Name:  sdkLoopGatePrePayloadValue,
		Value: hooks.LoopGatePrePayload{},
	},
	sdkLoopGatePostPayloadValue: {
		Name:  sdkLoopGatePostPayloadValue,
		Value: hooks.LoopGatePostPayload{},
	},
	sdkLoopNodeTerminalPayloadValue: {
		Name:  sdkLoopNodeTerminalPayloadValue,
		Value: hooks.LoopNodeTerminalPayload{},
	},
	sdkLoopControlPatchValue: {Name: sdkLoopControlPatchValue, Value: hooks.LoopControlPatch{}},
	sdkLoopGenerationPrePatchValue: {
		Name:  sdkLoopGenerationPrePatchValue,
		Value: hooks.LoopGenerationPrePatch{},
	},
	sdkLoopGatePrePatchValue:     {Name: sdkLoopGatePrePatchValue, Value: hooks.LoopGatePrePatch{}},
	sdkLoopObservationPatchValue: {Name: sdkLoopObservationPatchValue, Value: hooks.LoopObservationPatch{}},
	sdkTaskContextValue:          {Name: sdkTaskContextValue, Value: hooks.TaskContext{}},
	sdkTaskBlockedPayloadValue: {
		Name:  sdkTaskBlockedPayloadValue,
		Value: hooks.TaskBlockedPayload{},
	},
	sdkTaskUnblockedPayloadValue: {
		Name:  sdkTaskUnblockedPayloadValue,
		Value: hooks.TaskUnblockedPayload{},
	},
	sdkTaskNeedsAttentionPayloadValue: {
		Name:  sdkTaskNeedsAttentionPayloadValue,
		Value: hooks.TaskNeedsAttentionPayload{},
	},
	sdkTaskRecoveredPayloadValue:     {Name: sdkTaskRecoveredPayloadValue, Value: hooks.TaskRecoveredPayload{}},
	sdkTaskStatusChangedPayloadValue: {Name: sdkTaskStatusChangedPayloadValue, Value: hooks.TaskStatusChangedPayload{}},
	sdkTaskObservationPatchValue:     {Name: sdkTaskObservationPatchValue, Value: hooks.TaskObservationPatch{}},
	sdkPermissionSetValue:            {Name: sdkPermissionSetValue, Value: hooks.PermissionSet{}},
	sdkSpawnContextValue:             {Name: sdkSpawnContextValue, Value: hooks.SpawnContext{}},
	sdkSpawnPreCreatePayloadValue: {
		Name:  sdkSpawnPreCreatePayloadValue,
		Value: hooks.SpawnPreCreatePayload{},
	},
	sdkSpawnLifecyclePayloadValue: {
		Name:  sdkSpawnLifecyclePayloadValue,
		Value: hooks.SpawnLifecyclePayload{},
	},
	"SpawnCreatedPayload": {
		Name:  "SpawnCreatedPayload",
		Value: hooks.SpawnCreatedPayload{},
	},
	"SpawnParentStoppedPayload": {
		Name:  "SpawnParentStoppedPayload",
		Value: hooks.SpawnParentStoppedPayload{},
	},
	"SpawnTTLExpiredPayload": {
		Name:  "SpawnTTLExpiredPayload",
		Value: hooks.SpawnTTLExpiredPayload{},
	},
	"SpawnReapedPayload": {
		Name:  "SpawnReapedPayload",
		Value: hooks.SpawnReapedPayload{},
	},
	sdkSpawnCreatePatchValue: {Name: sdkSpawnCreatePatchValue, Value: hooks.SpawnCreatePatch{}},
	"SpawnObservationPatch":  {Name: "SpawnObservationPatch", Value: hooks.SpawnObservationPatch{}},
	sdkAutonomyMatcherValue:  {Name: sdkAutonomyMatcherValue, Value: hooks.AutonomyMatcher{}},
}, networkParticipationNamedHookTypes(), windowManagerNamedHookTypes())

func namedHookType(name string) (NamedType, error) {
	namedType, ok := namedHookTypes[name]
	if !ok {
		return NamedType{}, fmt.Errorf("unknown hook contract type %q", name)
	}
	return namedType, nil
}
