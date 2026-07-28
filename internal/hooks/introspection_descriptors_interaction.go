package hooks

func interactionHookEventDescriptors() map[HookEvent]EventDescriptor {
	return mergeHookEventDescriptors(
		turnFamilyHookEventDescriptors(),
		messageFamilyHookEventDescriptors(),
		toolFamilyHookEventDescriptors(),
		permissionFamilyHookEventDescriptors(),
		contextFamilyHookEventDescriptors(),
	)
}
func turnFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookTurnStart: {
		Event:         HookTurnStart,
		Family:        HookEventFamilyTurn,
		SyncEligible:  true,
		PayloadSchema: "TurnStartPayload",
		PatchSchema:   "TurnStartPatch",
	},
		HookTurnEnd: {
			Event:         HookTurnEnd,
			Family:        HookEventFamilyTurn,
			SyncEligible:  true,
			PayloadSchema: "TurnEndPayload",
			PatchSchema:   "TurnEndPatch",
		}}
}
func messageFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookMessageStart: {
		Event:         HookMessageStart,
		Family:        HookEventFamilyMessage,
		SyncEligible:  true,
		PayloadSchema: "MessageStartPayload",
		PatchSchema:   "MessageStartPatch",
	},
		HookMessageDelta: {
			Event:         HookMessageDelta,
			Family:        HookEventFamilyMessage,
			SyncEligible:  false,
			PayloadSchema: "MessageDeltaPayload",
			PatchSchema:   "MessageDeltaPatch",
		},
		HookMessageEnd: {
			Event:         HookMessageEnd,
			Family:        HookEventFamilyMessage,
			SyncEligible:  true,
			PayloadSchema: "MessageEndPayload",
			PatchSchema:   "MessageEndPatch",
		}}
}
func toolFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookToolPreCall: {
		Event:         HookToolPreCall,
		Family:        HookEventFamilyTool,
		SyncEligible:  true,
		PayloadSchema: "ToolPreCallPayload",
		PatchSchema:   "ToolCallPatch",
	},
		HookToolPostCall: {
			Event:         HookToolPostCall,
			Family:        HookEventFamilyTool,
			SyncEligible:  true,
			PayloadSchema: "ToolPostCallPayload",
			PatchSchema:   "ToolResultPatch",
		},
		HookToolPostError: {
			Event:         HookToolPostError,
			Family:        HookEventFamilyTool,
			SyncEligible:  true,
			PayloadSchema: "ToolPostErrorPayload",
			PatchSchema:   "ToolPostErrorPatch",
		}}
}
func permissionFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookPermissionRequest: {
		Event:         HookPermissionRequest,
		Family:        HookEventFamilyPermission,
		SyncEligible:  true,
		PayloadSchema: "PermissionRequestPayload",
		PatchSchema:   "PermissionRequestPatch",
	},
		HookPermissionResolved: {
			Event:         HookPermissionResolved,
			Family:        HookEventFamilyPermission,
			SyncEligible:  false,
			PayloadSchema: "PermissionResolvedPayload",
			PatchSchema:   "PermissionResolvedPatch",
		},
		HookPermissionDenied: {
			Event:         HookPermissionDenied,
			Family:        HookEventFamilyPermission,
			SyncEligible:  false,
			PayloadSchema: "PermissionDeniedPayload",
			PatchSchema:   "PermissionDeniedPatch",
		}}
}
func contextFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookContextPreCompact: {
		Event:         HookContextPreCompact,
		Family:        HookEventFamilyContext,
		SyncEligible:  true,
		PayloadSchema: "ContextPreCompactPayload",
		PatchSchema:   "ContextPreCompactPatch",
	},
		HookContextPostCompact: {
			Event:         HookContextPostCompact,
			Family:        HookEventFamilyContext,
			SyncEligible:  true,
			PayloadSchema: "ContextPostCompactPayload",
			PatchSchema:   "ContextPostCompactPatch",
		}}
}
