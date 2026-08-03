package daemon

import toolspkg "github.com/compozy/compozy/internal/tools"

func (n *daemonNativeTools) resourceToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDResourcesList:     {call: n.resourcesList, availability: availability},
		toolspkg.ToolIDResourcesInfo:     {call: n.resourcesInfo, availability: availability},
		toolspkg.ToolIDResourcesSnapshot: {call: n.resourcesSnapshot, availability: availability},
	}
}
