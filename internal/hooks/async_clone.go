package hooks

type asyncPayloadCloner[P any] interface {
	cloneForAsync() P
}

func cloneAsyncPayload[P any](payload P) P {
	cloner, ok := any(payload).(asyncPayloadCloner[P])
	if !ok {
		return payload
	}

	return cloner.cloneForAsync()
}

func (payload SessionPreCreatePayload) cloneForAsync() SessionPreCreatePayload {
	return cloneSessionPreCreatePayload(payload)
}

func (payload SessionLifecyclePayload) cloneForAsync() SessionLifecyclePayload {
	return cloneSessionLifecyclePayload(payload)
}

func (payload InputPreSubmitPayload) cloneForAsync() InputPreSubmitPayload {
	return cloneInputPreSubmitPayload(payload)
}

func (payload PromptPayload) cloneForAsync() PromptPayload {
	return clonePromptPayload(payload)
}

func (payload EventRecordPayload) cloneForAsync() EventRecordPayload {
	return cloneEventRecordPayload(payload)
}

func (payload SandboxPreparePayload) cloneForAsync() SandboxPreparePayload {
	return cloneSandboxPreparePayload(payload)
}

func (payload SandboxReadyPayload) cloneForAsync() SandboxReadyPayload {
	return cloneSandboxReadyPayload(payload)
}

func (payload SandboxSyncBeforePayload) cloneForAsync() SandboxSyncBeforePayload {
	return cloneSandboxSyncBeforePayload(payload)
}

func (payload SandboxSyncAfterPayload) cloneForAsync() SandboxSyncAfterPayload {
	return cloneSandboxSyncAfterPayload(payload)
}

func (payload SandboxStopPayload) cloneForAsync() SandboxStopPayload {
	return cloneSandboxStopPayload(payload)
}

func (payload AgentPreStartPayload) cloneForAsync() AgentPreStartPayload {
	return cloneAgentPreStartPayload(payload)
}

func (payload AgentLifecyclePayload) cloneForAsync() AgentLifecyclePayload {
	return cloneAgentLifecyclePayload(payload)
}

func (payload SessionMessagePersistedPayload) cloneForAsync() SessionMessagePersistedPayload {
	return cloneSessionMessagePersistedPayload(payload)
}

func (payload MessagePayload) cloneForAsync() MessagePayload {
	return cloneMessagePayload(payload)
}

func (payload ToolPreCallPayload) cloneForAsync() ToolPreCallPayload {
	return cloneToolPreCallPayload(payload)
}

func (payload ToolPostCallPayload) cloneForAsync() ToolPostCallPayload {
	return cloneToolPostCallPayload(payload)
}

func (payload ToolPostErrorPayload) cloneForAsync() ToolPostErrorPayload {
	return cloneToolPostErrorPayload(payload)
}

func (payload PermissionRequestPayload) cloneForAsync() PermissionRequestPayload {
	return clonePermissionRequestPayload(payload)
}

func (payload PermissionResolutionPayload) cloneForAsync() PermissionResolutionPayload {
	return clonePermissionResolutionPayload(payload)
}

func (payload ContextCompactPayload) cloneForAsync() ContextCompactPayload {
	return cloneContextCompactPayload(payload)
}

func (payload AgentHeartbeatWakeBeforePayload) cloneForAsync() AgentHeartbeatWakeBeforePayload {
	return cloneAgentHeartbeatWakeBeforePayload(payload)
}

func (payload AgentHeartbeatWakeAfterPayload) cloneForAsync() AgentHeartbeatWakeAfterPayload {
	return cloneAgentHeartbeatWakeAfterPayload(payload)
}

func (payload SessionHealthUpdateAfterPayload) cloneForAsync() SessionHealthUpdateAfterPayload {
	return cloneSessionHealthUpdateAfterPayload(payload)
}

func (payload TurnPayload) cloneForAsync() TurnPayload {
	return cloneTurnPayload(payload)
}

func (payload AutomationJobPreFirePayload) cloneForAsync() AutomationJobPreFirePayload {
	return cloneAutomationJobPreFirePayload(payload)
}

func (payload AutomationTriggerPreFirePayload) cloneForAsync() AutomationTriggerPreFirePayload {
	return cloneAutomationTriggerPreFirePayload(payload)
}

func (payload CoordinatorPreSpawnPayload) cloneForAsync() CoordinatorPreSpawnPayload {
	payload.CoordinatorContext = cloneCoordinatorContext(payload.CoordinatorContext)
	return payload
}

func (payload CoordinatorLifecyclePayload) cloneForAsync() CoordinatorLifecyclePayload {
	payload.CoordinatorContext = cloneCoordinatorContext(payload.CoordinatorContext)
	return payload
}

func (payload TaskRunEnqueuedPayload) cloneForAsync() TaskRunEnqueuedPayload {
	return cloneTaskRunEnqueuedPayload(payload)
}

func (payload TaskRunPreClaimPayload) cloneForAsync() TaskRunPreClaimPayload {
	return cloneTaskRunPreClaimPayload(payload)
}

func (payload TaskRunPostClaimPayload) cloneForAsync() TaskRunPostClaimPayload {
	payload.TaskRunContext = cloneTaskRunContext(payload.TaskRunContext)
	return payload
}

func (payload TaskRunLeasePayload) cloneForAsync() TaskRunLeasePayload {
	payload.TaskRunContext = cloneTaskRunContext(payload.TaskRunContext)
	return payload
}

func (payload LoopLifecyclePayload) cloneForAsync() LoopLifecyclePayload {
	payload.LoopContext = cloneLoopContext(payload.LoopContext)
	payload.Details = cloneRawJSON(payload.Details)
	return payload
}

func (payload LoopGenerationPayload) cloneForAsync() LoopGenerationPayload {
	payload.LoopContext = cloneLoopContext(payload.LoopContext)
	payload.Details = cloneRawJSON(payload.Details)
	return payload
}

func (payload LoopGatePayload) cloneForAsync() LoopGatePayload {
	payload.LoopContext = cloneLoopContext(payload.LoopContext)
	payload.Details = cloneRawJSON(payload.Details)
	return payload
}

func (payload LoopNodeTerminalPayload) cloneForAsync() LoopNodeTerminalPayload {
	payload.LoopContext = cloneLoopContext(payload.LoopContext)
	payload.Details = cloneRawJSON(payload.Details)
	return payload
}

func (payload SpawnPreCreatePayload) cloneForAsync() SpawnPreCreatePayload {
	return cloneSpawnPreCreatePayload(payload)
}

func (payload SpawnLifecyclePayload) cloneForAsync() SpawnLifecyclePayload {
	return cloneSpawnLifecyclePayload(payload)
}
