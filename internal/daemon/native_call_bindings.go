package daemon

import (
	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) callToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDAgentCall:    {call: n.agentCall, availability: availability},
		toolspkg.ToolIDCallReturn:   {call: n.callReturn, availability: availability},
		toolspkg.ToolIDCallAwait:    {call: n.callAwait, availability: availability},
		toolspkg.ToolIDCallCancel:   {call: n.callCancel, availability: availability},
		toolspkg.ToolIDCallResult:   {call: n.callResult, availability: availability},
		toolspkg.ToolIDCallPublish:  {call: n.callPublish, availability: availability},
		toolspkg.ToolIDAgentMessage: {call: n.agentMessage, availability: availability},
	}
}

func (n *daemonNativeTools) callsService() core.CallsService {
	if n == nil || n.deps == nil || n.deps.Calls == nil {
		return nil
	}
	return n.deps.Calls()
}
