package daemon

import toolspkg "github.com/compozy/compozy/internal/tools"

func (n *daemonNativeTools) workspaceToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
	describeAvailability toolspkg.NativeAvailabilityFunc,
	agentCreateAvailability toolspkg.NativeAvailabilityFunc,
	agentCatalogAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDAgentList: {
			call:         n.agentList,
			availability: agentCatalogAvailability,
		},
		toolspkg.ToolIDWorkspaceList: {
			call:         n.workspaceList,
			availability: availability,
		},
		toolspkg.ToolIDWorkspaceInfo: {
			call:         n.workspaceInfo,
			availability: availability,
		},
		toolspkg.ToolIDWorkspaceDescribe: {
			call:         n.workspaceDescribe,
			availability: describeAvailability,
		},
		toolspkg.ToolIDAgentCreate: {
			call:         n.agentCreate,
			availability: agentCreateAvailability,
		},
	}
}

func (n *daemonNativeTools) vaultToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDVaultList: {call: n.vaultList, availability: availability},
	}
}
