package hooks

func cloneSessionPreCreatePayload(payload SessionPreCreatePayload) SessionPreCreatePayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	return payload
}

func cloneSessionLifecyclePayload(payload SessionLifecyclePayload) SessionLifecyclePayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	return payload
}

func cloneSandboxPreparePayload(payload SandboxPreparePayload) SandboxPreparePayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.Profile = cloneSandboxProfilePayload(payload.Profile)
	payload.LocalAdditionalDirs = cloneStringSlice(payload.LocalAdditionalDirs)
	payload.AgentEnv = cloneStringSlice(payload.AgentEnv)
	payload.EnvOverrides = cloneStringMap(payload.EnvOverrides)
	return payload
}

func cloneSandboxReadyPayload(payload SandboxReadyPayload) SandboxReadyPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.RuntimeAdditionalDirs = cloneStringSlice(payload.RuntimeAdditionalDirs)
	return payload
}

func cloneSandboxSyncBeforePayload(payload SandboxSyncBeforePayload) SandboxSyncBeforePayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ExcludePatterns = cloneStringSlice(payload.ExcludePatterns)
	return payload
}

func cloneSandboxSyncAfterPayload(payload SandboxSyncAfterPayload) SandboxSyncAfterPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.Errors = cloneStringSlice(payload.Errors)
	return payload
}

func cloneSandboxStopPayload(payload SandboxStopPayload) SandboxStopPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	return payload
}

func cloneInputPreSubmitPayload(payload InputPreSubmitPayload) InputPreSubmitPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ContextBlocks = cloneContextBlocks(payload.ContextBlocks)
	return payload
}

func clonePromptPayload(payload PromptPayload) PromptPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ContextBlocks = cloneContextBlocks(payload.ContextBlocks)
	return payload
}

func cloneEventRecordPayload(payload EventRecordPayload) EventRecordPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.Content = cloneRawJSON(payload.Content)
	return payload
}

func cloneAutomationJobPreFirePayload(payload AutomationJobPreFirePayload) AutomationJobPreFirePayload {
	payload.Schedule = cloneAutomationSchedulePayload(payload.Schedule)
	payload.Payload = cloneAnyMap(payload.Payload)
	return payload
}

func cloneAutomationTriggerPreFirePayload(payload AutomationTriggerPreFirePayload) AutomationTriggerPreFirePayload {
	payload.Payload = cloneAnyMap(payload.Payload)
	return payload
}

func cloneSpawnPreCreatePayload(payload SpawnPreCreatePayload) SpawnPreCreatePayload {
	payload.SpawnContext = cloneSpawnContext(payload.SpawnContext)
	payload.ParentPermissions = clonePermissionSet(payload.ParentPermissions)
	payload.ChildPermissions = clonePermissionSet(payload.ChildPermissions)
	return payload
}

func cloneSpawnLifecyclePayload(payload SpawnLifecyclePayload) SpawnLifecyclePayload {
	payload.SpawnContext = cloneSpawnContext(payload.SpawnContext)
	payload.ParentPermissions = clonePermissionSet(payload.ParentPermissions)
	payload.ChildPermissions = clonePermissionSet(payload.ChildPermissions)
	return payload
}

func clonePermissionSet(src *PermissionSet) *PermissionSet {
	if src == nil {
		return nil
	}
	return &PermissionSet{
		Tools:           cloneStringSlice(src.Tools),
		Skills:          cloneStringSlice(src.Skills),
		MCPServers:      cloneStringSlice(src.MCPServers),
		WorkspacePaths:  cloneStringSlice(src.WorkspacePaths),
		NetworkChannels: cloneStringSlice(src.NetworkChannels),
		SandboxProfiles: cloneStringSlice(src.SandboxProfiles),
	}
}

func cloneAgentPreStartPayload(payload AgentPreStartPayload) AgentPreStartPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.Args = cloneStringSlice(payload.Args)
	return payload
}

func cloneAgentLifecyclePayload(payload AgentLifecyclePayload) AgentLifecyclePayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.Args = cloneStringSlice(payload.Args)
	return payload
}

func cloneSessionMessagePersistedPayload(payload SessionMessagePersistedPayload) SessionMessagePersistedPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.Raw = cloneRawJSON(payload.Raw)
	payload.Persisted = cloneRawJSON(payload.Persisted)
	return payload
}

func cloneMessagePayload(payload MessagePayload) MessagePayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.Raw = cloneRawJSON(payload.Raw)
	return payload
}

func cloneToolPreCallPayload(payload ToolPreCallPayload) ToolPreCallPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ToolInput = cloneRawJSON(payload.ToolInput)
	return payload
}

func cloneToolPostCallPayload(payload ToolPostCallPayload) ToolPostCallPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ToolInput = cloneRawJSON(payload.ToolInput)
	payload.ToolResult = cloneRawJSON(payload.ToolResult)
	return payload
}

func cloneToolPostErrorPayload(payload ToolPostErrorPayload) ToolPostErrorPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ToolInput = cloneRawJSON(payload.ToolInput)
	return payload
}

func clonePermissionRequestPayload(payload PermissionRequestPayload) PermissionRequestPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ToolInput = cloneRawJSON(payload.ToolInput)
	payload.ToolCall = clonePermissionToolCall(payload.ToolCall)
	payload.Options = clonePermissionOptions(payload.Options)
	return payload
}

func clonePermissionResolutionPayload(payload PermissionResolutionPayload) PermissionResolutionPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ToolInput = cloneRawJSON(payload.ToolInput)
	payload.ToolCall = clonePermissionToolCall(payload.ToolCall)
	return payload
}

func cloneContextCompactPayload(payload ContextCompactPayload) ContextCompactPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	payload.ContextBlocks = cloneContextBlocks(payload.ContextBlocks)
	return payload
}

func cloneAgentHeartbeatWakeBeforePayload(payload AgentHeartbeatWakeBeforePayload) AgentHeartbeatWakeBeforePayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	return payload
}

func cloneAgentHeartbeatWakeAfterPayload(payload AgentHeartbeatWakeAfterPayload) AgentHeartbeatWakeAfterPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	return payload
}

func cloneSessionHealthUpdateAfterPayload(payload SessionHealthUpdateAfterPayload) SessionHealthUpdateAfterPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	return payload
}

func cloneTurnPayload(payload TurnPayload) TurnPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	return payload
}
