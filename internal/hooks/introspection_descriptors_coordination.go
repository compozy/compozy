package hooks

func coordinationHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookCoordinatorPreSpawn: {
		Event:         HookCoordinatorPreSpawn,
		Family:        HookEventFamilyCoordinator,
		SyncEligible:  true,
		PayloadSchema: "CoordinatorPreSpawnPayload",
		PatchSchema:   "CoordinatorSpawnPatch",
	},
		HookCoordinatorSpawned: {
			Event:         HookCoordinatorSpawned,
			Family:        HookEventFamilyCoordinator,
			SyncEligible:  true,
			PayloadSchema: "CoordinatorSpawnedPayload",
			PatchSchema:   introspectionCoordinatorObservationPatchValue,
		},
		HookCoordinatorDecision: {
			Event:         HookCoordinatorDecision,
			Family:        HookEventFamilyCoordinator,
			SyncEligible:  true,
			PayloadSchema: "CoordinatorDecisionPayload",
			PatchSchema:   introspectionCoordinatorObservationPatchValue,
		},
		HookCoordinatorStopped: {
			Event:         HookCoordinatorStopped,
			Family:        HookEventFamilyCoordinator,
			SyncEligible:  true,
			PayloadSchema: "CoordinatorStoppedPayload",
			PatchSchema:   introspectionCoordinatorObservationPatchValue,
		},
		HookCoordinatorFailed: {
			Event:         HookCoordinatorFailed,
			Family:        HookEventFamilyCoordinator,
			SyncEligible:  true,
			PayloadSchema: "CoordinatorFailedPayload",
			PatchSchema:   introspectionCoordinatorObservationPatchValue,
		},
		HookTaskBlocked: {
			Event: HookTaskBlocked, Family: HookEventFamilyTask, SyncEligible: true,
			PayloadSchema: "TaskBlockedPayload", PatchSchema: introspectionTaskObservationPatchValue,
		},
		HookTaskUnblocked: {
			Event: HookTaskUnblocked, Family: HookEventFamilyTask, SyncEligible: true,
			PayloadSchema: "TaskUnblockedPayload", PatchSchema: introspectionTaskObservationPatchValue,
		},
		HookTaskNeedsAttention: {
			Event: HookTaskNeedsAttention, Family: HookEventFamilyTask, SyncEligible: true,
			PayloadSchema: "TaskNeedsAttentionPayload", PatchSchema: introspectionTaskObservationPatchValue,
		},
		HookTaskRecovered: {
			Event: HookTaskRecovered, Family: HookEventFamilyTask, SyncEligible: true,
			PayloadSchema: "TaskRecoveredPayload", PatchSchema: introspectionTaskObservationPatchValue,
		},
		HookTaskStatusChanged: {
			Event:         HookTaskStatusChanged,
			Family:        HookEventFamilyTask,
			SyncEligible:  false,
			PayloadSchema: "TaskStatusChangedPayload",
			PatchSchema:   introspectionTaskObservationPatchValue,
		}}
}
