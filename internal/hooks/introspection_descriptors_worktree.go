package hooks

func worktreeHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{
		HookWorktreePreCreate: {
			Event: HookWorktreePreCreate, Family: HookEventFamilyWorktree, SyncEligible: true,
			PayloadSchema: "WorktreePreCreatePayload", PatchSchema: "WorktreeControlPatch",
		},
		HookWorktreePreRemove: {
			Event: HookWorktreePreRemove, Family: HookEventFamilyWorktree, SyncEligible: true,
			PayloadSchema: "WorktreePreRemovePayload", PatchSchema: "WorktreeControlPatch",
		},
		HookWorktreeCreated: {
			Event: HookWorktreeCreated, Family: HookEventFamilyWorktree,
			PayloadSchema: "WorktreeObservationPayload", PatchSchema: "WorktreeObservationPatch",
		},
		HookWorktreeAdopted: {
			Event: HookWorktreeAdopted, Family: HookEventFamilyWorktree,
			PayloadSchema: "WorktreeObservationPayload", PatchSchema: "WorktreeObservationPatch",
		},
		HookWorktreeRemoved: {
			Event: HookWorktreeRemoved, Family: HookEventFamilyWorktree,
			PayloadSchema: "WorktreeObservationPayload", PatchSchema: "WorktreeObservationPatch",
		},
	}
}
