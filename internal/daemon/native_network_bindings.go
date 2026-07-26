package daemon

import toolspkg "github.com/compozy/agh/internal/tools"

func (n *daemonNativeTools) networkToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
	readAvailability toolspkg.NativeAvailabilityFunc,
	usageAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDNetworkStatus: {
			call:         n.networkStatus,
			availability: availability,
		},
		toolspkg.ToolIDNetworkUsage: {
			call:         n.networkUsage,
			availability: usageAvailability,
		},
		toolspkg.ToolIDNetworkChannels: {
			call:         n.networkChannels,
			availability: availability,
		},
		toolspkg.ToolIDNetworkInbox: {
			call:         n.networkInbox,
			availability: availability,
		},
		toolspkg.ToolIDNetworkPeers: {
			call:         n.networkPeers,
			availability: availability,
		},
		toolspkg.ToolIDNetworkSend: {
			call:         n.networkSend,
			availability: availability,
		},
		toolspkg.ToolIDNetworkChannelCreate: {
			call:         n.networkChannelCreate,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkChannelUpdate: {
			call:         n.networkChannelUpdate,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkSubscriptions: {
			call:         n.networkSubscriptions,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkSubscribe: {
			call:         n.networkSubscribe,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkMute: {
			call:         n.networkMute,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkUnmute: {
			call:         n.networkUnmute,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkThreads: {
			call:         n.networkThreads,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkThreadMessages: {
			call:         n.networkThreadMessages,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkDirects: {
			call:         n.networkDirects,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkDirectResolve: {
			call:         n.networkDirectResolve,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkDirectMessages: {
			call:         n.networkDirectMessages,
			availability: readAvailability,
		},
		toolspkg.ToolIDNetworkWork: {
			call:         n.networkWork,
			availability: readAvailability,
		},
	}
}
