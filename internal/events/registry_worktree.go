package events

var worktreeRegistryEntries = []Metadata{
	global(success(WorktreeCreated, "worktree", ComponentWorktree)),
	global(success(WorktreeAdopted, "worktree", ComponentWorktree)),
	global(success(WorktreeRemoved, "worktree", ComponentWorktree)),
	global(warning(WorktreeMissing, "worktree", ComponentWorktree)),
	global(info(WorktreeDismissed, "worktree", ComponentWorktree)),
	global(info(WorktreeCreationCanceled, "worktree", ComponentWorktree)),
	global(warning(WorktreeSetupFailed, "worktree", ComponentWorktree)),
	global(info(WorktreeStatusRefreshed, "worktree", ComponentWorktree)),
	global(success(WorktreeBranchReclaimed, "worktree", ComponentWorktree)),
	global(info(WorktreeExitActionStarted, "worktree.exit", ComponentWorktree)),
	global(info(WorktreeExitActionStep, "worktree.exit", ComponentWorktree)),
	global(success(WorktreeExitActionCompleted, "worktree.exit", ComponentWorktree)),
	global(failure(WorktreeExitActionFailed, "worktree.exit", ComponentWorktree)),
	global(info(WorktreeExitActionCanceled, "worktree.exit", ComponentWorktree)),
}
