package hooks

const (
	HookWorktreePreCreate HookEvent = "worktree.pre_create"
	HookWorktreePreRemove HookEvent = "worktree.pre_remove"
	HookWorktreeCreated   HookEvent = "worktree.created"
	HookWorktreeAdopted   HookEvent = "worktree.adopted"
	HookWorktreeRemoved   HookEvent = "worktree.removed"
)

func worktreeHookEventDefinitions() []hookEventDefinition {
	return []hookEventDefinition{
		{event: HookWorktreePreCreate, family: HookEventFamilyWorktree, syncEligible: true},
		{event: HookWorktreePreRemove, family: HookEventFamilyWorktree, syncEligible: true},
		{event: HookWorktreeCreated, family: HookEventFamilyWorktree, syncEligible: false},
		{event: HookWorktreeAdopted, family: HookEventFamilyWorktree, syncEligible: false},
		{event: HookWorktreeRemoved, family: HookEventFamilyWorktree, syncEligible: false},
	}
}
