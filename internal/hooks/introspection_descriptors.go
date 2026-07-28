package hooks

var hookEventDescriptors = mergeHookEventDescriptors(
	sessionHookEventDescriptors(),
	agentHookEventDescriptors(),
	interactionHookEventDescriptors(),
	coordinationHookEventDescriptors(),
	executionHookEventDescriptors(),
	networkHookEventDescriptors(),
	windowManagerHookEventDescriptors(),
)
