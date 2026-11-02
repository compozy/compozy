package hooks

import "maps"

var hookEventDescriptors = mergeHookEventDescriptors(
	subagentHookEventDescriptors(),
	instanceHookEventDescriptors(),
	sessionHookEventDescriptors(),
	sessionAttentionHookEventDescriptors(),
	agentHookEventDescriptors(),
	instanceHookEventDescriptors(),
	interactionHookEventDescriptors(),
	coordinationHookEventDescriptors(),
	executionHookEventDescriptors(),
	windowManagerHookEventDescriptors(),
	worktreeHookEventDescriptors(),
	terminalHookEventDescriptors(),
)

func mergeHookEventDescriptors(
	base map[HookEvent]EventDescriptor,
	overlays ...map[HookEvent]EventDescriptor,
) map[HookEvent]EventDescriptor {
	for _, overlay := range overlays {
		maps.Copy(base, overlay)
	}
	return base
}
