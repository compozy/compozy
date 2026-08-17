package hooks

var hookEventDescriptors = mergeHookEventDescriptors(
	sessionHookEventDescriptors(),
	sessionAttentionHookEventDescriptors(),
	agentHookEventDescriptors(),
	interactionHookEventDescriptors(),
	coordinationHookEventDescriptors(),
	executionHookEventDescriptors(),
	networkHookEventDescriptors(),
	windowManagerHookEventDescriptors(),
	worktreeHookEventDescriptors(),
)
