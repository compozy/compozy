package hooks

func agentHookEventDescriptors() map[HookEvent]EventDescriptor {
	return mergeHookEventDescriptors(automationFamilyHookEventDescriptors(), agentFamilyHookEventDescriptors())
}
func automationFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookAutomationJobPreFire: {
		Event:         HookAutomationJobPreFire,
		Family:        HookEventFamilyAutomation,
		SyncEligible:  true,
		PayloadSchema: "AutomationJobPreFirePayload",
		PatchSchema:   introspectionAutomationFirePatchValue,
	},
		HookAutomationJobPostFire: {
			Event:         HookAutomationJobPostFire,
			Family:        HookEventFamilyAutomation,
			SyncEligible:  false,
			PayloadSchema: "AutomationJobPostFirePayload",
			PatchSchema:   introspectionAutomationObservationPatchValue,
		},
		HookAutomationTriggerPreFire: {
			Event:         HookAutomationTriggerPreFire,
			Family:        HookEventFamilyAutomation,
			SyncEligible:  true,
			PayloadSchema: "AutomationTriggerPreFirePayload",
			PatchSchema:   introspectionAutomationFirePatchValue,
		},
		HookAutomationTriggerPostFire: {
			Event:         HookAutomationTriggerPostFire,
			Family:        HookEventFamilyAutomation,
			SyncEligible:  false,
			PayloadSchema: "AutomationTriggerPostFirePayload",
			PatchSchema:   introspectionAutomationObservationPatchValue,
		},
		HookAutomationRunCompleted: {
			Event:         HookAutomationRunCompleted,
			Family:        HookEventFamilyAutomation,
			SyncEligible:  false,
			PayloadSchema: "AutomationRunCompletedPayload",
			PatchSchema:   introspectionAutomationObservationPatchValue,
		},
		HookAutomationRunFailed: {
			Event:         HookAutomationRunFailed,
			Family:        HookEventFamilyAutomation,
			SyncEligible:  false,
			PayloadSchema: "AutomationRunFailedPayload",
			PatchSchema:   introspectionAutomationObservationPatchValue,
		}}
}
func agentFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookAgentPreStart: {
		Event:         HookAgentPreStart,
		Family:        HookEventFamilyAgent,
		SyncEligible:  true,
		PayloadSchema: "AgentPreStartPayload",
		PatchSchema:   "AgentStartPatch",
	},
		HookAgentSpawned: {
			Event:         HookAgentSpawned,
			Family:        HookEventFamilyAgent,
			SyncEligible:  true,
			PayloadSchema: "AgentSpawnedPayload",
			PatchSchema:   "AgentSpawnedPatch",
		},
		HookAgentCrashed: {
			Event:         HookAgentCrashed,
			Family:        HookEventFamilyAgent,
			SyncEligible:  true,
			PayloadSchema: "AgentCrashedPayload",
			PatchSchema:   "AgentCrashedPatch",
		},
		HookAgentStopped: {
			Event:         HookAgentStopped,
			Family:        HookEventFamilyAgent,
			SyncEligible:  true,
			PayloadSchema: "AgentStoppedPayload",
			PatchSchema:   "AgentStoppedPatch",
		},
		HookAgentSoulSnapshotResolved: {
			Event:         HookAgentSoulSnapshotResolved,
			Family:        HookEventFamilyAgent,
			SyncEligible:  false,
			PayloadSchema: "AgentSoulSnapshotResolvedPayload",
			PatchSchema:   introspectionAuthoredContextObservationPatchValue,
		},
		HookAgentSoulMutationAfter: {
			Event:         HookAgentSoulMutationAfter,
			Family:        HookEventFamilyAgent,
			SyncEligible:  false,
			PayloadSchema: "AgentSoulMutationAfterPayload",
			PatchSchema:   introspectionAuthoredContextObservationPatchValue,
		},
		HookAgentHeartbeatPolicyResolved: {
			Event:         HookAgentHeartbeatPolicyResolved,
			Family:        HookEventFamilyAgent,
			SyncEligible:  false,
			PayloadSchema: "AgentHeartbeatPolicyResolvedPayload",
			PatchSchema:   introspectionAuthoredContextObservationPatchValue,
		},
		HookAgentHeartbeatWakeBefore: {
			Event:         HookAgentHeartbeatWakeBefore,
			Family:        HookEventFamilyAgent,
			SyncEligible:  true,
			PayloadSchema: "AgentHeartbeatWakeBeforePayload",
			PatchSchema:   introspectionAuthoredContextObservationPatchValue,
		},
		HookAgentHeartbeatWakeAfter: {
			Event:         HookAgentHeartbeatWakeAfter,
			Family:        HookEventFamilyAgent,
			SyncEligible:  false,
			PayloadSchema: "AgentHeartbeatWakeAfterPayload",
			PatchSchema:   introspectionAuthoredContextObservationPatchValue,
		}}
}
