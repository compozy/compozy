package hooks

import "strings"

func applyNoop[P any, R any](payload P, _ R) P {
	return payload
}

func automationFirePatchDenied(patch AutomationFirePatch) bool {
	return patch.Cancel
}

func applyAutomationJobPreFirePatch(
	payload AutomationJobPreFirePayload,
	patch AutomationFirePatch,
) AutomationJobPreFirePayload {
	if patch.Prompt != nil {
		payload.Prompt = *patch.Prompt
	}
	return payload
}

func applyAutomationTriggerPreFirePatch(
	payload AutomationTriggerPreFirePayload,
	patch AutomationFirePatch,
) AutomationTriggerPreFirePayload {
	if patch.Prompt != nil {
		payload.Prompt = *patch.Prompt
	}
	return payload
}

func applySessionContextPatch(payload SessionContext, patch SessionCreatePatch) SessionContext {
	if patch.SessionName != nil {
		payload.SessionName = *patch.SessionName
	}
	if patch.SessionType != nil {
		payload.SessionType = *patch.SessionType
	}
	if patch.AgentName != nil {
		payload.AgentName = *patch.AgentName
	}
	if patch.WorkspaceID != nil {
		payload.WorkspaceID = *patch.WorkspaceID
	}
	if patch.Workspace != nil {
		payload.Workspace = *patch.Workspace
	}
	return payload
}

func applySessionCreatePatch(payload SessionPreCreatePayload, patch SessionCreatePatch) SessionPreCreatePayload {
	payload.SessionContext = applySessionContextPatch(payload.SessionContext, patch)
	return payload
}

func applySessionLifecyclePatch(payload SessionLifecyclePayload, patch SessionCreatePatch) SessionLifecyclePayload {
	payload.SessionContext = applySessionContextPatch(payload.SessionContext, patch)
	return payload
}

func applySandboxPreparePatch(
	payload SandboxPreparePayload,
	patch SandboxPreparePatch,
) SandboxPreparePayload {
	if patch.Deny {
		payload.Denied = true
		payload.DenyReason = patch.DenyReason
	}
	if patch.EnvOverrides != nil {
		payload.EnvOverrides = cloneStringMap(patch.EnvOverrides)
	}
	return payload
}

func applySandboxSyncBeforePatch(
	payload SandboxSyncBeforePayload,
	patch SandboxSyncBeforePatch,
) SandboxSyncBeforePayload {
	if patch.Deny {
		payload.Denied = true
		payload.DenyReason = patch.DenyReason
	}
	if patch.ExcludePatterns != nil {
		payload.ExcludePatterns = append([]string(nil), patch.ExcludePatterns...)
	}
	return payload
}

func applySandboxStopPatch(payload SandboxStopPayload, patch SandboxStopPatch) SandboxStopPayload {
	if patch.Deny {
		payload.Denied = true
		payload.DenyReason = patch.DenyReason
	}
	return payload
}

func applyInputPreSubmitPatch(payload InputPreSubmitPayload, patch InputPreSubmitPatch) InputPreSubmitPayload {
	if patch.Message != nil {
		payload.Message = *patch.Message
	}
	if patch.ContextBlocks != nil {
		payload.ContextBlocks = cloneContextBlocks(patch.ContextBlocks)
	}
	return payload
}

func applyPromptPatch(payload PromptPayload, patch PromptPatch) PromptPayload {
	if patch.Prompt != nil {
		payload.Prompt = *patch.Prompt
	}
	if patch.ContextBlocks != nil {
		payload.ContextBlocks = cloneContextBlocks(patch.ContextBlocks)
	}
	return payload
}

func applyAgentStartPatch(payload AgentPreStartPayload, patch AgentStartPatch) AgentPreStartPayload {
	if patch.Command != nil {
		payload.Command = *patch.Command
	}
	if patch.Args != nil {
		payload.Args = append([]string(nil), patch.Args...)
	}
	if patch.Cwd != nil {
		payload.Cwd = *patch.Cwd
	}
	return payload
}

func applyMessagePatch(payload MessagePayload, patch MessagePatch) MessagePayload {
	if patch.Role != nil {
		payload.Role = *patch.Role
	}
	if patch.DeltaType != nil {
		payload.DeltaType = *patch.DeltaType
	}
	if patch.Text != nil {
		payload.Text = *patch.Text
	}
	return payload
}

func applyToolCallPatch(payload ToolPreCallPayload, patch ToolCallPatch) ToolPreCallPayload {
	if patch.ToolID != nil {
		payload.ToolID = *patch.ToolID
	}
	if patch.ReadOnly != nil {
		payload.ReadOnly = *patch.ReadOnly
	}
	if patch.ToolInput != nil {
		payload.ToolInput = cloneRawMessage(patch.ToolInput)
	}
	return payload
}

func applyToolResultPatch(payload ToolPostCallPayload, patch ToolResultPatch) ToolPostCallPayload {
	if patch.Title != nil {
		payload.Title = *patch.Title
	}
	if patch.ToolResult != nil {
		payload.ToolResult = cloneRawMessage(patch.ToolResult)
	}
	return payload
}

func applyToolPostErrorPatch(payload ToolPostErrorPayload, patch ToolPostErrorPatch) ToolPostErrorPayload {
	if patch.Title != nil {
		payload.Title = *patch.Title
	}
	if patch.Error != nil {
		payload.Error = *patch.Error
	}
	return payload
}

func mergePermissionRequestPatch(
	payload PermissionRequestPayload,
	patch PermissionRequestPatch,
) PermissionRequestPayload {
	if patch.Decision != nil {
		payload.Decision = *patch.Decision
	}
	if patch.Deny {
		payload.Decision = permissionDecisionDeny
	}
	if patch.DecisionClass != nil {
		payload.DecisionClass = *patch.DecisionClass
	}
	return payload
}

func applyContextCompactionPatch(payload ContextCompactPayload, patch ContextCompactionPatch) ContextCompactPayload {
	if patch.Reason != nil {
		payload.Reason = *patch.Reason
	}
	if patch.Strategy != nil {
		payload.Strategy = *patch.Strategy
	}
	if patch.ContextBlocks != nil {
		payload.ContextBlocks = cloneContextBlocks(patch.ContextBlocks)
	}
	return payload
}

func applyCoordinatorSpawnPatch(
	payload CoordinatorPreSpawnPayload,
	patch CoordinatorSpawnPatch,
) CoordinatorPreSpawnPayload {
	if patch.Deny {
		payload.Denied = true
		payload.DenyReason = patch.DenyReason
	}
	if patch.AgentName != nil {
		payload.AgentName = strings.TrimSpace(*patch.AgentName)
	}
	if patch.Provider != nil {
		payload.Provider = strings.TrimSpace(*patch.Provider)
	}
	if patch.Model != nil {
		payload.Model = strings.TrimSpace(*patch.Model)
	}
	return payload
}

func applyTaskRunPreClaimPatch(
	payload TaskRunPreClaimPayload,
	patch TaskRunPreClaimPatch,
) TaskRunPreClaimPayload {
	if patch.Deny {
		payload.Denied = true
		payload.DenyReason = patch.DenyReason
	}
	if patch.AddRequiredCapabilities != nil {
		payload.Criteria.RequiredCapabilities = unionStringSet(
			payload.Criteria.RequiredCapabilities,
			patch.AddRequiredCapabilities,
		)
	}
	if patch.PriorityMin != nil && *patch.PriorityMin > payload.Criteria.PriorityMin {
		payload.Criteria.PriorityMin = *patch.PriorityMin
	}
	return payload
}

func applyLoopGenerationPrePatch(
	payload LoopGenerationPrePayload,
	patch LoopGenerationPrePatch,
) LoopGenerationPrePayload {
	if patch.Deny {
		payload.Denied = true
		payload.DenyReason = patch.DenyReason
	}
	return payload
}

func applyLoopGatePrePatch(
	payload LoopGatePrePayload,
	patch LoopGatePrePatch,
) LoopGatePrePayload {
	if patch.Deny {
		payload.Denied = true
		payload.DenyReason = patch.DenyReason
	}
	return payload
}

func applySpawnCreatePatch(payload SpawnPreCreatePayload, patch SpawnCreatePatch) SpawnPreCreatePayload {
	if patch.Deny {
		payload.Denied = true
		payload.DenyReason = patch.DenyReason
	}
	if patch.AgentName != nil {
		payload.AgentName = strings.TrimSpace(*patch.AgentName)
	}
	if patch.SpawnRole != nil {
		payload.SpawnRole = strings.TrimSpace(*patch.SpawnRole)
	}
	if patch.TTLSeconds != nil {
		payload.TTLSeconds = *patch.TTLSeconds
	}
	if patch.ChildPermissions != nil {
		payload.ChildPermissions = normalizePermissionSet(patch.ChildPermissions)
	}
	return payload
}
