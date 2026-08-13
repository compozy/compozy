package hooks

const (
	worktreeObservationPayloadSchema = "WorktreeObservationPayload"
	worktreeObservationPatchSchema   = "WorktreeObservationPatch"
)

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
			PayloadSchema: worktreeObservationPayloadSchema, PatchSchema: worktreeObservationPatchSchema,
		},
		HookWorktreeAdopted: {
			Event: HookWorktreeAdopted, Family: HookEventFamilyWorktree,
			PayloadSchema: worktreeObservationPayloadSchema, PatchSchema: worktreeObservationPatchSchema,
		},
		HookWorktreeRemoved: {
			Event: HookWorktreeRemoved, Family: HookEventFamilyWorktree,
			PayloadSchema: worktreeObservationPayloadSchema, PatchSchema: worktreeObservationPatchSchema,
		},
	}
}
