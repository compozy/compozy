package hooks

func executionHookEventDescriptors() map[HookEvent]EventDescriptor {
	return mergeHookEventDescriptors(
		taskRunFamilyHookEventDescriptors(),
		loopFamilyHookEventDescriptors(),
		spawnFamilyHookEventDescriptors(),
	)
}
func taskRunFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookTaskRunEnqueued: {
		Event:         HookTaskRunEnqueued,
		Family:        HookEventFamilyTaskRun,
		SyncEligible:  true,
		PayloadSchema: "TaskRunEnqueuedPayload",
		PatchSchema:   introspectionTaskRunObservationPatchValue,
	},
		HookTaskRunPreClaim: {
			Event:         HookTaskRunPreClaim,
			Family:        HookEventFamilyTaskRun,
			SyncEligible:  true,
			PayloadSchema: "TaskRunPreClaimPayload",
			PatchSchema:   "TaskRunPreClaimPatch",
		},
		HookTaskRunPostClaim: {
			Event:         HookTaskRunPostClaim,
			Family:        HookEventFamilyTaskRun,
			SyncEligible:  true,
			PayloadSchema: "TaskRunPostClaimPayload",
			PatchSchema:   introspectionTaskRunObservationPatchValue,
		},
		HookTaskRunLeaseExtended: {
			Event:         HookTaskRunLeaseExtended,
			Family:        HookEventFamilyTaskRun,
			SyncEligible:  true,
			PayloadSchema: "TaskRunLeaseExtendedPayload",
			PatchSchema:   introspectionTaskRunObservationPatchValue,
		},
		HookTaskRunLeaseExpired: {
			Event:         HookTaskRunLeaseExpired,
			Family:        HookEventFamilyTaskRun,
			SyncEligible:  true,
			PayloadSchema: "TaskRunLeaseExpiredPayload",
			PatchSchema:   introspectionTaskRunObservationPatchValue,
		},
		HookTaskRunLeaseRecovered: {
			Event:         HookTaskRunLeaseRecovered,
			Family:        HookEventFamilyTaskRun,
			SyncEligible:  true,
			PayloadSchema: "TaskRunLeaseRecoveredPayload",
			PatchSchema:   introspectionTaskRunObservationPatchValue,
		},
		HookTaskRunReleased: {
			Event:         HookTaskRunReleased,
			Family:        HookEventFamilyTaskRun,
			SyncEligible:  true,
			PayloadSchema: "TaskRunReleasedPayload",
			PatchSchema:   introspectionTaskRunObservationPatchValue,
		},
		HookTaskRunCompleted: {
			Event:         HookTaskRunCompleted,
			Family:        HookEventFamilyTaskRun,
			SyncEligible:  true,
			PayloadSchema: "TaskRunCompletedPayload",
			PatchSchema:   introspectionTaskRunObservationPatchValue,
		},
		HookTaskRunFailed: {
			Event:         HookTaskRunFailed,
			Family:        HookEventFamilyTaskRun,
			SyncEligible:  true,
			PayloadSchema: "TaskRunFailedPayload",
			PatchSchema:   introspectionTaskRunObservationPatchValue,
		}}
}
func loopFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookLoopStarted: {
		Event:         HookLoopStarted,
		Family:        HookEventFamilyLoop,
		SyncEligible:  false,
		PayloadSchema: "LoopStartedPayload",
		PatchSchema:   introspectionLoopObservationPatchValue,
	},
		HookLoopGenerationPre: {
			Event:         HookLoopGenerationPre,
			Family:        HookEventFamilyLoop,
			SyncEligible:  true,
			PayloadSchema: "LoopGenerationPrePayload",
			PatchSchema:   "LoopGenerationPrePatch",
		},
		HookLoopGenerationPost: {
			Event:         HookLoopGenerationPost,
			Family:        HookEventFamilyLoop,
			SyncEligible:  false,
			PayloadSchema: "LoopGenerationPostPayload",
			PatchSchema:   introspectionLoopObservationPatchValue,
		},
		HookLoopGatePre: {
			Event:         HookLoopGatePre,
			Family:        HookEventFamilyLoop,
			SyncEligible:  true,
			PayloadSchema: "LoopGatePrePayload",
			PatchSchema:   "LoopGatePrePatch",
		},
		HookLoopGatePost: {
			Event:         HookLoopGatePost,
			Family:        HookEventFamilyLoop,
			SyncEligible:  false,
			PayloadSchema: "LoopGatePostPayload",
			PatchSchema:   introspectionLoopObservationPatchValue,
		},
		HookLoopNodeTerminal: {
			Event:         HookLoopNodeTerminal,
			Family:        HookEventFamilyLoop,
			SyncEligible:  false,
			PayloadSchema: "LoopNodeTerminalPayload",
			PatchSchema:   introspectionLoopObservationPatchValue,
		},
		HookLoopTerminal: {
			Event:         HookLoopTerminal,
			Family:        HookEventFamilyLoop,
			SyncEligible:  false,
			PayloadSchema: "LoopTerminalPayload",
			PatchSchema:   introspectionLoopObservationPatchValue,
		}}
}
func spawnFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookSpawnPreCreate: {
		Event:         HookSpawnPreCreate,
		Family:        HookEventFamilySpawn,
		SyncEligible:  true,
		PayloadSchema: "SpawnPreCreatePayload",
		PatchSchema:   "SpawnCreatePatch",
	},
		HookSpawnCreated: {
			Event:         HookSpawnCreated,
			Family:        HookEventFamilySpawn,
			SyncEligible:  true,
			PayloadSchema: "SpawnCreatedPayload",
			PatchSchema:   introspectionSpawnObservationPatchValue,
		},
		HookSpawnParentStopped: {
			Event:         HookSpawnParentStopped,
			Family:        HookEventFamilySpawn,
			SyncEligible:  true,
			PayloadSchema: "SpawnParentStoppedPayload",
			PatchSchema:   introspectionSpawnObservationPatchValue,
		},
		HookSpawnTTLExpired: {
			Event:         HookSpawnTTLExpired,
			Family:        HookEventFamilySpawn,
			SyncEligible:  true,
			PayloadSchema: "SpawnTTLExpiredPayload",
			PatchSchema:   introspectionSpawnObservationPatchValue,
		},
		HookSpawnReaped: {
			Event:         HookSpawnReaped,
			Family:        HookEventFamilySpawn,
			SyncEligible:  true,
			PayloadSchema: "SpawnReapedPayload",
			PatchSchema:   introspectionSpawnObservationPatchValue,
		}}
}
